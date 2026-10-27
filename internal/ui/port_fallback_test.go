package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestPortRowWithoutActionCopiesAddress: a port with no known next step
// still offers something to do with it.
func TestPortRowWithoutActionCopiesAddress(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.7")
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline}
	a.ports[h] = []model.Port{{Number: 1234, Open: true}}
	a.selRow, a.selIP = 0, h
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()

	if !tt.HasText("其他服务") {
		t.Errorf("an unknown port reads in English or not at all, texts %q", tt.Texts())
	}
	if err := tt.Click("复制地址"); err != nil {
		t.Fatalf("click copy: %v", err)
	}
	if got := tt.Clipboard(); got != "http://192.168.1.7:1234" {
		t.Errorf("clipboard = %q", got)
	}
}
