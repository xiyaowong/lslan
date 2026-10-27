package scan

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"lslan/internal/model"
)

// listenLoopback opens a listener so at least one address answers.
func listenLoopback() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

// TestSweepEventVolumeLive: one sweep must not put an event on the stream
// per address per fact. The interface folds every event into a repaint,
// so the count is the scan's cost to it. It was 63 progress events plus
// two host events each for 64 addresses; pacing the progress line and
// re-sending only hosts that changed brought it to one host event each
// and a handful of progress events.
func TestSweepEventVolumeLive(t *testing.T) {
	if testing.Short() {
		t.Skip("sweeps a real subnet")
	}
	const addresses = 64
	s := NewScanner()
	nic := model.NIC{IP: netip.MustParseAddr("127.0.0.1"), PrefixLen: 32}

	// Two loopback addresses answer, to give host events something to do.
	l, err := listenLoopback()
	if err != nil {
		t.Skipf("no listener: %v", err)
	}
	defer l.Close()

	if err := s.Start(nic, netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.63")); err != nil {
		t.Fatal(err)
	}

	var progress, hosts int
	deadline := time.After(60 * time.Second)
	for {
		select {
		case ev := <-s.Events():
			switch ev.Kind {
			case EventProgress:
				progress++
			case EventHost:
				hosts++
			case EventDone:
				t.Logf("%d addresses: %d progress events, %d host events", addresses, progress, hosts)
				// The progress line shows a percentage: a handful of
				// updates is plenty, one per address is not.
				if progress > addresses/4 {
					t.Errorf("%d progress events for %d addresses; the line is paced too finely", progress, addresses)
				}
				return
			}
		case <-deadline:
			t.Fatal("no EventDone")
		}
	}
}
