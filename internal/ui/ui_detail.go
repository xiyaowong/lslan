package ui

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// detail fills the sidebar for the selected host: a header with the
// device and a way to close it, the actions, the facts, then the ports,
// the services it advertises and the ping output. It scrolls when the
// content needs more room than the sidebar has.
func (a *app) detail(c *ui.Context) {
	h, ok := a.hosts[a.selIP]
	if !ok {
		return
	}
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(16, 18, 20, 18).Gap(18).Children(func() {
			a.detailHeader(c, t, h)
			a.detailActions(c, h)
			a.detailInfo(c, t, h)
			a.portSection(c, t, h)
			a.serviceSection(c, t, h)
			a.pingTerminal(c, t, h)
		})
	})
}

// detailHeader names the device: its state, address, a button copying
// the address, the badges, and the button that puts the sidebar away.
func (a *app) detailHeader(c *ui.Context, t *ui.Theme, h model.Host) {
	ui.Column(c).Gap(8).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(10, 10).Radius(5).Background(stateDot(t, h)).Shrink(0)
			ui.Text(c, h.IP.String()).FontSize(20).FontWeight(600).FontFeatures("tnum")
			ui.Box(c).Grow(1)
			if iconButton(c, "关闭详情", drawClose).Clicked() {
				a.selRow, a.selIP = -1, netip.Addr{}
			}
		})
		if h.Gateway || h.ViaMDNS {
			ui.Row(c).Gap(6).Children(func() {
				if h.Gateway {
					bg, fg := gatewayPill(t)
					pill(c, "网关", bg, fg)
				}
				if h.ViaMDNS {
					bg, fg := softPill(t)
					pill(c, "服务发现", bg, fg)
				}
			})
		}
	})
}

// detailInfo is the facts, a caption and its value a row, each with a
// button copying the value for pasting elsewhere.
func (a *app) detailInfo(c *ui.Context, t *ui.Theme, h model.Host) {
	scanned := a.ports[h.IP]
	open := len(openPorts(scanned))
	ports := ""
	if len(scanned) > 0 {
		ports = fmt.Sprintf("%d 个开放 · 共 %d 个", open, len(scanned))
	}
	seen := ""
	if !h.LastSeen.IsZero() {
		seen = h.LastSeen.Format("01-02 15:04")
	}
	type fact struct{ label, value string }
	for _, f := range [5]fact{
		{"主机名", h.Hostname},
		{"厂商", h.Vendor},
		{"MAC 地址", h.MAC},
		{"开放端口", ports},
		{"上次在线", seen},
	} {
		value := f.value
		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, f.label).FontSize(12).Width(64).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, orDash(value)).FontSize(13).SingleLine().Tooltip(value)
			if value != "" {
				if iconButton(c, "复制"+f.label, drawCopy).Clicked() {
					c.WriteClipboard(value)
				}
			}
		})
	}
}

// detailActions are what the host offers: reach it, probe its ports, or
// reach it in the browser; the presets beyond the common one wait behind
// 更多.
func (a *app) detailActions(c *ui.Context, h model.Host) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		pinging := a.pingCancel != nil && a.pingIP == h.IP
		label := "Ping"
		if pinging {
			label = "停止 Ping"
		}
		if smallPrimary(c, label).Clicked() {
			if pinging {
				a.stopPing()
			} else {
				a.startPing(h.IP)
			}
		}
		if btn := smallButton(c, "端口扫描").Disabled(a.portScanning); !a.portScanning && btn.Clicked() {
			a.startPortScan(h.IP)
		}
		if smallButton(c, "浏览器打开").Clicked() {
			c.OpenURL("http://" + h.IP.String())
		}
		more := flatButton(c, "更多")
		if more.Clicked() {
			a.moreOpen = !a.moreOpen
		}
		ui.Popover(c, more, &a.moreOpen, func() {
			for i, p := range a.presets() {
				if i == 0 {
					continue
				}
				if flatButton(c, p.Name+"端口").Clicked() {
					a.moreOpen = false
					a.startPortScanPreset(h.IP, i)
				}
			}
		})
	})
}

// pingTerminal streams the ping output of the selected host — the
// terminal plugin view when one was created, else a plain line list —
// and only while pinging or with lines to show.
func (a *app) pingTerminal(c *ui.Context, t *ui.Theme, h model.Host) {
	if a.pingIP != h.IP || (a.pingTerm == nil && len(a.pingLines) == 0) {
		return
	}
	ui.Column(c).Gap(8).Shrink(0).Children(func() {
		sectionTitle(c, "Ping 结果")
		if a.pingTerm != nil {
			// The terminal view paints itself and has no height of its own:
			// without an explicit height it collapses to nothing.
			ui.Box(c).Background(t.Background).Radius(6).Clip().Children(func() {
				a.pingTerm.view(c).Height(140)
			})
		} else {
			ui.List(c, &a.pingList, len(a.pingLines), func(i int) {
				color := t.Text
				if a.pingLines[i] == "请求超时。" {
					color = t.TextMuted
				}
				ui.Text(c, a.pingLines[i]).Font("monospace").FontSize(12).TextColor(color)
			}).Background(t.Background).Radius(6).Padding(10).Height(140)
		}
		ui.Textf(c, "已发送 %d · 已接收 %d · 丢包率 %.0f%% · 平均 %.1fms",
			a.pingStats.sent, a.pingStats.recv, a.pingStats.lossPct(), a.pingStats.avgRTT().Seconds()*1000).
			FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
	})
}

// sectionTitle names a group of results.
func sectionTitle(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(13).FontWeight(600).Shrink(0)
}

// section is a group of results: a title line with a note, and the rows
// boxed under it.
func section(c *ui.Context, t *ui.Theme, title, note string, rows func()) {
	ui.Column(c).Gap(8).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, title)
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
		})
		ui.Column(c).Radius(6).Border(1, t.Border).Clip().Children(rows)
	})
}

// portSection shows the selected host's port scan: a guide line before
// the first scan, a spinner while it runs, then the open ports. The
// results come from the cache, so a sweep's probes and a manual scan show
// the same list, and they stay as the user moves between hosts.
func (a *app) portSection(c *ui.Context, t *ui.Theme, h model.Host) {
	scanned, done := a.ports[h.IP]
	switch {
	case a.portProbing(h.IP):
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在扫描端口…").FontSize(12).TextColor(t.TextMuted)
		})
	case a.portIP == h.IP && a.portErr != "":
		ui.Text(c, "端口扫描没有完成，请稍后再试").FontSize(12).TextColor(t.Danger).Shrink(0)
	case done:
		openCount := 0
		var open []model.Port
		for _, p := range sortedPorts(scanned) {
			if p.Open {
				openCount++
				open = append(open, p)
			}
		}
		note := fmt.Sprintf("开放 %d 个", openCount)
		if openCount == 0 {
			note = "没有开放"
		}
		section(c, t, "端口", note, func() {
			if openCount == 0 {
				ui.Row(c).Padding(10, 12).Children(func() {
					ui.Text(c, "所有常用端口都关着").FontSize(12).TextColor(t.TextMuted)
				})
				return
			}
			ui.Scroll(c).TrackScroll(&a.portScroll).Height(float32(min(openCount, 5)) * 34).
				Children(func() {
					for i, p := range open {
						if i > 0 {
							ui.Divider(c).Shrink(0)
						}
						a.portRow(c, t, h.IP, p)
					}
				})
		})
	default:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			sectionTitle(c, "端口")
			ui.Text(c, "还没扫描过").FontSize(12).TextColor(t.TextMuted)
		})
	}
}

// portProbing reports whether the host's ports are being probed now, by
// the sweep or by a scan the user asked for.
func (a *app) portProbing(ip netip.Addr) bool {
	if a.portScanning && a.portIP == ip {
		return true
	}
	return a.sweepPending[ip]
}

// portRow is one port: the number, the service in plain words, its next
// step (issue #18) as a text button, and a button copying the port.
func (a *app) portRow(c *ui.Context, t *ui.Theme, ip netip.Addr, p model.Port) {
	acts := portActions(ip, p)
	ui.Row(c).Height(34).Padding(0, 8, 0, 12).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Textf(c, "%d", p.Number).FontSize(13).FontWeight(600).FontFeatures("tnum").Width(42).
			TextAlign(ui.End)
		ui.Text(c, portLabel(p)).FontSize(13).SingleLine().TextColor(t.TextMuted)
		ui.Box(c).Grow(1)
		// The copy of the port itself ends every list of actions.
		for _, act := range acts[:len(acts)-1] {
			if flatButton(c, act.Label).Clicked() {
				runPortAction(c, act)
			}
		}
		if iconButton(c, "复制端口", drawCopy).Clicked() {
			c.WriteClipboard(acts[len(acts)-1].Arg)
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
	ui.Box(c).Padding(2, 8).Radius(99).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(fg)
	})
}
