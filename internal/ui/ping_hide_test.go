package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestPingPaneHide: the dash folds the pane's output away while the
// session runs on; a new session or a stop brings it back (issue #64).
func TestPingPaneHide(t *testing.T) {
	a := newApp()
	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	a.pingIP = host
	a.pingLines = []string{"来自 192.168.1.9: 字节=32 时间=1.0ms TTL=64"}
	a.pingStats = pingStats{sent: 1, recv: 1}
	a.selRow, a.selIP = 0, host
	tt := ui.NewTester(a.View, 1040, 720)

	tt.Frame()
	if !tt.HasText("已发送") {
		t.Fatalf("pane missing, texts %q", tt.Texts())
	}
	if err := tt.Click("隐藏 Ping 输出"); err != nil {
		t.Fatalf("click hide: %v", err)
	}
	tt.Frame()
	if tt.HasText("已发送") || !tt.HasText("Ping 结果") {
		t.Errorf("hide did not fold the output, texts %q", tt.Texts())
	}
	// A new session shows it again.
	a.startPing(host)
	tt.Frame()
	if a.pingHidden {
		t.Error("a new session kept the pane hidden")
	}
	// Stop ends the pane altogether.
	a.stopPing()
	tt.Frame()
	if tt.HasText("Ping 结果") {
		t.Error("stop left the pane up")
	}
}
