package ui

import (
	"context"
	"io"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// fakeTraceStream hands out its scripted lines once, then blocks until
// closed (as a running tracert does between hops); the wait blocks until
// then too.
type fakeTraceStream struct {
	lines  []string
	closed chan struct{}
	once   sync.Once
	read   int
}

func (f *fakeTraceStream) Read(b []byte) (int, error) {
	if f.read < len(f.lines) {
		n := copy(b, f.lines[f.read]+"\r\n")
		f.read++
		return n, nil
	}
	<-f.closed
	return 0, io.EOF
}

func (f *fakeTraceStream) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

// waitTrace waits for the trace goroutine to report back.
func waitTrace(t *testing.T, a *app, done bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.runPending()
		if done == (a.traceCancel == nil) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("trace never settled")
}

// TestTracePane: the run streams into the pane, offers 停止 while the
// process is up, and an ✕ once it has ended.
func TestTracePane(t *testing.T) {
	a := newApp()
	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	a.selRow, a.selIP = 0, host
	stream := &fakeTraceStream{lines: []string{"跳数 1  192.168.1.1"}, closed: make(chan struct{})}
	var out io.ReadCloser = stream
	a.traceStarter = func(ctx context.Context, ip netip.Addr) (io.ReadCloser, func() error, error) {
		go func() {
			<-ctx.Done()
			stream.Close()
		}()
		return out, func() error { return nil }, nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host

	a.traceRoute(host)
	waitTrace(t, a, false)
	deadline := time.Now().Add(time.Second)
	for len(a.traceLines) == 0 && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText("路由跟踪") {
		t.Fatalf("trace pane missing, texts %q", tt.Texts())
	}
	// Running: the ✕ is absent (the pane is the only output); 停止 ends it.
	if err := tt.Click("关闭路由跟踪"); err == nil {
		t.Error("a running trace offered the close button")
	}
	if err := tt.Click("停止"); err != nil {
		t.Fatalf("stop button missing: %v", err)
	}
	waitTrace(t, a, true) // the killed process reports back
	tt.Frame()
	// After the stop the pane stays up with what landed; the ✕ closes it.
	if err := tt.Click("关闭路由跟踪"); err != nil {
		t.Fatalf("close button missing after stop: %v", err)
	}
	tt.Frame()
	if tt.HasText("路由跟踪") {
		t.Error("close left the trace pane up")
	}
}

// TestTraceRun ends by itself when the process does, its lines reaching
// the pane; the ✕ then closes it.
func TestTraceRun(t *testing.T) {
	a := newApp()
	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	script := strings.Join([]string{
		"通过最多 30 个跃点跟踪",
		"  1    1 ms    1 ms    1 ms  192.168.1.1",
		"跟踪完成。",
	}, "\r\n")
	pr := io.NopCloser(strings.NewReader(script))
	a.traceStarter = func(context.Context, netip.Addr) (io.ReadCloser, func() error, error) {
		return pr, func() error { return nil }, nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host

	a.traceRoute(host)
	waitTrace(t, a, true)
	a.runPending()
	tt.Frame()
	if a.traceCancel != nil {
		t.Error("the run did not end by itself")
	}
	if err := tt.Click("关闭路由跟踪"); err != nil {
		t.Fatalf("close missing after end: %v", err)
	}
	tt.Frame()
	if tt.HasText("路由跟踪") {
		t.Error("close left the trace pane up")
	}
}

// TestTraceRealProcess runs tracert against localhost, which answers in
// one hop, to pin that real process output reaches the pane and the run
// ends by itself.
func TestTraceRealProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("the real tracert takes seconds")
	}
	a := newApp()
	host := netip.MustParseAddr("127.0.0.1")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	tt := ui.NewTester(a.View, 1040, 720)

	a.traceRoute(host)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		a.runPending()
		if a.traceCancel == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	tt.Frame()
	if a.traceCancel != nil {
		t.Fatal("the run did not end by itself")
	}
	if len(a.traceLines) == 0 {
		t.Error("no trace output reached the pane")
	}
}
