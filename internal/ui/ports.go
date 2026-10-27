package ui

import (
	"context"
	"net/netip"
	"sync"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// presets caches the engine's port presets.
func (a *app) presets() []scan.Preset {
	if a.presetCache == nil {
		a.presetCache = a.portPresets()
	}
	return a.presetCache
}

// sweepPorts probes the ports of every host the sweep found online, a
// few at a time, and fills the results in as they come. A later sweep or
// a manual scan leaves the slow ones to their results, which portGen
// drops.
func (a *app) sweepPorts() {
	var ips []netip.Addr
	for ip, h := range a.hosts {
		if h.State != model.StateOffline {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return
	}
	a.sweepLeft = len(ips)
	a.sweepPending = make(map[netip.Addr]bool, len(ips))
	for _, ip := range ips {
		a.sweepPending[ip] = true
	}
	a.portGen++
	gen := a.portGen
	preset := a.presets()[0]
	src := a.sourceAddr()
	sem := make(chan struct{}, 4)
	go func() {
		var wg sync.WaitGroup
		for _, ip := range ips {
			sem <- struct{}{}
			wg.Go(func() {
				defer func() { <-sem }()
				ports, err := a.scanPorts(context.Background(), src, ip, preset.Ports)
				a.dispatch(func() {
					if a.portGen != gen {
						return
					}
					a.sweepLeft = max(a.sweepLeft-1, 0)
					delete(a.sweepPending, ip)
					if err != nil {
						return
					}
					// A scan the user asked for meanwhile is the newer word.
					if _, done := a.ports[ip]; !done {
						a.ports[ip] = ports
					}
				})
			})
		}
		wg.Wait()
	}()
}

// startPortScan probes ip with the common preset.
func (a *app) startPortScan(ip netip.Addr) { a.startPortScanPreset(ip, 0) }

// startPortScanPreset probes ip with the preset at index i of presets().
// One scan runs at a time; a host the user asked for while another is
// being scanned waits its turn rather than being dropped.
func (a *app) startPortScanPreset(ip netip.Addr, i int) {
	ps := a.presets()
	if i < 0 || i >= len(ps) {
		return
	}
	if a.portScanning {
		a.portWanted = ip
		a.portWantedPreset = i
		return
	}
	a.runPortScan(ip, i)
}

// runPortScan starts one scan; its gen drops a result the user has since
// superseded with a new sweep.
func (a *app) runPortScan(ip netip.Addr, i int) {
	preset := a.presets()[i]
	ctx, cancel := context.WithCancel(context.Background())
	src := a.sourceAddr()
	a.portScanning = true
	a.portIP = ip
	a.portErr = ""
	a.portGen++
	gen := a.portGen
	go func() {
		ports, err := a.scanPorts(ctx, src, ip, preset.Ports)
		cancel()
		a.dispatch(func() {
			if a.portGen != gen {
				return // a new sweep owns the cache now
			}
			a.portScanning = false
			if err != nil {
				a.portErr = "端口扫描未完成，请稍后重试"
			} else {
				a.ports[ip] = ports
			}
			// The host the user moved to meanwhile gets its turn.
			if want := a.portWanted; want.IsValid() {
				preset := a.portWantedPreset
				a.portWanted = netip.Addr{}
				if _, done := a.ports[want]; !done {
					a.runPortScan(want, preset)
				}
			}
		})
	}()
}
