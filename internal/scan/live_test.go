package scan

import (
	"cmp"
	"net/netip"
	"slices"
	"testing"
	"time"

	"lslan/internal/model"
)

func TestListNICsLive(t *testing.T) {
	nics, err := ListNICs()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nics {
		if !n.IP.Is4() || n.PrefixLen <= 0 || n.PrefixLen > 32 {
			t.Errorf("NIC %q: bad ip/prefix %v/%d", n.Name, n.IP, n.PrefixLen)
		}
		if n.Name == "" {
			t.Error("NIC with empty friendly name")
		}
	}
}

func TestScanPortsLive(t *testing.T) {
	got, err := ScanPorts(t.Context(), netip.MustParseAddr("127.0.0.1"), []int{9100, 80, 445})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSortedFunc(got, func(a, b model.Port) int { return cmp.Compare(a.Number, b.Number) }) {
		t.Errorf("results not sorted: %v", got)
	}
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
}

func TestScannerSweepLive(t *testing.T) {
	s := NewScanner()
	nic := model.NIC{IP: netip.MustParseAddr("127.0.0.1"), PrefixLen: 8}
	if err := s.Start(nic, netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(nic, netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.1")); err == nil {
		t.Error("second Start while running did not error")
	}
	done := false
	timeout := time.After(15 * time.Second)
	for !done {
		select {
		case ev := <-s.Events():
			if ev.Kind == EventDone {
				done = true
			}
			if ev.Kind == EventError {
				t.Fatalf("sweep error: %v", ev.Err)
			}
		case <-timeout:
			t.Fatal("no EventDone within 15s")
		}
	}
}

func TestPingSessionLive(t *testing.T) {
	ctx := t.Context()
	p, err := StartPing(ctx, netip.MustParseAddr("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case s, ok := <-p.Samples():
		if !ok {
			t.Fatal("samples closed before any reply")
		}
		if !s.OK || s.Seq != 1 {
			t.Errorf("first sample = %+v, want OK seq 1", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no sample within 5s")
	}
	p.Cancel()
	select {
	case _, ok := <-p.Samples():
		if ok {
			// A reply already in flight; the next receive must be closed.
			if _, ok := <-p.Samples(); ok {
				t.Error("samples did not close after Cancel")
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("samples did not close after Cancel")
	}
}
