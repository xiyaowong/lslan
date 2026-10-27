package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestDeviceKind: the guess follows the strongest signal first, and says
// nothing when it has none.
func TestDeviceKind(t *testing.T) {
	cases := []struct {
		name  string
		host  model.Host
		ports []model.Port
		want  deviceKind
	}{
		{"gateway", model.Host{Gateway: true}, nil, kindRouter},
		{"printer service", model.Host{Services: []model.Service{{Name: "_ipp._tcp", Port: 631}}}, nil, kindPrinter},
		{"printer port", model.Host{}, []model.Port{{Number: 9100, Open: true}}, kindPrinter},
		{"camera by model", model.Host{Model: "Hikvision IPC-HDW1230S"}, nil, kindCamera},
		{"cast", model.Host{Services: []model.Service{{Name: "_googlecast._tcp", Port: 8009}}}, nil, kindTV},
		{"nas port", model.Host{}, []model.Port{{Number: 445, Open: true}}, kindNAS},
		{"nothing", model.Host{}, nil, kindUnknown},
		// A vendor says who made a device, not what it is: TP-Link makes
		// routers, cameras and plugs alike, so the vendor alone is no clue.
		{"vendor alone", model.Host{Vendor: "Tp-Link"}, nil, kindUnknown},
		// A model the device announced outranks what it serves: an
		// iPhone airplaying is a phone, not a TV.
		{"iphone over airplay", model.Host{
			Model:    "iPhone14,2",
			Services: []model.Service{{Name: "_airplay._tcp", Port: 7000}},
		}, nil, kindPhone},
		{"camera model", model.Host{Model: "IPC-HDW1230S"}, nil, kindCamera},
	}
	for _, c := range cases {
		if got := deviceKindOf(c.host, c.ports); got != c.want {
			t.Errorf("%s: kind = %v, want %v", c.name, got.Label(), c.want.Label())
		}
	}
}

// TestUnknownKindSaysNothing: a device with no clue gets no label rather
// than a wrong one.
func TestUnknownKindSaysNothing(t *testing.T) {
	if kindUnknown.Label() != "未知" {
		t.Errorf("the unknown kind reads %q", kindUnknown.Label())
	}
	a := newApp()
	h := ip("192.168.1.50")
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline}
	a.ports[h] = nil
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	for _, kind := range []deviceKind{kindRouter, kindPrinter, kindCamera, kindNAS, kindTV} {
		if tt.HasText(kind.Label()) {
			t.Errorf("a device with no clue is labelled %q", kind.Label())
		}
	}
}

// TestSelfMarker: the row for this machine says so.
func TestSelfMarker(t *testing.T) {
	a := newApp()
	me := netip.MustParseAddr("192.168.1.23")
	a.nics = []model.NIC{{Name: "以太网", IP: me, PrefixLen: 24}}
	a.nicIdx = 0
	a.hosts[me] = model.Host{IP: me, State: model.StateOnline}
	a.hosts[ip("192.168.1.5")] = model.Host{IP: ip("192.168.1.5"), State: model.StateOnline}
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	if !tt.HasText("本机") {
		t.Errorf("this machine's row is not marked, texts %q", tt.Texts())
	}
}
