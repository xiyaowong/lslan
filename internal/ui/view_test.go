package ui

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// Smoke test: the view renders without error and NIC loading fills the
// range. Engine calls are faked; no real scan runs.
func TestSmoke(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.0"), netip.MustParseAddr("192.168.1.255")
	}
	tt := ui.NewTester(a.View, 1000, 680)
	a.loadNICs()
	tt.Frame()
	if !tt.HasText("以太网 (192.168.1.23)") {
		t.Errorf("NIC option missing, texts %q", tt.Texts())
	}
	if a.startIP != "192.168.1.1" || a.endIP != "192.168.1.254" {
		t.Errorf("range = %s..%s, want 192.168.1.1..192.168.1.254", a.startIP, a.endIP)
	}
}

// fakePingTerminal records fed lines and shows them as plain text.
type fakePingTerminal struct {
	lines  []string
	closed bool
}

func (f *fakePingTerminal) feed(line string, ok bool) {
	if !ok {
		line = "× " + line
	}
	f.lines = append(f.lines, line)
}

func (f *fakePingTerminal) view(c *ui.Context) ui.Element {
	return ui.Column(c).Gap(2).Children(func() {
		for _, l := range f.lines {
			ui.Text(c, l).Font("monospace")
		}
	})
}

func (f *fakePingTerminal) close() { f.closed = true }

type fakePingSession struct {
	samples chan model.PingSample
}

func (s *fakePingSession) Samples() <-chan model.PingSample { return s.samples }
func (s *fakePingSession) Cancel()                          {}

// TestPing runs a ping session against a fake terminal: samples land in
// the terminal and the stats line, stopping closes the terminal.
func TestPing(t *testing.T) {
	a := newApp()
	sess := &fakePingSession{samples: make(chan model.PingSample, 8)}
	a.startPingFn = func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) { return sess, nil }
	ft := &fakePingTerminal{}
	a.newPingTerm = func() (pingTerminalIface, error) { return ft, nil }
	tt := ui.NewTester(a.View, 1000, 680)

	ip := netip.MustParseAddr("192.168.1.23")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	a.selRow, a.selIP = 0, ip
	a.startPing(ip)
	sess.samples <- model.PingSample{OK: true, RTT: 2 * time.Millisecond, TTL: 64}
	sess.samples <- model.PingSample{OK: false}
	close(sess.samples)
	deadline := time.Now().Add(time.Second)
	for a.pingStats.sent < 2 && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	tt.Frame()

	if !tt.HasText("已发送 2 · 已接收 1 · 丢包率 50% · 平均 2.0ms") {
		t.Errorf("stats line missing, texts %q", tt.Texts())
	}
	want := []string{"正在 Ping 192.168.1.23:", "来自 192.168.1.23: 字节=32 时间=2.0ms TTL=64", "× 请求超时。"}
	if len(ft.lines) != len(want) {
		t.Fatalf("terminal lines = %q, want %q", ft.lines, want)
	}
	for i, l := range want {
		if ft.lines[i] != l {
			t.Errorf("terminal line %d = %q, want %q", i, ft.lines[i], l)
		}
	}
	if ft.closed {
		t.Error("terminal closed while session is live")
	}

	a.stopPing()
	if !ft.closed {
		t.Error("terminal not closed on stop")
	}
	if a.pingTerm != nil {
		t.Error("pingTerm not cleared on stop")
	}
	tt.Frame()
}
