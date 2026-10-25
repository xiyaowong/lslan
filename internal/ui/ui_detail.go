package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// detail fills the split's lower pane for the selected host.
func (a *app) detail(c *ui.Context) {
	h, ok := a.hosts[a.selIP]
	if !ok {
		return
	}
	ui.Column(c).Fill().Padding(10, 16).Gap(8).Children(func() {
		a.detailHeader(c, h)
		a.detailInfo(c, h)
		a.pingTerminal(c, h)
		a.portSection(c, h)
	})
}

func (a *app) detailHeader(c *ui.Context, h model.Host) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, h.IP.String()).FontSize(16).Bold().Font("monospace")
		statusPill(c, h.State)
		if h.Gateway {
			pill(c, "网关", t.Accent, t.AccentText)
		}
		ui.Box(c).Grow(1)

		if ui.Button(c, "浏览器打开").Clicked() {
			c.OpenURL("http://" + h.IP.String())
		}
		pinging := a.pingCancel != nil && a.pingIP == h.IP
		if pinging {
			if ui.Button(c, "停止 Ping").Clicked() {
				a.stopPing()
			}
		} else if ui.Button(c, "Ping").Clicked() {
			a.startPing(h.IP)
		}

		// The preset belongs to the port scan, not to the row's actions:
		// it hugs the scan button's left as one group.
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			if len(a.presetNames) > 0 {
				sel := ui.Select(c, &a.presetSel, a.presetNames).Width(104).Label("端口预设")
				if sel.Changed() {
					if i := slices.Index(a.presetNames, a.presetSel); i >= 0 {
						a.portPreset = i
					}
				}
			}
			scanBtn := ui.Button(c, "端口扫描").Disabled(a.portScanning)
			if !a.portScanning && scanBtn.Clicked() {
				a.startPortScan(h.IP)
			}
		})
	})
}

func (a *app) detailInfo(c *ui.Context, h model.Host) {
	t := c.Theme()
	ui.Textf(c, "主机名: %s · MAC: %s · 厂商: %s",
		orDash(h.Hostname), orDash(h.MAC), orDash(h.Vendor)).TextColor(t.TextMuted)
}

// pingTerminal streams the ping output of the selected host, in the
// terminal plugin when one was created, else a plain line list.
func (a *app) pingTerminal(c *ui.Context, h model.Host) {
	if a.pingIP != h.IP || (a.pingTerm == nil && len(a.pingLines) == 0) {
		return
	}
	t := c.Theme()
	if a.pingTerm != nil {
		a.pingTerm.view(c).MaxHeight(140).Shrink(0)
	} else {
		ui.List(c, &a.pingList, len(a.pingLines), func(i int) {
			color := t.Text
			if strings.HasPrefix(a.pingLines[i], "请求超时") {
				color = t.TextMuted
			}
			ui.Text(c, a.pingLines[i]).Font("monospace").FontSize(12).TextColor(color)
		}).Background(ui.Hex("#f1f2f3")).Radius(6).Padding(8).MaxHeight(140).Shrink(0)
	}
	ui.Textf(c, "已发送 %d · 已接收 %d · 丢包率 %.0f%% · 平均 %.1fms",
		a.pingStats.sent, a.pingStats.recv, a.pingStats.lossPct(), a.pingStats.avgRTT().Seconds()*1000).
		FontSize(12).TextColor(t.TextMuted).Shrink(0)
}

// portSection shows the selected host's port scan: a compact, aligned,
// scrollable list, open ports first.
func (a *app) portSection(c *ui.Context, h model.Host) {
	if a.portIP != h.IP {
		return
	}
	t := c.Theme()
	switch {
	case a.portScanning:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在扫描端口…").FontSize(12).TextColor(t.TextMuted)
		})
	case a.portErr != "":
		ui.Text(c, "端口扫描失败，请稍后再试").FontSize(12).TextColor(t.Danger)
	case len(a.portResults) > 0:
		results := sortedPorts(a.portResults)
		open := 0
		for _, p := range results {
			if p.Open {
				open++
			}
		}
		ui.Textf(c, "开放 %d · 共 %d", open, len(results)).FontSize(12).TextColor(t.TextMuted).Shrink(0)
		height := float32(len(results))*26 + 8
		ui.Scroll(c).TrackScroll(&a.portScroll).Height(min(height, 180)).Shrink(0).Children(func() {
			for _, p := range results {
				portRow(c, p, t)
			}
		})
	}
}

// portRow is one aligned row: port number, service, state. Closed ports
// recede into gray.
func portRow(c *ui.Context, p model.Port, t *ui.Theme) {
	numCol, stateCol, stateText := t.TextMuted, t.TextMuted, "已关闭"
	if p.Open {
		numCol, stateCol, stateText = t.Text, t.Success, "开放"
	}
	ui.Row(c).Height(26).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "%d", p.Number).Font("monospace").Width(60).TextAlign(ui.End).
			FontSize(12).TextColor(numCol)
		ui.Text(c, orDash(p.Service)).Width(110).SingleLine().FontSize(12).TextColor(numCol)
		ui.Text(c, stateText).FontSize(12).FontWeight(600).TextColor(stateCol)
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

func statusPill(c *ui.Context, s model.HostState) {
	t := c.Theme()
	switch s {
	case model.StateOnline:
		pill(c, "在线", t.Success, ui.Hex("#ffffff"))
	case model.StateOffline:
		pill(c, "离线", t.SurfaceHover, t.TextMuted)
	default:
		pill(c, "未知", t.SurfaceHover, t.TextMuted)
	}
}

func pill(c *ui.Context, label string, bg, fg ui.Color) {
	ui.Box(c).Padding(1, 8).Radius(99).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(fg)
	})
}
