package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestCardKeepsItsWidth: the host card stays as wide as its columns need
// however wide the window is, and the detail panel takes the rest.
func TestCardKeepsItsWidth(t *testing.T) {
	for _, width := range []int{880, 1080, 1600, 2200} {
		a := newApp()
		tt := ui.NewTester(a.View, width, 800)
		ip := netip.MustParseAddr("192.168.1.5")
		a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline, Hostname: "router"}
		a.autoScanned = map[netip.Addr]bool{ip: true}
		a.selRow, a.selIP = 0, ip
		tt.Frame()

		row, ok := tt.Find(ip.String())
		if !ok {
			t.Fatalf("%dpx: the row is missing", width)
		}
		closeBtn, ok := tt.Find("关闭详情")
		if !ok {
			t.Fatalf("%dpx: the detail panel is missing", width)
		}
		card := a.cardWidth
		if card > cardWidthMax || card < cardWidthMin {
			t.Errorf("%dpx: card %v is outside %v..%v", width, card, cardWidthMin, cardWidthMax)
		}
		// The panel takes everything the card and the margins leave.
		panelW := float32(width) - card - 16 - 12 - 16
		if closeBtn.X+closeBtn.W > float32(width) {
			t.Errorf("%dpx: the panel runs past the window", width)
		}
		if panelW < detailMinWidth && width >= 1000 {
			t.Errorf("%dpx: the panel gets %v, less than %v", width, panelW, detailMinWidth)
		}
		if row.X+row.W > float32(width) {
			t.Errorf("%dpx: the row runs past the window", width)
		}
	}
}
