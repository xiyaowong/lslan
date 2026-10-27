package ui

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestNICLoadFailureOffersRetry: a failed adapter read says so and offers
// a retry. It once left the row on "正在加载网卡…" forever, contradicting
// the error in the status bar.
func TestNICLoadFailureOffersRetry(t *testing.T) {
	a := newApp()
	fail := true
	a.listNICs = func() ([]model.NIC, error) {
		if fail {
			return nil, errors.New("access denied")
		}
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()
	a.runPending()
	tt.Frame()

	if tt.HasText("正在加载网卡") {
		t.Errorf("a failed read still reads as loading, texts %q", tt.Texts())
	}
	if !tt.HasText("没读到网卡") {
		t.Fatalf("no failure words, texts %q", tt.Texts())
	}

	// Retry succeeds this time, and the adapter shows up.
	fail = false
	if err := tt.Click("重试"); err != nil {
		t.Fatalf("click retry: %v", err)
	}
	tt.Frame()
	a.runPending()
	tt.Frame()
	if !tt.HasText("以太网 (192.168.1.23)") {
		t.Errorf("the adapter did not load on retry, texts %q", tt.Texts())
	}
}
