package ui

import (
	"context"
	"errors"
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
	if a.ipCells[0] != "192" || a.ipCells[1] != "168" || a.ipCells[2] != "1" ||
		a.ipCells[3] != "1" || a.ipCells[7] != "254" {
		t.Errorf("cells = %v, want 192.168.1.1..192.168.1.254", a.ipCells)
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

func (f *fakePingTerminal) setDark(bool) {}

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

// TestRowContextMenu: a row carries no context menu of its own; its
// actions live in the detail panel, each fact with a button copying it.
func TestRowContextMenu(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.View, 1000, 680)

	ip := netip.MustParseAddr("192.168.1.5")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline, MAC: "AA:BB", Hostname: "router"}
	tt.Frame()
	r, ok := tt.Find(ip.String())
	if !ok {
		t.Fatalf("row of %s not found", ip)
	}
	tt.RightClickAt(r.X+2, r.Y+r.H/2)
	tt.Frame()
	if got := tt.Menu(); len(got) != 0 {
		t.Errorf("menu = %q, want none", got)
	}

	// The detail panel copies a fact through its button.
	a.selRow, a.selIP = 0, ip
	tt.Frame()
	if err := tt.Click("复制MAC 地址"); err != nil {
		t.Fatalf("click copy: %v", err)
	}
	if tt.Clipboard() != "AA:BB" {
		t.Errorf("clipboard = %q, want AA:BB", tt.Clipboard())
	}
}

// bareTerminal mimics the plugin's terminal view: a labeled box that only
// paints, with no height of its own.
type bareTerminal struct {
	fakePingTerminal
}

func (bareTerminal) view(c *ui.Context) ui.Element {
	return ui.Box(c).Label("Terminal")
}

// TestPingTerminalHeight guards the terminal's size in the detail panel:
// the plugin view paints itself and has no height of its own, so without
// an explicit height it collapses to nothing (issue #15).
func TestPingTerminalHeight(t *testing.T) {
	a := newApp()
	sess := &fakePingSession{samples: make(chan model.PingSample)}
	a.startPingFn = func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) { return sess, nil }
	a.newPingTerm = func() (pingTerminalIface, error) { return &bareTerminal{}, nil }
	tt := ui.NewTester(a.View, 1000, 680)

	ip := netip.MustParseAddr("192.168.1.23")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	a.selRow, a.selIP = 0, ip
	a.startPing(ip)
	tt.Frame()

	r, ok := tt.Find("Terminal")
	if !ok {
		t.Fatal("terminal view missing from the detail panel")
	}
	// Before the fix the view had no height of its own and collapsed to 0.
	if r.H <= 0 || r.H > 140 {
		t.Errorf("terminal view height = %v, want within (0, 140]", r.H)
	}
}

// TestPingTerminalFallback: with no terminal, the line list stands in and
// shows the session's opening lines at once, before any sample arrives.
func TestPingTerminalFallback(t *testing.T) {
	a := newApp()
	a.startPingFn = func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) {
		return nil, errors.New("no engine in test")
	}
	a.newPingTerm = func() (pingTerminalIface, error) { return nil, errors.New("no terminal in test") }
	tt := ui.NewTester(a.View, 1000, 680)

	ip := netip.MustParseAddr("192.168.1.23")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	a.selRow, a.selIP = 0, ip
	a.startPing(ip)
	tt.Frame()

	for _, s := range []string{"打不开实时输出窗口", "Ping 没能启动"} {
		if !tt.HasText(s) {
			t.Errorf("%q missing, texts %q", s, tt.Texts())
		}
	}
}

// TestDetailLayout: selecting a host puts the detail panel beside the
// host tree, at its right, with the facts and the port guide line.
func TestDetailLayout(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.View, 1000, 680)

	ip := netip.MustParseAddr("192.168.1.23")
	a.hosts[ip] = model.Host{
		IP: ip, State: model.StateOnline,
		MAC: "AA:BB:CC:DD:EE:FF", Vendor: "小米", Hostname: "router",
		LastSeen: time.Now(),
	}
	a.selRow, a.selIP = 0, ip
	tt.Frame()

	row, ok := tt.Find(ip.String())
	if !ok {
		t.Fatal("table row missing")
	}
	if row.H <= 0 {
		t.Error("table row collapsed")
	}
	head, ok := tt.Find("关闭详情")
	if !ok {
		t.Fatalf("detail header missing, texts %q", tt.Texts())
	}
	if head.X <= row.X+row.W {
		t.Errorf("detail panel at x=%v sits beside the row ending at %v, want it at the right", head.X, row.X+row.W)
	}
	for _, s := range []string{"MAC 地址", "厂商", "主机名", "上次在线", "还没扫描过"} {
		if !tt.HasText(s) {
			t.Errorf("%q missing from the detail panel, texts %q", s, tt.Texts())
		}
	}
}
