package scan

import (
	"testing"

	"lslan/internal/model"
)

// TestOfflineBackfill: after a sweep, every address it never found
// responding comes back as an offline host, in order (issue #21).
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

// TestOfflineBackfillFullRange: a range where everything answered
// backfills nothing.
func TestOfflineBackfillFullRange(t *testing.T) {
	known := map[uint32]struct{}{}
	for _, s := range []string{"192.168.1.1", "192.168.1.2"} {
		known[be32(na(s))] = struct{}{}
	}
	if got := offlineBackfill(be32(na("192.168.1.1")), be32(na("192.168.1.2")), known); len(got) != 0 {
		t.Errorf("backfill = %v, want none", got)
	}
}
