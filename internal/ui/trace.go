package ui

import (
	"bufio"
	"context"
	"io"
	"net/netip"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/egoist/mygo/ui"
)

// traceStarter runs one route trace against ip and returns its output
// stream and a wait for the process's end; tests replace it. The wait
// must end when the traceStarter's ctx is cancelled, which is how 停止
// kills the process.
type traceStarter func(ctx context.Context, ip netip.Addr) (stream io.ReadCloser, wait func() error, err error)

// realTrace starts the system tracert: one hop line at a time, the wait
// reaping the process; cancelling ctx kills it. chcp 65001 puts the child
// on UTF-8 — tracert otherwise writes in the OEM code page (GBK on
// Chinese Windows), which garbles its Chinese lines in the terminal.
// CREATE_NO_WINDOW keeps the console off the screen.
func realTrace(ctx context.Context, ip netip.Addr) (io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, "cmd", "/c", "chcp 65001 >nul & tracert -d -w 1000 -h 15 "+ip.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	pid := cmd.Process.Pid
	// CommandContext kills only cmd.exe; tracert.exe is its child and
	// would run out its 15 hops on its own. The tree goes together.
	kill := func() { killTree(pid) }
	// Stopping is a cancel of ctx, so the tree has to go when it fires.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			kill()
		case <-done:
		}
	}()
	return stdout, func() error {
		err := cmd.Wait()
		close(done)
		return err
	}, nil
}

// killTree ends a process and everything it started, which is what a
// stopped tracert needs: the hops are a child of the cmd.exe shell.
func killTree(pid int) {
	kill := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = kill.Run()
}

// traceRoute runs the system tracert against ip, streaming each hop's
// output line into the pane as it lands; the reader goroutine ends when
// the process exits or ctx is cancelled (the process killed with it).
func (a *app) traceRoute(ip netip.Addr) {
	a.startTool()
	ctx, cancel := context.WithCancel(context.Background())
	gen := a.trace.start(ip, cancel)
	a.traceLines = nil
	if a.traceStarter == nil {
		a.traceStarter = realTrace
	}
	stdout, wait, err := a.traceStarter(ctx, ip)
	if err != nil {
		a.feedTrace("路由跟踪没能启动，请稍后再试", false)
		a.finishTrace(gen)
		return
	}
	go func() {
		defer cancel()
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 4096), 64*1024)
		for sc.Scan() {
			line := sc.Text()
			a.dispatch(func() {
				if a.trace.live(gen) {
					a.feedTrace(line, true)
				}
			})
		}
		if err := sc.Err(); err != nil && a.trace.live(gen) {
			a.dispatch(func() {
				if a.trace.live(gen) {
					a.feedTrace("读取路由输出时出错", false)
				}
			})
		}
		_ = wait()
		a.dispatch(func() {
			a.finishTrace(gen)
		})
	}()
}

func (a *app) feedTrace(line string, ok bool) {
	a.traceLines = append(a.traceLines, line)
}

// finishTrace marks the run complete; the ✕ appears once no run is in
// flight. A run the user has since restarted is left alone.
func (a *app) finishTrace(gen int) {
	if !a.trace.live(gen) {
		return
	}
	a.trace.finish()
}

// stopTrace ends the run; the pane stays up with the results until
// closeTrace clears it.
func (a *app) stopTrace() { a.trace.stop() }

// closeTrace clears the finished run's pane entirely; a run still going
// is stopped first, so its process never outlives its pane.
func (a *app) closeTrace() {
	a.stopTrace()
	a.trace.ip = netip.Addr{}
	a.traceLines = nil
}

// traceVisible reports whether the trace pane shows for ip.
func (a *app) traceVisible(ip netip.Addr) bool {
	return a.trace.forHost(ip) && len(a.traceLines) > 0
}

// tracePane is the route trace docked to the panel's foot: a title with
// the target, a stop button while the process runs, an ✕ once it has
// ended, then the hops as they land.
func (a *app) tracePane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(space(2)).Padding(10, 18, 12, 18).Shrink(0).Children(func() {
		ui.Row(c).Gap(space(2)).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "Tracert 结果")
			ui.Text(c, a.trace.ip.String()).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if a.trace.running() {
				if flatButton(c, "停止").Clicked() {
					a.stopTrace()
				}
			} else if iconButton(c, "关闭路由跟踪", "关闭", iconSize, drawClose).Clicked() {
				a.closeTrace()
			}
		})
		ui.List(c, &a.traceList, len(a.traceLines), func(i int) {
			ui.Text(c, a.traceLines[i]).Font("monospace").FontSize(12)
		}).Background(t.Background).Radius(6).Padding(10).Height(toolPaneHeight)
	})
}
