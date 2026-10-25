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

	hosts := make(map[uint32]*model.Host)
	macs := make(map[uint32]string)
	var dataMu sync.Mutex // guards hosts and macs
	var scanned, online atomic.Int64
	var next atomic.Int64

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
					h := &model.Host{IP: ip, State: model.StateOnline, LastSeen: time.Now()}
					if gatewayValid && be32(ip) == gateway {
						h.Gateway = true
					}
					dataMu.Lock()
					hosts[be32(ip)] = h
					dataMu.Unlock()
					online.Add(1)
					s.emit(ctx, Event{Kind: EventHost, Host: h})
				}
				scanned.Add(1)
				s.emit(ctx, Event{Kind: EventProgress, Progress: progress(total, &scanned, &online)})
			}
		}()
	}
	wg.Wait()

	// ARP enrichment. A host the ICMP burst left in the ARP cache but
	// that did not answer ping gets a quick TCP liveness check (catches
	// devices that block ICMP); cache entries may be a few minutes
	// stale, so a device gone dark just before a resweep can leak
	// through as online.
	for addr, mac := range readARPTable() {
		if !addr.Is4() {
			continue
		}
		key := be32(addr)
		if key < lo || key > hi {
			continue
		}
		dataMu.Lock()
		_, known := hosts[key]
		dataMu.Unlock()
		if known {
			macs[key] = mac
			continue
		}
		if ctx.Err() != nil {
			break
		}
		if tcpAlive(addr, arpProbeTimeout) {
			h := &model.Host{IP: addrFromUint32(key), State: model.StateOnline, LastSeen: time.Now()}
			if gatewayValid && key == gateway {
				h.Gateway = true
			}
			dataMu.Lock()
			hosts[key] = h
			macs[key] = mac
			dataMu.Unlock()
			online.Add(1)
			s.emit(ctx, Event{Kind: EventHost, Host: h})
		}
	}

	// Hostname enrichment, bounded-concurrency reverse DNS.
	names := make(map[uint32]string, len(hosts))
	var namesMu sync.Mutex
	var hwg sync.WaitGroup
	sem := make(chan struct{}, hostnameWorkers)
	dataMu.Lock()
	keys := make([]uint32, 0, len(hosts))
	for k := range hosts {
		keys = append(keys, k)
	}
	dataMu.Unlock()
	for _, key := range keys {
		hwg.Add(1)
		go func(key uint32) {
			defer hwg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			lctx, lc := context.WithTimeout(ctx, hostnameTimeout)
			defer lc()
			name, err := defaultLookup(lctx, addrFromUint32(key).String())
			if err != nil || name == "" {
				return
			}
			namesMu.Lock()
			names[key] = name
			namesMu.Unlock()
		}(key)
	}
	hwg.Wait()

	// Final pass: re-emit every discovered host with MAC, vendor and
	// hostname filled in, then the final progress.
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		dataMu.Lock()
		h := *hosts[k]
		dataMu.Unlock()
		h.Hostname = names[k]
		h.MAC = macs[k]
		if h.MAC != "" {
			h.Vendor = OUIVendor(h.MAC)
		}
		s.emit(ctx, Event{Kind: EventHost, Host: &h})
	}
	s.emitFinal(ctx, Event{Kind: EventDone, Progress: progress(total, &scanned, &online)})
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
