package ui

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// A home user's real question about a device is "why can't I reach it",
// and the answer takes three probes to tell apart: nothing answers at
// all, it answers ping but no ports are open, or it answers both and the
// problem is elsewhere. 查一查 runs them in order and says which one it
// found.
const (
	diagnosePings    = 4
	diagnosePingWait = 1200 * time.Millisecond
)

// diagnoseState is one run of 查一查: the step it is on, what each step
// found, and the sentence they add up to.
type diagnoseState struct {
	run    toolRun
	ip     netip.Addr
	step   string   // the words of the step in progress
	notes  []string // what each finished step found
	answer string   // the conclusion, once there is one
	// ports that were probed, to say how many were open.
	openPorts int
}

// running reports whether a diagnosis is in progress.
func (d *diagnoseState) running() bool { return d.run.running() }

// startDiagnose asks the app's own probes, one after another, then words
// the result. It ends the other tools first: the foot shows one thing.
func (a *app) startDiagnose(ip netip.Addr) {
	a.startTool()
	a.closeDiagnose()
	d := &a.diagnose
	ctx, cancel := context.WithCancel(context.Background())
	gen := d.run.start(ip, cancel)
	d.ip = ip
	d.step, d.notes, d.answer, d.openPorts = "正在 Ping…", nil, "", 0

	go func() {
		// Step 1: does it answer at all?
		answered := 0
		for i := 0; i < diagnosePings; i++ {
			if ctx.Err() != nil {
				return
			}
			if a.diagnosePingFn(ctx, ip) {
				answered++
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(diagnosePingWait):
			}
		}
		a.dispatch(func() {
			if !d.run.live(gen) {
				return
			}
			if answered > 0 {
				d.notes = append(d.notes, fmt.Sprintf("Ping 通了（%d/%d）", answered, diagnosePings))
				d.step = "正在查看端口…"
			} else {
				d.notes = append(d.notes, "Ping 没有回应")
				d.answer = "这台设备完全没有响应：可能关机了，也可能不在同一个网络。"
				a.finishDiagnose(gen)
			}
		})
		if answered == 0 {
			return
		}

		// Step 2: is anything listening?
		ctxPorts, cancelPorts := context.WithTimeout(ctx, 8*time.Second)
		ports, err := a.scanPorts(ctxPorts, a.sourceAddr(), ip, a.presets()[0].Ports)
		cancelPorts()
		if ctx.Err() != nil {
			return
		}
		open := 0
		for _, p := range ports {
			if p.Open {
				open++
			}
		}
		a.dispatch(func() {
			if !d.run.live(gen) {
				return
			}
			d.openPorts = open
			switch {
			case err != nil:
				d.notes = append(d.notes, "端口没能查完")
			case open == 0:
				d.notes = append(d.notes, "常用端口都关着")
			default:
				d.notes = append(d.notes, fmt.Sprintf("开放 %d 个端口", open))
			}
			d.answer = diagnoseAnswer(answered, open, err)
			a.finishDiagnose(gen)
		})
	}()
}

// diagnoseAnswer turns what the two steps found into one sentence a home
// user can act on.
func diagnoseAnswer(pinged, open int, err error) string {
	switch {
	case err != nil:
		return "Ping 是通的，但端口没查完。可以稍后重试，或直接用它提供的服务。"
	case open > 0:
		return "设备在线，也有服务在跑。如果还是连不上，问题多半在应用或权限，不在网络。"
	default:
		return "Ping 能通但常用端口都关着：设备在线，可能有防火墙，或者服务没开。"
	}
}

// finishDiagnose ends the run without discarding its answer.
func (a *app) finishDiagnose(gen int) {
	d := &a.diagnose
	if !d.run.live(gen) {
		return
	}
	d.step = ""
	d.run.finish()
}

// closeDiagnose puts the pane away entirely.
func (a *app) closeDiagnose() {
	d := &a.diagnose
	d.run.clear()
	d.ip = netip.Addr{}
	d.step, d.notes, d.answer, d.openPorts = "", nil, "", 0
}

// diagnoseVisible reports whether the pane has anything to show for ip.
func (a *app) diagnoseVisible(ip netip.Addr) bool {
	d := &a.diagnose
	return d.run.forHost(ip) && (d.step != "" || d.answer != "" || len(d.notes) > 0)
}

// diagnosePane is the diagnosis docked to the panel's foot: the step in
// progress or the steps that ran, then the conclusion.
func (a *app) diagnosePane(c *ui.Context, t *ui.Theme) {
	d := &a.diagnose
	ui.Column(c).Gap(space(2)).Padding(space(2), 18, space(3), 18).Shrink(0).Children(func() {
		ui.Row(c).Gap(space(2)).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "查一查")
			ui.Text(c, d.ip.String()).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if d.running() {
				if flatButton(c, "停止").Clicked() {
					a.stopDiagnose()
				}
			} else if iconButton(c, "关闭诊断结果", "关闭", iconSize, drawClose).Clicked() {
				a.closeDiagnose()
			}
		})
		if d.step != "" {
			ui.Row(c).Gap(space(2)).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, d.step).FontSize(12).TextColor(t.TextMuted)
			})
		}
		for _, n := range d.notes {
			ui.Text(c, "· "+n).FontSize(12).TextColor(t.TextMuted)
		}
		if d.answer != "" {
			ui.Text(c, d.answer).FontSize(13).Wrap()
		}
	})
}

// stopDiagnose ends the run and clears the pane; safe with none running.
func (a *app) stopDiagnose() {
	d := &a.diagnose
	d.run.stop()
	d.step = ""
}

// diagnoseText is the one-line version for a copy action or a test.
func (a *app) diagnoseText() string {
	d := &a.diagnose
	if d.answer == "" {
		return ""
	}
	return strings.Join(append(append([]string{}, d.notes...), d.answer), "；")
}
