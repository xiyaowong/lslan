package ui

import (
	"bufio"
	"context"
	"io"
	"net/netip"
	"os/exec"
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
	return stdout, cmd.Wait, nil
}

// traceRoute runs the system tracert against ip, streaming each hop's
// output line into the pane as it lands; the reader goroutine ends when
// the process exits or ctx is cancelled (the process killed with it).
func (a *app) traceRoute(ip netip.Addr) {
	a.stopPing()
	a.stopTelnet()
	a.closePing() // the foot holds one output at a time
	a.traceGen++
	gen := a.traceGen
	ctx, cancel := context.WithCancel(context.Background())
	a.traceCancel = cancel
	a.traceIP = ip
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
				if a.traceGen == gen {
					a.feedTrace(line, true)
				}
			})
		}
		if err := sc.Err(); err != nil && a.traceGen == gen {
			a.dispatch(func() {
				if a.traceGen == gen {
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

// feedTrace shows one output line in the pane's terminal, or the fallback
// line list; the ping pane's terminal is reused, so both live tools share
// one interactive area at a time.
func (a *app) feedTrace(line string, ok bool) {
	if a.traceTerm != nil {
		a.traceTerm.feed(line, ok)
		return
	}
	a.traceLines = append(a.traceLines, line)
}

// finishTrace marks the run complete; the ✕ appears once traceCancel is
// nil, as the ping pane's does. The gen check drops a run the user has
// since restarted.
func (a *app) finishTrace(gen int) {
	if a.traceGen != gen {
		return
	}
	if a.traceCancel != nil {
		cancel := a.traceCancel
		a.traceCancel = nil
		cancel()
	}
	if a.traceTerm != nil {
		a.traceTerm.close()
		a.traceTerm = nil
	}
}

// stopTrace ends the run, killing the process; the pane stays up with the
// results until closeTrace clears it.
func (a *app) stopTrace() {
	a.traceGen++
	if a.traceCancel != nil {
		cancel := a.traceCancel
		a.traceCancel = nil
		cancel() // kills the process via CommandContext
	}
	if a.traceTerm != nil {
		a.traceTerm.close()
		a.traceTerm = nil
	}
}

// closeTrace clears the finished run's pane entirely; a run still going
// is stopped first, so its process never outlives its pane.
func (a *app) closeTrace() {
	a.stopTrace()
	a.traceIP = netip.Addr{}
	a.traceLines = nil
	a.traceDone = false
	if a.traceTerm != nil {
		a.traceTerm.close()
		a.traceTerm = nil
	}
}

// traceVisible reports whether the trace pane shows for ip.
func (a *app) traceVisible(ip netip.Addr) bool {
	return a.traceIP == ip && (a.traceTerm != nil || len(a.traceLines) > 0)
}

// tracePane is the route trace docked to the panel's foot: a title with
// the target, a stop button while the process runs, an ✕ once it has
// ended, then the hops as they land.
func (a *app) tracePane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(8).Padding(10, 18, 12, 18).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "Tracert 结果")
			ui.Text(c, a.traceIP.String()).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if a.traceCancel != nil {
				if flatButton(c, "停止").Clicked() {
					a.stopTrace()
				}
			} else if iconButton(c, "关闭路由跟踪", "关闭", 16, drawClose).Clicked() {
				a.closeTrace()
			}
		})
		if a.traceTerm != nil {
			// The terminal view paints itself and has no height of its own:
			// without an explicit height it collapses to nothing.
			ui.Box(c).Background(t.Background).Radius(6).Clip().Children(func() {
				a.traceTerm.view(c).Height(140)
			})
		} else {
			ui.List(c, &a.traceList, len(a.traceLines), func(i int) {
				ui.Text(c, a.traceLines[i]).Font("monospace").FontSize(12)
			}).Background(t.Background).Radius(6).Padding(10).Height(140)
		}
	})
}
