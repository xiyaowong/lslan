package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

var hostTableColumns = []ui.TableColumn{
	{Title: "状态", ID: "state", Width: 52, Sortable: true},
	{Title: "主机号", ID: "host", Width: 66, Align: ui.End, Sortable: true},
	{Title: "IP", ID: "ip", Width: 148, Sortable: true},
	{Title: "厂商", ID: "vendor", Width: 130, Sortable: true},
	{Title: "MAC", ID: "mac", Width: 158},
	{Title: "主机名", ID: "hostname", Sortable: true},
	{Title: "开放端口", ID: "ports", Width: 104},
}

// View rebuilds the whole interface each frame: title bar, toolbar, the
// table (split from the detail panel when a host is selected), status bar.
func (a *app) View(c *ui.Context) {
	t := appTheme()
	c.SetTheme(t)
	a.runPending()
	a.presets()
	ui.Column(c).Fill().Children(func() {
		a.titleBar(c)
		a.toolbar(c)
		a.body(c, t)
		a.statusBar(c)
	})
}

// body is the table pane, split from the detail pane once a host is
// selected; the divider between them drags.
func (a *app) body(c *ui.Context, t *ui.Theme) {
	tablePane := func() {
		ui.Column(c).Fill().Background(t.Surface).Children(func() {
			a.hostTable(c)
		})
	}
	if !a.selIP.IsValid() {
		tablePane()
		return
	}
	detailPane := func() {
		ui.Column(c).Fill().Background(t.Surface).Children(func() {
			a.detail(c)
		})
	}
	ui.SplitVertical(c, &a.detailHeight, tablePane, detailPane).Fill()
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "网卡").TextColor(t.TextMuted)
		switch {
		case len(a.nicOptions) > 0:
			sel := ui.Select(c, &a.nicSel, a.nicOptions).Width(190).Label("网卡")
			if sel.Changed() {
				if i := slices.Index(a.nicOptions, a.nicSel); i >= 0 {
					a.nicIdx = i
					a.fillRange()
				}
			}
		case a.nics == nil:
			ui.Text(c, "正在加载网卡…").TextColor(t.TextMuted)
		default:
			ui.Text(c, "未找到可用网卡").TextColor(t.Danger)
		}
		ui.Text(c, "范围").TextColor(t.TextMuted)
		ui.TextInput(c, &a.startIP).Label("开始 IP").Width(100)
		ui.Text(c, "–").TextColor(t.TextMuted)
		ui.TextInput(c, &a.endIP).Label("结束 IP").Width(100)

		ui.Box(c).Grow(1)

		hasSel := a.selIP.IsValid()
		copyBtn := ui.Button(c, "复制 IP").Disabled(!hasSel)
		if hasSel && copyBtn.Clicked() {
			c.WriteClipboard(a.selIP.String())
		}
		online := a.onlineCount()
		copyAll := ui.Button(c, "复制在线 IP").Disabled(online == 0)
		if online > 0 && copyAll.Clicked() {
			c.WriteClipboard(a.onlineIPs())
		}
		if a.scanning {
			if ui.Button(c, "取消扫描").Clicked() {
				a.scanner.Cancel()
			}
		} else if ui.PrimaryButton(c, "开始扫描").Clicked() {
			a.startScan()
		}
	})
}

// statusBar sits at the bottom: tallies, scan progress, errors, filters.
func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Divider(c).Shrink(0)
	ui.Row(c).Height(26).Padding(0, 12).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Textf(c, "在线 %d · 已发现 %d · 总数 %d", a.onlineCount(), len(a.hosts), a.progress.Total).
			FontSize(12).TextColor(t.TextMuted)
		if a.scanning {
			pct := 0.0
			if a.progress.Total > 0 {
				pct = float64(a.progress.Scanned) / float64(a.progress.Total)
			}
			ui.Progress(c, pct).Width(160)
			ui.Textf(c, "%d / %d", a.progress.Scanned, a.progress.Total).FontSize(12).TextColor(t.TextMuted)
		}
		if a.scanErr != "" {
			ui.Text(c, a.scanErr).FontSize(12).TextColor(t.Danger)
		}
		if start, end, ok := a.parseRange(); ok {
			if n, _ := rangeCount(start, end); n > 256 {
				ui.Textf(c, "范围共 %d 个地址，建议限制在一个 C 段以内", n).FontSize(12).TextColor(t.TextMuted)
			}
		}
		ui.Box(c).Grow(1)
		ui.Toggle(c, &a.showOffline, "显示离线").FontSize(12)
	})
}

func (a *app) hostTable(c *ui.Context) {
	visible := a.visibleHosts()
	a.syncSelection(visible)
	// Ping follows the selection.
	if a.pingCancel != nil && a.pingIP != a.selIP {
		a.stopPing()
	}
	ui.Table(c, &a.tableState, hostTableColumns, len(visible), func(row, col int) {
		a.hostCell(c, visible[row], col)
	}).Grow(1)
}

func (a *app) hostCell(c *ui.Context, h model.Host, col int) {
	t := c.Theme()
	switch col {
	case 0:
		dotColor := t.Border
		switch h.State {
		case model.StateOnline:
			dotColor = t.Success
		case model.StateOffline:
			dotColor = t.TextMuted
		}
		ui.Box(c).Size(9, 9).Radius(4.5).Background(dotColor).Shrink(0)
	case 1:
		ui.Textf(c, "%d", h.LastOctet())
	case 2:
		ui.Text(c, h.IP.String()).Font("monospace")
	case 3:
		ui.Text(c, orDash(h.Vendor)).SingleLine()
	case 4:
		ui.Text(c, orDash(h.MAC)).Font("monospace")
	case 5:
		ui.Text(c, orDash(h.Hostname)).SingleLine()
	case 6:
		ui.Text(c, portsSummary(h.Ports)).SingleLine().TextColor(t.TextMuted)
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
