package ui

import (
	"context"
	"errors"
	"net/netip"
	"slices"
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

// TestRowContextMenu right-clicks a row: the menu opens with the row's
// actions, right-clicking selects the row, and choosing an item acts on
// the row right-clicked.
func TestRowContextMenu(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.View, 1000, 680)

	ip1 := netip.MustParseAddr("192.168.1.5")
	ip2 := netip.MustParseAddr("192.168.1.6")
	a.hosts[ip1] = model.Host{IP: ip1, State: model.StateOnline, MAC: "AA:BB"}
	a.hosts[ip2] = model.Host{IP: ip2, State: model.StateOnline}
	// Select the second row, then right-click the first's IP cell.
	a.selRow, a.selIP = 1, ip2
	tt.Frame()
	r, ok := tt.Find(ip1.String())
	if !ok {
		t.Fatalf("row of %s not found", ip1)
	}
	tt.RightClickAt(r.X+2, r.Y+r.H/2)
	tt.Frame()

	want := []string{"复制 IP", "复制 MAC", "-", "Ping", "端口扫描", "浏览器打开"}
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if a.selIP != ip1 {
		t.Errorf("right-click selection = %s, want %s", a.selIP, ip1)
	}

	// A host without a MAC has that item grayed out, not hidden.
	tt.CloseMenu()
	tt.Frame()
	r, ok = tt.Find(ip2.String())
	if !ok {
		t.Fatalf("row of %s not found", ip2)
	}
	tt.RightClickAt(r.X+2, r.Y+r.H/2)
	tt.Frame()
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem("复制 MAC"); err == nil {
		t.Error("复制 MAC chosen though the host has no MAC")
	}
	if err := tt.ChooseMenuItem("复制 IP"); err != nil {
		t.Fatalf("choose 复制 IP: %v", err)
	}
	if tt.Clipboard() != ip2.String() {
		t.Errorf("clipboard = %q, want %q", tt.Clipboard(), ip2.String())
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

// TestDetailLayout: selecting a host leaves the table at least half the
// room and fills the detail panel — facts, actions, the port guide line —
// with no empty stretch (issue #17).
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
	head, ok := tt.Find("复制")
	if !ok {
		t.Fatal("detail header missing")
	}
	if head.Y < row.Y+row.H {
		t.Errorf("detail header at y=%v sits inside the table pane (row ends at %v)", head.Y, row.Y+row.H)
	}
	for _, s := range []string{"MAC", "厂商", "主机名", "上次在线", "开放端口", "点「端口扫描」"} {
		if !tt.HasText(s) {
			t.Errorf("%q missing from the detail panel, texts %q", s, tt.Texts())
		}
	}
}
