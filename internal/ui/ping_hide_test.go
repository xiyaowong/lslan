package ui

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestPingPaneClose: the ✕ appears once the session has stopped and
// clears the pane entirely; while the session runs there is no ✕ — the
// pane is the only output there is (issue #64).
func TestPingPaneClose(t *testing.T) {
	a := newApp()
	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	sess := &fakePingSession{samples: make(chan model.PingSample, 2)}
	a.startPingFn = func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) {
		return sess, nil
	}
	a.pingIP = host
	a.pingLines = []string{"来自 192.168.1.9: 字节=32 时间=1.0ms TTL=64"}
	a.pingStats = pingStats{sent: 1, recv: 1}
	a.selRow, a.selIP = 0, host
	tt := ui.NewTester(a.View, 1040, 720)

	// Stopped results offer the ✕, which closes the pane whole.
	tt.Frame()
	if !tt.HasText("已发送") {
		t.Fatalf("pane missing, texts %q", tt.Texts())
	}
	if err := tt.Click("关闭 Ping 结果"); err != nil {
		t.Fatalf("click close: %v", err)
	}
	tt.Frame()
	if tt.HasText("Ping 结果") {
		t.Errorf("close left the pane up, texts %q", tt.Texts())
	}

	// A running session has no ✕; 停止 Ping is the way out.
	a.startPing(host)
	sess.samples <- model.PingSample{OK: true, RTT: 2 * time.Millisecond, TTL: 64}
	deadline := time.Now().Add(time.Second)
	for a.pingStats.sent < 1 && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	tt.Frame()
	if err := tt.Click("关闭 Ping 结果"); err == nil {
		t.Error("a running session offered the close button")
	}
	a.stopPing()
	tt.Frame()
	if !tt.HasText("Ping 结果") {
		t.Error("stop dropped the results pane")
	}
	if err := tt.Click("关闭 Ping 结果"); err != nil {
		t.Fatalf("click close after stop: %v", err)
	}
	tt.Frame()
	if tt.HasText("Ping 结果") {
		t.Error("close left the pane up after stop")
	}
}
