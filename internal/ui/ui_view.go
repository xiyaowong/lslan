package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

var hostTableColumns = []ui.TableColumn{
	{Title: "状态", ID: "state", Width: 46, Sortable: true},
	{Title: "主机号", ID: "host", Width: 56, Align: ui.End, Sortable: true},
	{Title: "IP", ID: "ip", Width: 118, Sortable: true},
	{Title: "厂商", ID: "vendor", Width: 130, Sortable: true},
	{Title: "MAC", ID: "mac", Width: 138},
	{Title: "主机名", ID: "hostname", Sortable: true},
	{Title: "开放端口", ID: "ports", Width: 110},
}

// View rebuilds the whole interface each frame: toolbar, status line,
// host table, detail panel.
func (a *app) View(c *ui.Context) {
	a.runPending()
	a.presets()
	ui.Column(c).Fill().Children(func() {
		a.toolbar(c)
		a.statusLine(c)
		a.hostTable(c)
		a.detail(c)
	})
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(10, 12).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "网卡").TextColor(t.TextMuted)
		switch {
		case len(a.nicOptions) > 0:
			sel := ui.Select(c, &a.nicSel, a.nicOptions).Width(220).Label("网卡")
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
		ui.Text(c, "开始").TextColor(t.TextMuted)
		ui.TextInput(c, &a.startIP).Label("开始 IP").Width(116)
		ui.Text(c, "结束").TextColor(t.TextMuted)
		ui.TextInput(c, &a.endIP).Label("结束 IP").Width(116)

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

func (a *app) statusLine(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(0, 12).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Textf(c, "在线 %d · 已发现 %d · 总数 %d", a.onlineCount(), len(a.hosts), a.progress.Total).
			TextColor(t.TextMuted)
		if a.scanning {
			pct := 0.0
			if a.progress.Total > 0 {
				pct = float64(a.progress.Scanned) / float64(a.progress.Total)
			}
			ui.Progress(c, pct).Width(160)
			ui.Textf(c, "%d / %d", a.progress.Scanned, a.progress.Total).TextColor(t.TextMuted)
		}
		if a.scanErr != "" {
			ui.Text(c, a.scanErr).TextColor(t.Danger)
		}
		if ui.Toggle(c, &a.showOffline, "显示离线").Changed() {
			a.syncSelection(nil)
		}
		if start, end, ok := a.parseRange(); ok {
			if n, _ := rangeCount(start, end); n > 256 {
				ui.Textf(c, "范围共 %d 个地址，建议限制在一个 C 段以内", n).TextColor(t.TextMuted)
			}
		}
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
	switch col {
	case 0:
		dotColor := c.Theme().Border
		switch h.State {
		case model.StateOnline:
			dotColor = c.Theme().Success
		case model.StateOffline:
			dotColor = c.Theme().TextMuted
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
		ui.Text(c, portsSummary(h.Ports)).SingleLine().TextColor(c.Theme().TextMuted)
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
