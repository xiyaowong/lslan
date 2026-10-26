package ui

import (
	"fmt"
	"net/netip"
	"os/exec"
	"slices"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// detail fills the sidebar for the selected host: a header with the
// device and a way to close it, the actions, the facts, then the ports
// and the services it advertises — all scrolling when they need more
// room — and the ping output pinned to the panel's foot, so it stays in
// sight however long the scrolling part runs (#51).
func (a *app) detail(c *ui.Context) {
	h, ok := a.hosts[a.selIP]
	if !ok {
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Column(c).Padding(16, 18, 16, 18).Gap(18).Children(func() {
				a.detailHeader(c, t, h)
				a.detailActions(c, h)
				a.detailInfo(c, t, h)
				a.portSection(c, t, h)
				a.serviceSection(c, t, h)
			})
		})
		if a.pingVisible(h.IP) {
			ui.Divider(c).Shrink(0)
			a.pingPane(c, t)
		}
		if a.telnetVisible(h.IP) {
			ui.Divider(c).Shrink(0)
			a.telnetPane(c, t)
		}
		if a.traceVisible(h.IP) {
			ui.Divider(c).Shrink(0)
			a.tracePane(c, t)
		}
	})
}

// pingVisible reports whether the ping pane shows for ip: the session
// belongs to it and has anything to show — a live terminal, lines of a
// fallback list, or the tally of a session that has stopped.
func (a *app) pingVisible(ip netip.Addr) bool {
	return a.pingIP == ip && (a.pingTerm != nil || len(a.pingLines) > 0 || a.pingStats.sent > 0)
}

// telnetVisible reports whether the telnet pane shows for ip: the test
// belongs to it and is dialing, live, or saying why it failed.
func (a *app) telnetVisible(ip netip.Addr) bool {
	return a.telnetIP == ip && (a.telnetPending || a.telnetTerm != nil || a.telnetErr != "")
}

// detailHeader names the device: its state, address, a button copying
// the address, the badges, and the button that puts the sidebar away.
func (a *app) detailHeader(c *ui.Context, t *ui.Theme, h model.Host) {
	ui.Column(c).Gap(8).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(10, 10).Radius(5).Background(stateDot(t, h)).Shrink(0)
			ui.Text(c, h.IP.String()).FontSize(20).FontWeight(600).FontFeatures("tnum")
			ui.Box(c).Grow(1)
			if iconButton(c, "关闭详情", "关闭", 18, drawClose).Clicked() {
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
// button copying the value for pasting elsewhere. The ports live in the
// port section below, which says how many are open — no separate row.
func (a *app) detailInfo(c *ui.Context, t *ui.Theme, h model.Host) {
	seen := ""
	if !h.LastSeen.IsZero() {
		seen = h.LastSeen.Format("01-02 15:04")
	}
	type fact struct{ label, value string }
	for _, f := range [4]fact{
		{"主机名", h.Hostname},
		{"厂商", h.Vendor},
		{"MAC 地址", h.MAC},
		{"上次在线", seen},
	} {
		value := f.value
		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, f.label).FontSize(12).Width(64).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, orDash(value)).FontSize(13).SingleLine().Tooltip(value)
			if value != "" {
				if iconButton(c, "复制"+f.label, "复制", 18, drawCopy).Clicked() {
					c.WriteClipboard(value)
				}
			}
		})
	}
}

// detailActions are what the host offers: reach it, probe its ports, or
// reach it in the browser — two rows, so the words stay whole at the
// least window size; the presets beyond the common one wait behind 更多.
func (a *app) detailActions(c *ui.Context, h model.Host) {
	ui.Column(c).Gap(6).Shrink(0).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
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
			tracing := a.traceCancel != nil && a.traceIP == h.IP
			traceLabel := "Tracert"
			if tracing {
				traceLabel = "停止 Tracert"
			}
			if smallButton(c, traceLabel).Clicked() {
				if tracing {
					a.stopTrace()
				} else {
					a.traceRoute(h.IP)
				}
			}
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if smallButton(c, "浏览共享").Clicked() {
				_ = exec.Command("explorer.exe", `\\`+h.IP.String()).Start()
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
	})
}

// pingPane is the ping output of the selected host docked to the panel's
// foot — the terminal plugin view when one was created, else a plain
// line list. The caller checks there is anything to show. While the
// session runs the pane is the only output there is, so the ✕ that
// clears it appears once the session has stopped (#64).
func (a *app) pingPane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(8).Padding(10, 18, 12, 18).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "Ping 结果")
			ui.Box(c).Grow(1)
			if a.pingCancel == nil {
				if iconButton(c, "关闭 Ping 结果", "关闭", 16, drawClose).Clicked() {
					a.closePing()
				}
			}
		})
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

// telnetPane is the one-click telnet test docked to the panel's foot
// (#54): a spinner while dialing, the reason when it failed, else the
// live session — the device's output streaming in, the user's keys
// going out — with a way to hang up.
func (a *app) telnetPane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(8).Padding(10, 18, 12, 18).Shrink(0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "远程终端测试")
			ui.Textf(c, "%s:%d", a.telnetIP, a.telnetPort).
				FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if flatButton(c, "断开").Clicked() {
				a.stopTelnet()
			}
		})
		switch {
		case a.telnetPending:
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在连接…").FontSize(12).TextColor(t.TextMuted)
			})
		case a.telnetTerm == nil:
			ui.Text(c, a.telnetErr).FontSize(12).TextColor(t.Danger)
		default:
			// The terminal view paints itself and has no height of its own:
			// without an explicit height it collapses to nothing.
			ui.Box(c).Background(t.Background).Radius(6).Clip().Children(func() {
				a.telnetTerm.view(c).Height(180)
			})
			ui.Text(c, "在窗口里输入命令，回车发送").FontSize(12).TextColor(t.TextMuted)
		}
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
				if act.Kind == portTelnet {
					a.startTelnet(ip, p.Number) // the in-app test (#54)
				} else {
					runPortAction(c, act)
				}
			}
		}
		if iconButton(c, "复制端口", "复制", 18, drawCopy).Clicked() {
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
