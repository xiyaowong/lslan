package ui

import (
	"errors"
	"net/netip"
	"strings"

	"lslan/internal/model"
	"lslan/internal/scan"
)

func (a *app) startScan() {
	if a.scanning {
		return
	}
	nic := a.selectedNIC()
	if nic == nil {
		a.scanErr = "请先选择网卡"
		return
	}
	start, end, ok := a.parseRange()
	if !ok {
		a.scanErr = "请填写完整的开始和结束 IP"
		return
	}
	n, _ := rangeCount(start, end)
	if n > rangeLimit {
		a.scanErr = "范围过大，请先缩小范围"
		return
	}
	if a.scanner == nil {
		a.scanner = a.newScanner()
	}
	a.startPump()
	if err := a.scanner.Start(*nic, start, end); err != nil {
		a.scanErr = scanStartError(err)
		return
	}
	// The sweep clears the table and the selection, which would hide the
	// foot's pane and with it every way to stop a running tool.
	stopped := a.stopTools()
	a.scanning = true
	a.scanStopping = false
	a.scanCancelled = false
	a.scanErr = ""
	if stopped != "" {
		a.scanNote = "已停止 " + stopped + "，开始新的扫描"
	}
	a.progress = model.Progress{Total: int(n)}
	a.hosts = map[netip.Addr]model.Host{}
	a.ports = map[netip.Addr][]model.Port{}
	a.portGen++ // a sweep still probing ports is stale now
	a.sweepLeft = 0
	a.sweepPending = nil
	a.netbiosReset() // a device may have been renamed since
	a.selRow, a.selIP = -1, netip.Addr{}
}

// stopTools ends every tool that outlives a selection — ping, the telnet
// session, the route trace — and reports which ones were running, so the
// user learns why their output disappeared.
func (a *app) stopTools() string {
	var running []string
	if a.ping.cancel != nil {
		running = append(running, "Ping")
	}
	if a.telnetPending || a.telnetTerm != nil {
		running = append(running, "远程终端")
	}
	if a.trace.running() {
		running = append(running, "路由跟踪")
	}
	a.stopPing() // ends the telnet test too
	a.stopTrace()
	a.closePing()
	a.closeTrace()
	if len(running) == 0 {
		return ""
	}
	return strings.Join(running, "、")
}

// scanStartError says why a sweep was refused in words the user can act
// on: the engine's sentinels mean different things.
func scanStartError(err error) string {
	switch {
	case errors.Is(err, scan.ErrBusy):
		return "正在扫描，请等待当前扫描完成"
	case errors.Is(err, scan.ErrRangeReversed):
		return "开始地址不能大于结束地址"
	default:
		return "无法开始扫描，请检查网卡后重试"
	}
}

// rangeFits reports whether the range is one the app is willing to scan.
func (a *app) rangeFits() bool {
	n, ok := a.rangeTotal()
	return ok && n <= rangeLimit
}

// cancelScan asks the engine to stop and says so at once, since the sweep
// can take a while to unwind and a dead button reads as a broken one.
func (a *app) cancelScan() {
	if !a.scanning || a.scanStopping {
		return
	}
	a.scanStopping = true
	a.scanCancelled = true
	a.scanner.Cancel()
}

// startPump drains the scanner's event stream once, for its lifetime: the
// channel stays open across sweeps.
func (a *app) startPump() {
	a.pumpOnce.Do(func() {
		events := a.scanner.Events()
		go func() {
			for ev := range events {
				a.dispatch(func() { a.applyEvent(ev) })
			}
		}()
	})
}

// applyEvent folds one engine event into the state; main thread only.
func (a *app) applyEvent(ev scan.Event) {
	switch ev.Kind {
	case scan.EventProgress:
		a.progress = ev.Progress
	case scan.EventHost:
		if ev.Host != nil {
			a.hosts[ev.Host.IP] = *ev.Host
		}
	case scan.EventDone:
		a.scanning, a.scanStopping = false, false
		a.progress = ev.Progress
		cancelled := a.scanCancelled
		a.scanCancelled = false
		switch {
		case cancelled:
			// The button and the progress line already said so.
		case a.withPorts:
			a.sweepPorts()
		}
		if !cancelled {
			// A finished sweep shows its result: the row count, the
			// table itself. Only a change from last time is worth words.
			a.noteChanges()
		}
	case scan.EventError:
		a.scanning = false
		a.scanErr = "扫描中断，已停止"
	}
}
