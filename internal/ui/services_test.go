package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestServiceRowWithoutAction: a service whose port has no next step — a
// Chromecast, an AirPlay speaker, a printer — shows its name and port
// alone. It once indexed an empty action list and took the app down.
func TestServiceRowWithoutAction(t *testing.T) {
	for _, svc := range []model.Service{
		{Name: "_googlecast._tcp", Port: 8009},
		{Name: "_airplay._tcp", Port: 5000},
		{Name: "_printer._tcp", Port: 9100},
		{Name: "_ipp._tcp", Port: 631},
	} {
		a := newApp()
		h := ip("192.168.1.7")
		a.hosts[h] = model.Host{IP: h, State: model.StateOnline, Services: []model.Service{svc}}
		a.ports[h] = nil // the sweep asked already
		a.selRow, a.selIP = 0, h
		tt := ui.NewTester(a.View, 1040, 720)

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s:%d panicked in the service row: %v", svc.Name, svc.Port, r)
				}
			}()
			tt.Frame()
		}()
		if !tt.HasText("网络服务") {
			t.Errorf("%s: the services section is missing", svc.Name)
		}
	}
}

// TestServiceRowWithAction: a mapped service keeps its next step.
func TestServiceRowWithAction(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.7")
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline, Services: []model.Service{
		{Name: "_http._tcp", Port: 80},
	}}
	a.ports[h] = nil
	a.selRow, a.selIP = 0, h
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	if !tt.HasText("网页服务") {
		t.Errorf("service label missing, texts %q", tt.Texts())
	}
}
