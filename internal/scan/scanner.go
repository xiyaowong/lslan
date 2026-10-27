//go:build windows

package scan

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lslan/internal/model"
)

const (
	sweepWorkers    = 64
	hostnameWorkers = 16
	hostnameTimeout = 1500 * time.Millisecond
	arpProbeTimeout = 300 * time.Millisecond
	arpCacheWindow  = 500 * time.Millisecond
	eventBuf        = 1024
	// progressEvery paces the progress line: it shows a percentage, and a
	// repaint per address tells the user nothing extra.
	progressEvery = 100 * time.Millisecond
	// finalEventWait bounds the last event's delivery: a caller that has
	// stopped draining must not wedge the sweep goroutines.
	finalEventWait = 2 * time.Second
)

// probeHost decides liveness: ICMP echo when the ICMP device opens,
// otherwise a TCP connect fallback made from src, the adapter's address
// (see tcpAliveFrom).
func probeHost(src, ip netip.Addr) bool {
	if icmpAvailable() {
		_, _, ok := icmpPing(ip, time.Second)
		return ok
	}
	return tcpAliveFrom(src, ip, arpProbeTimeout)
}

func defaultLookup(ctx context.Context, ip string) (string, error) {
	addrs, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", nil
	}
	return strings.TrimSuffix(addrs[0], "."), nil
}

// Scanner runs one host sweep at a time in background goroutines and
// reports results on Events. Events are drained by the caller; Start
// returns an error if a sweep is already running.
type Scanner struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	events  chan Event
}

// NewScanner returns an idle Scanner.
func NewScanner() *Scanner { return &Scanner{events: make(chan Event, eventBuf)} }

// Events returns the stream the active sweep reports on. It stays open
// across sweeps; callers must keep draining it.
func (s *Scanner) Events() <-chan Event { return s.events }

// Start sweeps [start,end] via the given NIC, emitting EventHost as
// hosts are resolved and EventProgress as the sweep advances. Its errors
// are sentinels the interface can tell apart and explain.
func (s *Scanner) Start(nic model.NIC, start, end netip.Addr) error {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() {
		return errInvalidAddress
	}
	lo, hi := be32(start), be32(end)
	if lo > hi {
		return ErrRangeReversed
	}
	total := int64(hi-lo) + 1

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.cancel = cancel
	go s.sweep(ctx, cancel, nic, lo, hi, total)
	return nil
}

// Cancel aborts the running sweep, if any. It does not wait for it.
func (s *Scanner) Cancel() {
	s.mu.Lock()
	c := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if c != nil {
		c()
	}
}

func (s *Scanner) sweep(ctx context.Context, cancel context.CancelFunc, nic model.NIC, lo, hi uint32, total int64) {
	defer cancel()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	// A panic in a sweep goroutine would take the whole app down; report
	// it as a failed sweep instead, so the interface can say so and carry on.
	defer func() {
		if r := recover(); r != nil {
			s.emitFinal(ctx, Event{Kind: EventError, Err: fmt.Errorf("scan: sweep failed: %v", r)})
		}
	}()

	gateway := uint32(0)
	gatewayValid := nic.Gateway.IsValid() && nic.Gateway.Is4()
	if gatewayValid {
		gateway = be32(nic.Gateway)
	}

	// Streaming path: one shared ARP snapshot rate-limits the table
	// reads across all workers; hostname lookups run per discovered
	// host under a bounded semaphore and re-emit as each name lands.
	arp := newARPCache(arpCacheWindow)
	names := newHostnames(ctx)

	hosts := make(map[uint32]*model.Host)
	var dataMu sync.Mutex // guards hosts
	// seen holds the fingerprint of what the interface was last told about
	// each host, so the final pass can skip the ones already up to date.
	seen := make(map[uint32]string)
	var scanned, online atomic.Int64
	var next atomic.Int64

	// snapshotHost copies a host record so events never alias live state,
	// and remembers what the interface will now know of it.
	snapshotHost := func(key uint32) *model.Host {
		dataMu.Lock()
		h := *hosts[key]
		seen[key] = fingerprint(&h)
		dataMu.Unlock()
		return &h
	}

	// mDNS runs beside the sweep: unicast queries to every host found
	// and a passive ear on the multicast group, both folded back into
	// the same host map by IP.
	var mdns *mdnsRunner
	mdns = startMDNS(ctx, nic, func(ip netip.Addr, hostname, kind string, svcs []model.Service) {
		if ctx.Err() != nil {
			return
		}
		key := be32(ip)
		// The passive ear hears the whole subnet, not just the range asked
		// for; a host outside it is not part of this sweep.
		if key < lo || key > hi {
			return
		}
		dataMu.Lock()
		h, known := hosts[key]
		if !known {
			h = &model.Host{IP: ip, State: model.StateOnline, LastSeen: time.Now(), ViaMDNS: true}
			hosts[key] = h
		}
		changed := !known
		// The first name wins, as it does for the reverse lookup: two
		// responders on one address would otherwise flip it on every
		// answer.
		if hostname != "" && h.Hostname == "" {
			h.Hostname = hostname
			changed = true
		}
		if kind != "" && h.Model == "" {
			h.Model = kind
			changed = true
		}
		for _, sv := range svcs {
			n := len(h.Services)
			h.Services = appendService(h.Services, sv)
			changed = changed || len(h.Services) != n
		}
		h.LastSeen = time.Now()
		dataMu.Unlock()
		if changed {
			if !known {
				online.Add(1)
			}
			s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(key)})
		}
		mdns.probe(ip)
	})

	// enrichHost stores a freshly discovered host, emits it, asks mDNS
	// about it, then starts its hostname lookup: the row lights up with
	// MAC/vendor at once, then updates in place when the name lands.
	// Events carry snapshots, never the live record — the lookup
	// callback mutates it.
	//
	// Called at most once per key from the sweep side: workers see
	// disjoint addresses and the consistency pass only for keys not
	// stored here yet. mDNS may have stored facts for the key first,
	// so those survive the store below.
	enrichHost := func(key uint32, h *model.Host) {
		dataMu.Lock()
		old, known := hosts[key]
		if known {
			if h.Hostname == "" {
				h.Hostname = old.Hostname
			}
			h.Services = old.Services
			h.ViaMDNS = h.ViaMDNS || old.ViaMDNS
		}
		hosts[key] = h
		dataMu.Unlock()
		if !known {
			online.Add(1)
		}
		s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(key)})
		mdns.probe(addrFromUint32(key))
		names.lookup(key, addrFromUint32(key), func(name string) {
			if name == "" {
				return
			}
			dataMu.Lock()
			// A mDNS hostname, set directly by the device, wins over
			// the reverse-DNS answer.
			if hosts[key].Hostname == "" {
				hosts[key].Hostname = name
			}
			dataMu.Unlock()
			s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(key)})
		})
	}

	var wg sync.WaitGroup
	// The progress line is a percentage on screen; one event per address
	// would cost the interface a repaint per address for no new
	// information. Sent at most this often, and once at the end.
	var lastProgress atomic.Int64
	for range sweepWorkers {
		wg.Go(func() {
			for {
				i := next.Add(1) - 1
				if i >= total || ctx.Err() != nil {
					return
				}
				ip := addrFromUint32(lo + uint32(i))
				if probeHost(nic.IP, ip) {
					key := be32(ip)
					h := &model.Host{IP: ip, State: model.StateOnline, LastSeen: time.Now()}
					if gatewayValid && key == gateway {
						h.Gateway = true
					}
					// The probe put the host into the ARP cache; pick
					// up its MAC right away so the first event already
					// carries vendor.
					if mac, ok := arp.mac(ip); ok {
						h.MAC = mac
						h.Vendor = OUIVendor(mac)
					}
					enrichHost(key, h)
				}
				scanned.Add(1)
				now := time.Now().UnixNano()
				if prev := lastProgress.Load(); now-prev >= int64(progressEvery) &&
					lastProgress.CompareAndSwap(prev, now) {
					s.emit(ctx, Event{Kind: EventProgress, Progress: progress(total, &scanned, &online)})
				}
			}
		})
	}
	wg.Wait()
	s.emit(ctx, Event{Kind: EventProgress, Progress: progress(total, &scanned, &online)})

	// Consistency pass. Addresses the ICMP burst left in the ARP cache
	// but that did not answer ping get a quick TCP liveness check
	// (catches devices that block ICMP; cache entries may be a few
	// minutes stale, so a device gone dark just before a resweep can
	// leak through as online). Known hosts missing a MAC are backfilled
	// from a fresh table read. The probes run in parallel: a machine's
	// ARP table can hold dozens of stale entries, and one at a time would
	// hold the sweep's end for tens of seconds.
	table := readARPTable()
	var unknown []struct {
		key uint32
		mac string
	}
	for addr, mac := range table {
		if !addr.Is4() {
			continue
		}
		key := be32(addr)
		if key < lo || key > hi {
			continue
		}
		dataMu.Lock()
		h, known := hosts[key]
		if known {
			if h.MAC == "" {
				h.MAC = mac
				h.Vendor = OUIVendor(mac)
			}
			dataMu.Unlock()
			continue
		}
		dataMu.Unlock()
		unknown = append(unknown, struct {
			key uint32
			mac string
		}{key, mac})
	}
	var probeWG sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, u := range unknown {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		probeWG.Go(func() {
			defer func() { <-sem }()
			if !tcpAliveFrom(nic.IP, addrFromUint32(u.key), arpProbeTimeout) {
				return
			}
			h := &model.Host{IP: addrFromUint32(u.key), State: model.StateOnline, LastSeen: time.Now()}
			if gatewayValid && u.key == gateway {
				h.Gateway = true
			}
			h.MAC = u.mac
			h.Vendor = OUIVendor(u.mac)
			enrichHost(u.key, h)
		})
	}
	probeWG.Wait()

	// UPnP, last: it needs the addresses the sweep found, and it names
	// TVs, routers and printers that mDNS never hears from. Every failure
	// there is silent, like mDNS.
	if ctx.Err() == nil {
		dataMu.Lock()
		found := make([]netip.Addr, 0, len(hosts))
		for k, h := range hosts {
			if h.Model == "" {
				found = append(found, addrFromUint32(k))
			}
		}
		dataMu.Unlock()
		slices.SortFunc(found, func(a, b netip.Addr) int { return a.Compare(b) })
		ssdpDiscover(ctx, found, func(ip netip.Addr, d ssdpDevice) {
			if ctx.Err() != nil {
				return
			}
			key := be32(ip)
			if key < lo || key > hi {
				return
			}
			dataMu.Lock()
			h, known := hosts[key]
			if !known {
				dataMu.Unlock()
				return
			}
			changed := false
			if d.Model != "" && h.Model == "" {
				h.Model = d.Model
				changed = true
			}
			// The OUI table knows the maker of a MAC; a device that
			// announces its own manufacturer knows better.
			if d.Vendor != "" && h.Vendor == "" {
				h.Vendor = d.Vendor
				changed = true
			}
			dataMu.Unlock()
			if changed {
				s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(key)})
			}
		})
	}

	// Offline backfill: on a full, uncancelled pass, every probed address
	// that never answered goes to the UI as an offline host, so the
	// 显示离线 filter has rows to show.
	if ctx.Err() == nil {
		dataMu.Lock()
		known := make(map[uint32]struct{}, len(hosts))
		for k := range hosts {
			known[k] = struct{}{}
		}
		dataMu.Unlock()
		for _, h := range offlineBackfill(lo, hi, known) {
			s.emit(ctx, Event{Kind: EventHost, Host: &h})
		}
	}

	// Final pass: wait for the remaining lookups, then re-emit only the
	// hosts that changed since their last event — a host whose facts all
	// arrived with it would otherwise be sent twice, and every send costs
	// the interface a repaint.
	names.wait()
	dataMu.Lock()
	var stale []uint32
	for k, h := range hosts {
		if seen[k] != fingerprint(h) {
			stale = append(stale, k)
		}
	}
	dataMu.Unlock()
	slices.Sort(stale)
	for _, k := range stale {
		s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(k)})
	}
	s.emitFinal(ctx, Event{Kind: EventDone, Progress: progress(total, &scanned, &online)})
}

// fingerprint is what the interface shows of a host; two snapshots with the
// same fingerprint look the same on screen.
func fingerprint(h *model.Host) string {
	return h.Hostname + "\x00" + h.MAC + "\x00" + h.Vendor +
		"\x00" + strconv.Itoa(len(h.Services))
}

// hostnames runs bounded-concurrency reverse DNS for discovered hosts
// and reports each result as soon as it lands, instead of holding all
// names for a sweep-end batch.
type hostnames struct {
	ctx context.Context
	sem chan struct{}
	wg  sync.WaitGroup
}

func newHostnames(ctx context.Context) *hostnames {
	return &hostnames{ctx: ctx, sem: make(chan struct{}, hostnameWorkers)}
}

// lookup resolves ip in the background and calls back with the name,
// or "" on failure or cancellation. The callback runs on the lookup
// goroutine, so wait covers it too.
func (h *hostnames) lookup(_ uint32, ip netip.Addr, cb func(name string)) {
	h.wg.Go(func() {
		select {
		case h.sem <- struct{}{}:
		case <-h.ctx.Done():
			return
		}
		defer func() { <-h.sem }()
		lctx, lc := context.WithTimeout(h.ctx, hostnameTimeout)
		defer lc()
		name, err := defaultLookup(lctx, ip.String())
		if err != nil || name == "" {
			cb("")
			return
		}
		cb(name)
	})
}

// wait blocks until every in-flight lookup, and its callback, is done.
func (h *hostnames) wait() { h.wg.Wait() }

// offlineBackfill lists every address in [lo,hi] the sweep never found
// responding, as an offline host each. It must be called only after the
// sweep is done adding hosts.
func offlineBackfill(lo, hi uint32, known map[uint32]struct{}) []model.Host {
	// known may hold keys outside [lo,hi] (mDNS hears the whole subnet), so
	// the spare capacity is floored at zero rather than going negative.
	spare := int(hi-lo) + 1 - len(known)
	out := make([]model.Host, 0, max(spare, 0))
	for key := lo; ; key++ {
		if _, ok := known[key]; !ok {
			out = append(out, model.Host{IP: addrFromUint32(key), State: model.StateOffline})
		}
		if key == hi {
			break
		}
	}
	return out
}

func progress(total int64, scanned, online *atomic.Int64) model.Progress {
	return model.Progress{Scanned: int(scanned.Load()), Total: int(total), Online: int(online.Load())}
}

// emit delivers an event unless the sweep was cancelled first.
func (s *Scanner) emit(ctx context.Context, ev Event) {
	select {
	case s.events <- ev:
	case <-ctx.Done():
	}
}

// emitFinal delivers terminal events (Done/Error) even after cancellation
// so the caller always sees the sweep end. It waits a bounded time and
// then gives up: a caller that stopped draining must not be able to wedge
// the sweep goroutines forever.
func (s *Scanner) emitFinal(_ context.Context, ev Event) {
	select {
	case s.events <- ev:
	case <-time.After(finalEventWait):
	}
}
