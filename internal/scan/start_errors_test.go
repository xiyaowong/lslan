package scan

import (
	"errors"
	"net/netip"
	"testing"

	"lslan/internal/model"
)

// TestStartErrorsAreTellable: the interface explains each refusal, so the
// reasons have to be distinguishable rather than one shared string.
func TestStartErrorsAreTellable(t *testing.T) {
	s := NewScanner()
	nic := model.NIC{IP: netip.MustParseAddr("127.0.0.1"), PrefixLen: 8}
	lo := netip.MustParseAddr("127.0.0.1")

	if err := s.Start(nic, lo, netip.MustParseAddr("127.0.0.5")); err != nil {
		t.Fatalf("start: %v", err)
	}
	// A second sweep while the first runs is ErrBusy.
	if err := s.Start(nic, lo, lo); !errors.Is(err, ErrBusy) {
		t.Errorf("second start = %v, want ErrBusy", err)
	}
	s.Cancel()
	for range s.Events() {
		break
	}
}

// TestRangeReversed: start after end has its own sentinel.
func TestRangeReversed(t *testing.T) {
	s := NewScanner()
	err := s.Start(model.NIC{}, netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("10.0.0.1"))
	if !errors.Is(err, ErrRangeReversed) {
		t.Errorf("reversed range = %v, want ErrRangeReversed", err)
	}
}
