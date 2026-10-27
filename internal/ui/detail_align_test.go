package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestPortListHeight: five ports fit without a scrollbar and without the
// last row being clipped. Sizing by rows alone forgot the dividers.
func TestPortListHeight(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.7")
	var ports []model.Port
	for _, n := range []int{80, 443, 445, 3389, 8080} {
		ports = append(ports, model.Port{Number: n, Open: true})
	}
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline}
	a.ports[h] = ports
	a.selRow, a.selIP = 0, h
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()

	first, ok := tt.Find("80")
	if !ok {
		t.Fatal("the first port is missing")
	}
	last, ok := tt.Find("8080")
	if !ok {
		t.Fatal("the fifth port is missing")
	}
	if last.Y+last.H > first.Y+float32(5*portRowHeight+4) {
		t.Errorf("the fifth row bottoms out at %v, past the pane", last.Y+last.H)
	}
}

// TestServiceAndPortAlign: the service names and the port service names
// start on one line, so the two lists do not look offset from each other.
func TestServiceAndPortAlign(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.7")
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline,
		Services: []model.Service{{Name: "_ipp._tcp", Port: 631}},
	}
	a.ports[h] = []model.Port{{Number: 80, Open: true, Service: "http"}}
	a.selRow, a.selIP = 0, h
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()

	// "网页" is the port list's service word; "打印服务" the services'.
	port, ok := tt.Find("网页")
	if !ok {
		t.Fatal("port service word missing")
	}
	svc, ok := tt.Find("打印服务")
	if !ok {
		t.Fatal("service word missing")
	}
	if svc.X != port.X {
		t.Errorf("service word at x=%v, port's at x=%v; they should line up", svc.X, port.X)
	}
}
