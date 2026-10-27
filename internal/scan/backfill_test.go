package scan

import (
	"testing"

	"lslan/internal/model"
)

// TestOfflineBackfillIgnoresOutOfRangeHosts: mDNS hears the whole subnet,
// so the host map can hold addresses outside the range asked for. The
// backfill must survive them: it once sized itself from the count and
// panicked with a negative capacity.
func TestOfflineBackfillIgnoresOutOfRangeHosts(t *testing.T) {
	lo := be32(na("192.168.1.10"))
	hi := lo
	known := map[uint32]struct{}{
		be32(na("192.168.1.20")): {},
		be32(na("192.168.1.21")): {},
		be32(na("192.168.1.22")): {},
	}
	got := offlineBackfill(lo, hi, known)
	if len(got) != 1 || got[0].IP.String() != "192.168.1.10" {
		t.Fatalf("backfill = %v, want the one in-range address", got)
	}
	if got[0].State != model.StateOffline {
		t.Errorf("state = %v, want offline", got[0].State)
	}
}

// TestOfflineBackfillFullRange: every address answered, nothing to fill.
func TestOfflineBackfillFullRange(t *testing.T) {
	lo, hi := be32(na("192.168.1.1")), be32(na("192.168.1.2"))
	known := map[uint32]struct{}{lo: {}, hi: {}}
	if got := offlineBackfill(lo, hi, known); len(got) != 0 {
		t.Errorf("backfill = %v, want none", got)
	}
}

// TestOfflineBackfill: every address the sweep never found responding
// comes back as an offline host, in order.
func TestOfflineBackfill(t *testing.T) {
	known := map[uint32]struct{}{
		be32(na("192.168.1.2")): {},
		be32(na("192.168.1.5")): {},
	}
	got := offlineBackfill(be32(na("192.168.1.1")), be32(na("192.168.1.6")), known)
	want := []string{"192.168.1.1", "192.168.1.3", "192.168.1.4", "192.168.1.6"}
	if len(got) != len(want) {
		t.Fatalf("backfill = %d hosts, want %d", len(got), len(want))
	}
	for i, h := range got {
		if h.IP.String() != want[i] {
			t.Errorf("backfill[%d] = %s, want %s", i, h.IP, want[i])
		}
		if h.State != model.StateOffline {
			t.Errorf("backfill[%d] state = %v, want offline", i, h.State)
		}
	}
}
