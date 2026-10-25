package ui

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// detail fills the split's lower pane for the selected host: one dense
// inspector — header, facts, actions, then the ping and port results —
// with no stretch of empty space. It scrolls when a ping or a long port
// list takes more room than the pane has.
func (a *app) detail(c *ui.Context) {
	h, ok := a.hosts[a.selIP]
	if !ok {
		return
	}
	ui.Scroll(c).Fill().Padding(10, 16).Gap(10).Children(func() {
		a.detailHeader(c, h)
		a.detailInfo(c, h)
		a.detailActions(c, h)
		a.pingTerminal(c, h)
		a.portSection(c, h)
	})
}

// detailHeader is one line that reads at a glance: status dot, IP, the
// gateway badge, and a copy button.
func (a *app) detailHeader(c *ui.Context, h model.Host) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		dot := t.Border
		switch h.State {
		case model.StateOnline:
			dot = t.Success
		case model.StateOffline:
			dot = t.TextMuted
		}
		ui.Box(c).Size(9, 9).Radius(4.5).Background(dot).Shrink(0)
		ui.Text(c, h.IP.String()).FontSize(16).Bold().Font("monospace")
		if h.Gateway {
			pill(c, "网关", t.Accent, t.AccentText)
		}
		ui.Box(c).Grow(1)
		if ui.Button(c, "复制").Clicked() {
			c.WriteClipboard(h.IP.String())
		}
	})
}

// detailInfo is the facts, two columns of label and value.
func (a *app) detailInfo(c *ui.Context, h model.Host) {
	t := c.Theme()
	open := 0
	for _, p := range h.Ports {
		if p.Open {
			open++
		}
	}
	ports := "—"
	if len(h.Ports) > 0 {
		ports = fmt.Sprintf("%d 个开放，共 %d 个", open, len(h.Ports))
	}
	seen := "—"
	if !h.LastSeen.IsZero() {
		seen = h.LastSeen.Format("01-02 15:04")
	}
	type fact struct{ label, value string }
	ui.Grid(c).Columns(2).GapX(24).GapY(6).Shrink(0).Children(func() {
		for _, f := range [5]fact{
			{"MAC", orDash(h.MAC)},
			{"厂商", orDash(h.Vendor)},
			{"主机名", orDash(h.Hostname)},
			{"上次在线", seen},
			{"开放端口", ports},
		} {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.label).FontSize(12).Width(60).TextColor(t.TextMuted)
				ui.Text(c, f.value).FontSize(12).SingleLine()
			})
		}
	})
}

// detailActions is the row's actions; the presets beyond the common one
// wait behind 更多, out of the main sight.
func (a *app) detailActions(c *ui.Context, h model.Host) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		pinging := a.pingCancel != nil && a.pingIP == h.IP
		if pinging {
			if ui.Button(c, "停止 Ping").Clicked() {
				a.stopPing()
			}
		} else if ui.Button(c, "Ping").Clicked() {
			a.startPing(h.IP)
		}

		scanBtn := ui.Button(c, "端口扫描").Disabled(a.portScanning)
		if !a.portScanning && scanBtn.Clicked() {
			a.startPortScan(h.IP)
		}
		more := ui.Button(c, "更多")
		if more.Clicked() {
			a.moreOpen = !a.moreOpen
		}
		ui.Popover(c, more, &a.moreOpen, func() {
			ui.Column(c).Padding(6).Gap(4).Children(func() {
				for i, p := range a.presets() {
					if i == 0 {
						continue
					}
					name := p.Name
					if ui.Button(c, name+"端口").Clicked() {
						a.moreOpen = false
						a.startPortScanPreset(h.IP, i)
					}
				}
			})
		})

		if ui.Button(c, "浏览器打开").Clicked() {
			c.OpenURL("http://" + h.IP.String())
		}
	})
}

// pingTerminal streams the ping output of the selected host — the
// terminal plugin view when one was created, else a plain line list —
// and only while pinging or with lines to show.
func (a *app) pingTerminal(c *ui.Context, h model.Host) {
	if a.pingIP != h.IP || (a.pingTerm == nil && len(a.pingLines) == 0) {
		return
	}
	t := c.Theme()
	if a.pingTerm != nil {
		// The terminal view paints itself and has no height of its own:
		// without an explicit height it collapses to nothing.
		a.pingTerm.view(c).Height(140).Shrink(0)
	} else {
		ui.List(c, &a.pingList, len(a.pingLines), func(i int) {
			color := t.Text
			if a.pingLines[i] == "请求超时。" {
				color = t.TextMuted
			}
			ui.Text(c, a.pingLines[i]).Font("monospace").FontSize(12).TextColor(color)
		}).Background(ui.Hex("#f1f2f3")).Radius(6).Padding(8).Height(140).Shrink(0)
	}
	ui.Textf(c, "已发送 %d · 已接收 %d · 丢包率 %.0f%% · 平均 %.1fms",
		a.pingStats.sent, a.pingStats.recv, a.pingStats.lossPct(), a.pingStats.avgRTT().Seconds()*1000).
		FontSize(12).TextColor(t.TextMuted).Shrink(0)
}

// portSection shows the selected host's port scan: a guide line before
// the first scan, then the open ports, closed ones folded into a count.
func (a *app) portSection(c *ui.Context, h model.Host) {
	t := c.Theme()
	switch {
	case a.portIP == h.IP && a.portScanning:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在扫描端口…").FontSize(12).TextColor(t.TextMuted)
		})
	case a.portIP == h.IP && a.portErr != "":
		ui.Text(c, "端口扫描失败，请稍后再试").FontSize(12).TextColor(t.Danger).Shrink(0)
	case a.portIP == h.IP && len(a.portResults) > 0:
		results := sortedPorts(a.portResults)
		open, closed := 0, 0
		for _, p := range results {
			if p.Open {
				open++
			} else {
				closed++
			}
		}
		ui.Textf(c, "开放 %d · 关闭 %d", open, closed).FontSize(12).TextColor(t.TextMuted).Shrink(0)
		if open == 0 {
			ui.Text(c, "没发现开放的端口").FontSize(12).TextColor(t.TextMuted).Shrink(0)
			return
		}
		openResults := make([]model.Port, 0, open)
		for _, p := range results {
			if p.Open {
				openResults = append(openResults, p)
			}
		}
		height := float32(len(openResults))*28 + 8
		ui.Scroll(c).TrackScroll(&a.portScroll).Height(min(height, 180)).Shrink(0).Children(func() {
			for _, p := range openResults {
				a.portRow(c, h.IP, p, t)
			}
		})
	default:
		ui.Text(c, "点「端口扫描」，探测这台设备的常用端口").FontSize(12).TextColor(t.TextMuted).Shrink(0)
	}
}

// portRow is one aligned row: port number, the service in plain words,
// state, then the port's next steps (issue #18) — open or copy only.
func (a *app) portRow(c *ui.Context, ip netip.Addr, p model.Port, t *ui.Theme) {
	ui.Row(c).Height(28).AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Textf(c, "%d", p.Number).Font("monospace").Width(52).TextAlign(ui.End).
			FontSize(12).TextColor(t.Text)
		ui.Text(c, portLabel(p)).Width(76).SingleLine().FontSize(12).TextColor(t.Text)
		ui.Text(c, "开放").FontSize(12).FontWeight(600).TextColor(t.Success)
		ui.Box(c).Grow(1)
		for _, act := range portActions(ip, p) {
			btn := ui.Button(c, act.Label).FontSize(12)
			if btn.Clicked() {
				runPortAction(c, act)
			}
		}
	})
}

// sortedPorts orders open ports first, then by number.
func sortedPorts(ports []model.Port) []model.Port {
	out := append([]model.Port(nil), ports...)
	slices.SortStableFunc(out, func(x, y model.Port) int {
		if x.Open != y.Open {
			if x.Open {
				return -1
			}
			return 1
		}
		return x.Number - y.Number
	})
	return out
}

func pill(c *ui.Context, label string, bg, fg ui.Color) {
	ui.Box(c).Padding(1, 8).Radius(99).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(fg)
	})
}
