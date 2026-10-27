package ui

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestPortScanQueuesTheNextHost: asking for another host's ports while one
// is being scanned used to drop the request silently. It now waits its
// turn.
func TestPortScanQueuesTheNextHost(t *testing.T) {
	a := newApp()
	first, second := ip("192.168.1.5"), ip("192.168.1.6")
	a.hosts[first] = model.Host{IP: first, State: model.StateOnline}
	a.hosts[second] = model.Host{IP: second, State: model.StateOnline}
	var mu sync.Mutex
	scanned := map[netip.Addr]bool{}
	release := make(chan struct{})
	a.scanPorts = func(ctx context.Context, src, target netip.Addr, ports []int) ([]model.Port, error) {
		mu.Lock()
		firstCall := len(scanned) == 0
		mu.Unlock()
		if firstCall {
			<-release // hold the first scan until the test lets go
		}
		mu.Lock()
		scanned[target] = true
		mu.Unlock()
		return []model.Port{{Number: 80, Open: true, Service: "http"}}, nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, first
	tt.Frame()

	a.startPortScan(first)
	if !a.portScanning {
		t.Fatal("the first scan did not start")
	}
	// The user moves on and asks for the other host.
	a.startPortScan(second)
	if a.portWanted != second {
		t.Errorf("the second request was dropped, wanted %v", a.portWanted)
	}
	close(release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.runPending()
		mu.Lock()
		done := scanned[second]
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	got := scanned[second]
	mu.Unlock()
	if !got {
		t.Error("the queued host was never scanned")
	}
	if _, ok := a.ports[second]; !ok {
		t.Error("the queued host's ports are missing")
	}
}
