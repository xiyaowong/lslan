//go:build windows

package scan

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
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
)

// probeHost decides liveness: ICMP echo when the ICMP device opens,
// otherwise a TCP connect fallback.
func probeHost(ip netip.Addr) bool {
	if icmpAvailable() {
		_, _, ok := icmpPing(ip, time.Second)
		return ok
	}
	return tcpAlive(ip, arpProbeTimeout)
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
// hosts are resolved and EventProgress as the sweep advances.
func (s *Scanner) Start(nic model.NIC, start, end netip.Addr) error {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() {
		return errInvalidAddress
	}
	lo, hi := be32(start), be32(end)
	if lo > hi {
		return errors.New("scan: start address is after end address")
	}
	total := int64(hi-lo) + 1

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("scan: a sweep is already running")
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
	var scanned, online atomic.Int64
	var next atomic.Int64

	// snapshotHost copies a host record so events never alias live state.
	snapshotHost := func(key uint32) *model.Host {
		dataMu.Lock()
		h := *hosts[key]
		dataMu.Unlock()
		return &h
	}

	// mDNS runs beside the sweep: unicast queries to every host found
	// and a passive ear on the multicast group, both folded back into
	// the same host map by IP.
	var mdns *mdnsRunner
	mdns = startMDNS(ctx, nic, func(ip netip.Addr, hostname string, svcs []model.Service) {
		if ctx.Err() != nil {
			return
		}
		key := be32(ip)
		dataMu.Lock()
		h, known := hosts[key]
		if !known {
			h = &model.Host{IP: ip, State: model.StateOnline, LastSeen: time.Now(), ViaMDNS: true}
			hosts[key] = h
		}
		changed := !known
		if hostname != "" && hostname != h.Hostname {
			h.Hostname = hostname
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
	for w := 0; w < sweepWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := next.Add(1) - 1
				if i >= total || ctx.Err() != nil {
					return
				}
				ip := addrFromUint32(lo + uint32(i))
				if probeHost(ip) {
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
				s.emit(ctx, Event{Kind: EventProgress, Progress: progress(total, &scanned, &online)})
			}
		}()
	}
	wg.Wait()

	// Consistency pass. Addresses the ICMP burst left in the ARP cache
	// but that did not answer ping get a quick TCP liveness check
	// (catches devices that block ICMP; cache entries may be a few
	// minutes stale, so a device gone dark just before a resweep can
	// leak through as online). Known hosts missing a MAC are backfilled
	// from a fresh table read.
	for addr, mac := range readARPTable() {
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
		if ctx.Err() != nil {
			break
		}
		if tcpAlive(addr, arpProbeTimeout) {
			h := &model.Host{IP: addrFromUint32(key), State: model.StateOnline, LastSeen: time.Now()}
			if gatewayValid && key == gateway {
				h.Gateway = true
			}
			h.MAC = mac
			h.Vendor = OUIVendor(mac)
			enrichHost(key, h)
		}
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
			h := h
			s.emit(ctx, Event{Kind: EventHost, Host: &h})
		}
	}

	// Final pass: wait for the remaining lookups, re-emit every host
	// with everything filled in, then the final progress.
	names.wait()
	dataMu.Lock()
	keys := make([]uint32, 0, len(hosts))
	for k := range hosts {
		keys = append(keys, k)
	}
	dataMu.Unlock()
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		s.emit(ctx, Event{Kind: EventHost, Host: snapshotHost(k)})
	}
	s.emitFinal(ctx, Event{Kind: EventDone, Progress: progress(total, &scanned, &online)})
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
func (h *hostnames) lookup(key uint32, ip netip.Addr, cb func(name string)) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
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
	}()
}

// wait blocks until every in-flight lookup, and its callback, is done.
func (h *hostnames) wait() { h.wg.Wait() }

// offlineBackfill lists every address in [lo,hi] the sweep never found
// responding, as an offline host each. It must be called only after the
// sweep is done adding hosts.
func offlineBackfill(lo, hi uint32, known map[uint32]struct{}) []model.Host {
	out := make([]model.Host, 0, int(hi-lo)+1-len(known))
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

// emitFinal delivers terminal events (Done/Error) even after
// cancellation so the caller always sees the sweep end.
func (s *Scanner) emitFinal(_ context.Context, ev Event) {
	s.events <- ev
}
