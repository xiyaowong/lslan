package ui

import (
	"math/rand"
	"net/netip"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestSortHostsStable pins the fix for the sort-jitter bug: rows with
// equal keys must keep one order frame to frame — the view re-sorts the
// host map every frame, and a map's iteration order changes, so ties
// decided by it made the table jump under a resting pointer.
func TestSortHostsStable(t *testing.T) {
	// Three hosts sharing every key but the IP.
	hs := []model.Host{
		{IP: netip.MustParseAddr("192.168.1.7"), State: model.StateOnline, Vendor: "x"},
		{IP: netip.MustParseAddr("192.168.1.3"), State: model.StateOnline, Vendor: "x"},
		{IP: netip.MustParseAddr("192.168.1.5"), State: model.StateOnline, Vendor: "x"},
	}
	ips := func(hs []model.Host) []string {
		out := make([]string, len(hs))
		for i, h := range hs {
			out[i] = h.IP.String()
		}
		return out
	}
	for _, col := range []string{"state", "vendor", "ports", "hostname", "mac", "ip"} {
		for _, desc := range []bool{false, true} {
			so := ui.SortOrder{Column: col, Descending: desc}
			want := append([]model.Host(nil), hs...)
			sortHosts(want, so)
			for range 20 {
				got := append([]model.Host(nil), hs...)
				rand.Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
				sortHosts(got, so)
				if !slices.EqualFunc(want, got, func(x, y model.Host) bool { return x.IP == y.IP }) {
					t.Fatalf("sort %q desc=%v unstable: %v vs %v", col, desc, ips(want), ips(got))
				}
			}
		}
	}
}
