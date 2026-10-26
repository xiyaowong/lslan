package ui

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// fakeTelnetConn hands the link the reads the test scripts, in order,
// then hangs up; what the session writes back (negotiation answers) is
// recorded.
type fakeTelnetConn struct {
	net.Conn
	reads   [][]byte
	written bytes.Buffer
}

func (c *fakeTelnetConn) Read(b []byte) (int, error) {
	if len(c.reads) == 0 {
		return 0, io.EOF
	}
	r := c.reads[0]
	c.reads = c.reads[1:]
	return copy(b, r), nil
}

func (c *fakeTelnetConn) Write(b []byte) (int, error) { return c.written.Write(b) }

// filterRound runs one round of device bytes through the link's filter;
// the state it keeps carries across rounds, as across reads.
func filterRound(t *testing.T, l *telnetLink, in string) string {
	t.Helper()
	return string(l.filter([]byte(in)))
}

func newTestLink() (*telnetLink, *fakeTelnetConn) {
	c := &fakeTelnetConn{}
	return &telnetLink{Conn: c}, c
}

// TestTelnetFilter checks the negotiation stripping of the session's
// link: output passes, protocol bytes and CR NUL go, answers come back.
func TestTelnetFilter(t *testing.T) {
	l, conn := newTestLink()

	if got := filterRound(t, l, "hello\r\n"); got != "hello\r\n" {
		t.Errorf("plain output = %q", got)
	}
	// WILL ECHO is taken up; IAC IAC is a data 255.
	if got := filterRound(t, l, "\xff\xfb\x01a\xff\xffb"); got != "a\xffb" {
		t.Errorf("negotiation + IAC IAC = %q", got)
	}
	if want := "\xff\xfd\x01"; conn.written.String() != want {
		t.Errorf("answers = % x, want % x", conn.written.Bytes(), []byte(want))
	}
	// WILL of anything else is declined, WONT would need no answer.
	conn.written.Reset()
	if got := filterRound(t, l, "\xff\xfb\x2a"); got != "" {
		t.Errorf("declined WILL left %q", got)
	}
	if want := "\xff\xfe\x2a"; conn.written.String() != want {
		t.Errorf("decline = % x, want % x", conn.written.Bytes(), []byte(want))
	}
	// DO is refused with WONT.
	conn.written.Reset()
	if got := filterRound(t, l, "\xff\xfd\x18"); got != "" {
		t.Errorf("DO left %q", got)
	}
	if want := "\xff\xfc\x18"; conn.written.String() != want {
		t.Errorf("refusal = % x, want % x", conn.written.Bytes(), []byte(want))
	}
	// A subnegotiation is skipped whole; CR NUL becomes CR.
	if got := filterRound(t, l, "x\xff\xfa\x18\x01\xff\xf0y\r\x00z"); got != "xy\rz" {
		t.Errorf("subnegotiation = %q", got)
	}
	// The CR flag holds through a negotiation: CR, then IAC, then output
	// keeps CR only when a NUL follows it, not before other bytes.
	if got := filterRound(t, l, "a\r\xff\xfb\x01b"); got != "a\rb" {
		t.Errorf("CR before IAC = %q", got)
	}
	if got := filterRound(t, l, "c\r\xff\xfb\x01\x00d"); got != "c\rd" {
		t.Errorf("CR NUL across IAC = %q", got)
	}
	// A sequence split across reads still parses.
	l2, _ := newTestLink()
	if got := filterRound(t, l2, "a\xff"); got != "a" {
		t.Errorf("half 1 = %q", got)
	}
	if got := filterRound(t, l2, "\xfb\x01b"); got != "b" {
		t.Errorf("half 2 = %q", got)
	}
	// A read of all negotiation keeps reading rather than report EOF...
	l3, c3 := newTestLink()
	c3.reads = [][]byte{[]byte("\xff\xfd\x22"), []byte("ok")}
	b := make([]byte, 64)
	n, err := l3.Read(b)
	if err != nil || string(b[:n]) != "ok" {
		t.Errorf("all-negotiation read = %q, %v; want ok", b[:n], err)
	}
	// ...and the hangup says goodbye once.
	n, err = l3.Read(b)
	if err != nil || string(b[:n]) != string(telnetBye) {
		t.Errorf("farewell = %q, %v", b[:n], err)
	}
	if _, err = l3.Read(b); err != io.EOF {
		t.Errorf("after farewell = %v, want EOF", err)
	}
}

// fakeTelnetTerm stands in for the plugin's terminal view.
type fakeTelnetTerm struct {
	closed bool
}

func (f *fakeTelnetTerm) view(c *ui.Context) ui.Element { return ui.Box(c).Label("Telnet") }
func (f *fakeTelnetTerm) close()                        { f.closed = true }
func (f *fakeTelnetTerm) setDark(bool)                  {}

// drain waits for the dial goroutine to report back, as the view's
// pending queue would.
func drain(t *testing.T, a *app) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.runPending()
		if !a.telnetPending {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("dial never reported back")
}

// TestTelnetSession: the one-click test lands in the pane, hangs up on
// 断开, and says why when the device does not answer.
func TestTelnetSession(t *testing.T) {
	a := newApp()
	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	ft := &fakeTelnetTerm{}
	a.dialTelnet = func(ip netip.Addr, port int) (net.Conn, error) { return &fakeTelnetConn{}, nil }
	a.newTelnet = func(net.Conn) (telnetTermIface, error) { return ft, nil }
	tt := ui.NewTester(a.View, 1000, 680)

	a.startTelnet(host, 23)
	drain(t, a)
	a.selRow, a.selIP = 0, host
	tt.Frame()
	if !tt.HasText("远程终端测试") {
		t.Errorf("pane missing, texts %q", tt.Texts())
	}
	if a.telnetTerm != ft {
		t.Error("session terminal not set")
	}
	if err := tt.Click("断开"); err != nil {
		t.Fatalf("click hang up: %v", err)
	}
	if !ft.closed || a.telnetTerm != nil {
		t.Error("session not closed on hang up")
	}

	// A device that refuses says so in the pane.
	a.dialTelnet = func(ip netip.Addr, port int) (net.Conn, error) { return nil, net.ErrClosed }
	a.startTelnet(host, 23)
	drain(t, a)
	tt.Frame()
	if !tt.HasText("连接 192.168.1.9:23 失败") {
		t.Errorf("failure missing, texts %q", tt.Texts())
	}
}

// TestTelnetDialError checks the dial failure words: a timeout reads as
// no answer, a refusal as the port being closed.
func TestTelnetDialError(t *testing.T) {
	host := ip("192.168.1.9")
	timeout := &fakeNetError{timeout: true}
	if got := telnetDialError(timeout, host, 23); got != "192.168.1.9:23 没有响应，设备可能已关机" {
		t.Errorf("timeout = %q", got)
	}
	if got := telnetDialError(syscall.ECONNREFUSED, host, 23); got != "192.168.1.9:23 拒绝了连接，远程登录可能没有开启" {
		t.Errorf("refused = %q", got)
	}
	if got := telnetDialError(net.ErrClosed, host, 23); got != "连接 192.168.1.9:23 失败，设备可能没开远程登录" {
		t.Errorf("other = %q", got)
	}
}

// fakeNetError is a net.Error with a set timeout flag.
type fakeNetError struct{ timeout bool }

func (e *fakeNetError) Error() string   { return "fake" }
func (e *fakeNetError) Timeout() bool   { return e.timeout }
func (e *fakeNetError) Temporary() bool { return false }
