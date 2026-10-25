package ui

import (
	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// detail is the panel under the table for the selected host.
func (a *app) detail(c *ui.Context) {
	if !a.selIP.IsValid() {
		return
	}
	h, ok := a.hosts[a.selIP]
	if !ok {
		return
	}
	ui.Divider(c)
	ui.Column(c).Padding(10, 12).Gap(8).Shrink(0).Children(func() {
		a.detailHeader(c, h)
		a.detailInfo(c, h)
		a.pingTerminal(c, h)
		a.portList(c, h)
	})
}

func (a *app) detailHeader(c *ui.Context, h model.Host) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, h.IP.String()).FontSize(17).Bold().Font("monospace")
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

		if len(a.presetNames) > 0 {
			sel := ui.Select(c, &a.presetSel, a.presetNames).Width(100)
			if sel.Changed() {
				for i, name := range a.presetNames {
					if name == a.presetSel {
						a.portPreset = i
					}
				}
			}
		}
		scanBtn := ui.Button(c, "端口扫描").Disabled(a.portScanning)
		if !a.portScanning && scanBtn.Clicked() {
			a.startPortScan(h.IP)
		}
	})
}

func (a *app) detailInfo(c *ui.Context, h model.Host) {
	t := c.Theme()
	ui.Textf(c, "主机名: %s · MAC: %s · 厂商: %s",
		orDash(h.Hostname), orDash(h.MAC), orDash(h.Vendor)).TextColor(t.TextMuted).SingleLine()
}

// pingTerminal streams the ping output of the selected host.
func (a *app) pingTerminal(c *ui.Context, h model.Host) {
	if a.pingIP != h.IP || (len(a.pingLines) == 0 && a.pingCancel == nil) {
		return
	}
	t := c.Theme()
	ui.Textf(c, "已发送 %d · 已接收 %d · 丢包率 %.0f%% · 平均 %.1fms",
		a.pingStats.sent, a.pingStats.recv, a.pingStats.lossPct(), a.pingStats.avgRTT().Seconds()*1000).
		TextColor(t.TextMuted)
	ui.List(c, &a.pingList, len(a.pingLines), func(i int) {
		ui.Text(c, a.pingLines[i]).Font("monospace").FontSize(12).TextColor(ui.Hex("#d4d4d8"))
	}).Background(ui.Hex("#18181b")).Radius(6).Padding(8).MaxHeight(150).Shrink(0)
}

// portList shows the selected host's port scan results.
func (a *app) portList(c *ui.Context, h model.Host) {
	if a.portIP != h.IP {
		return
	}
	t := c.Theme()
	switch {
	case a.portScanning:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在扫描端口…").TextColor(t.TextMuted)
		})
	case a.portErr != "":
		ui.Text(c, "端口扫描失败: "+a.portErr).TextColor(t.Danger)
	case len(a.portResults) > 0:
		ui.Column(c).Gap(2).MaxHeight(150).Clip().Children(func() {
			for _, p := range a.portResults {
				state, color := "关闭", t.TextMuted
				if p.Open {
					state, color = "开放", t.Success
				}
				ui.Row(c).Gap(12).Children(func() {
					ui.Textf(c, "%d", p.Number).Font("monospace").Width(56).TextAlign(ui.End)
					ui.Text(c, orDash(p.Service)).Width(120)
					ui.Text(c, state).TextColor(color)
				})
			}
		})
	}
}

func statusPill(c *ui.Context, s model.HostState) {
	t := c.Theme()
	switch s {
	case model.StateOnline:
		pill(c, "在线", t.Success, ui.Hex("#ffffff"))
	case model.StateOffline:
		pill(c, "离线", t.Border, t.TextMuted)
	default:
		pill(c, "未知", t.Surface, t.TextMuted)
	}
}

func pill(c *ui.Context, label string, bg, fg ui.Color) {
	ui.Box(c).Padding(1, 8).Radius(99).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(fg)
	})
}
