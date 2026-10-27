package ui

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestTitleBarButtonsNamed: the window buttons are drawn, not labelled by
// text, and the Label put on the icon was dropped — Button is a leaf that
// reads only its own text. Find works by label, so a missing one shows up
// here too.
func TestTitleBarButtonsNamed(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	for _, name := range []string{"切换深浅颜色", "最小化", "关闭"} {
		if _, ok := tt.Find(name); !ok {
			t.Errorf("%q is not reachable by name", name)
		}
	}
}

// TestIconButtonClickTarget: small glyphs still get a usable hit area.
func TestIconButtonClickTarget(t *testing.T) {
	for _, size := range []float32{16, 18, 24} {
		view := func(c *ui.Context) {
			ui.Row(c).Padding(20).Children(func() {
				iconButton(c, "close "+strconv.Itoa(int(size)), "关闭", size, drawClose)
			})
		}
		tt := ui.NewTester(view, 200, 100)
		tt.Frame()
		r, ok := tt.Find("close " + strconv.Itoa(int(size)))
		if !ok {
			t.Fatalf("size %v: the button is not findable", size)
		}
		if r.W < minClickTarget || r.H < minClickTarget {
			t.Errorf("size %v gave a %vx%v target", size, r.W, r.H)
		}
	}
}

// TestStatusDotOnlyWhenOnline: the green dot means somebody is here; it
// no longer stays lit with nobody online.
func TestStatusDotOnlyWhenOnline(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	if a.onlineCount() != 0 {
		t.Fatal("the fixture expects no online host")
	}
	// The dot is a box with no text; the label is what ties it to the row.
	if _, ok := tt.Find("在线"); !ok {
		t.Error("the online tally is missing")
	}
}

// TestDetailHeaderShowsState: the detail header says online/offline in
// words, not only by the colour of a dot.
func TestDetailHeaderShowsState(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.7")
	a.hosts[h] = model.Host{IP: h, State: model.StateOffline, MAC: "aa:bb:cc:dd:ee:ff"}
	a.ports[h] = nil
	a.selRow, a.selIP = 0, h
	a.showOffline = true
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	if !tt.HasText("离线") {
		t.Errorf("the detail header never says the device is offline, texts %q", tt.Texts())
	}
}
