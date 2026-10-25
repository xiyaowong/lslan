package ui

import (
	"slices"
	"strconv"
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
	a.runNextFrame(c)
	ui.Column(c).Fill().Children(func() {
		a.titleBar(c)
		a.toolbar(c)
		a.body(c, t)
		a.statusBar(c)
	})
}

// body is the table pane, split from the detail pane once a host is
// selected; the divider between them drags, and the table keeps at least
// half the room.
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
	// The table keeps 35–80% of the split, so neither pane squeezes the
	// other out (issue #21); the split itself enforces a minimum pane.
	a.tableShare = min(max(a.tableShare, 35), 80)
	ui.SplitVerticalPercent(c, &a.tableShare, tablePane, detailPane).Fill()
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

		ui.Box(c).Grow(1)

		if a.scanning {
			if ui.Button(c, "取消扫描").Clicked() {
				a.scanner.Cancel()
			}
		} else if ui.PrimaryButton(c, "开始扫描").Clicked() {
			a.startScan()
		}
	})
	a.rangeBar(c)
}

// rangeBar is the toolbar's second line: the sweep range as a segmented
// IP control, eight octet cells with fixed separators. The three network
// cells lock to the NIC's prefix, so a wrong network cannot be typed;
// "整个网段" restores the NIC's default.
func (a *app) rangeBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(0, 12).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "范围").TextColor(t.TextMuted)
		a.ipField(c, t)
		if ui.Button(c, "整个网段").Clicked() {
			a.fillRange()
		}
		if n, ok := a.rangeTotal(); ok && n > 256 {
			if ui.Button(c, "收窄到 C 段").Clicked() {
				a.narrowRange()
			}
			ui.Textf(c, "范围共 %d 个地址，扫描会很慢", n).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

// ipField is the segmented control: one bordered row holding eight octet
// cells, `.` between the cells of an address and `–` between addresses.
func (a *app) ipField(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(t.Space(0.5)).AlignItems(ui.Center).Padding(t.Space(1), t.Space(1.5)).
		Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Shrink(0).Children(func() {
		for i := range 8 {
			if i == 4 {
				ui.Text(c, "–").TextColor(t.TextMuted).Font("monospace")
			}
			if i%4 != 0 {
				ui.Text(c, ".").TextColor(t.TextMuted).Font("monospace")
			}
			a.ipCell(c, t, i)
		}
	})
}

// ipCell builds one octet cell: monospaced digits, right-aligned. A
// locked cell shows a network octet of the NIC, disabled and grayed. In
// an editable cell the third digit moves to the next one, and Backspace
// on an empty one goes back a cell; the moves land through a.focusCell,
// whose wish ipCell picks up as it builds.
func (a *app) ipCell(c *ui.Context, t *ui.Theme, i int) {
	w := t.Space(8)
	if a.cellLocked(i) {
		ui.TextInputBase(c, &a.ipCells[i]).Width(w).Font("monospace").TextAlign(ui.End).
			Disabled(true).TextColor(t.TextMuted).Label("IP 第 " + strconv.Itoa(i+1) + " 段（网段，自动）")
		return
	}
	cell := ui.TextInputBase(c, &a.ipCells[i]).Width(w).Font("monospace").TextAlign(ui.End).
		Padding(t.Space(0.8), 0).Label("IP 第 " + strconv.Itoa(i+1) + " 段")
	cell.OnTextKey(func(k ui.TextKey) bool {
		if k.Key == ui.KeyBackspace && k.Mods == 0 && k.Text() == "" {
			// Empty, the key would do nothing; take it as a step back.
			if p := a.prevEditable(i); p >= 0 {
				a.focusCell(p)
			}
		}
		return false
	})
	if a.lastCell == i {
		// A jump asked for this cell: focus it, caret at the end, and
		// drop the wish. Focus holds as the view calls it every frame.
		a.lastCell = -1
		cell.Focus().SetTextSelection(len(a.ipCells[i]), len(a.ipCells[i]))
	} else if cell.Changed() {
		a.ipCells[i] = clampOctet(a.ipCells[i])
		if len(a.ipCells[i]) >= 3 {
			if n := a.nextEditable(i); n >= 0 {
				a.focusCell(n)
			}
		}
	}
}

// focusCell asks for the focus to land on cell i as the frame builds;
// ipCell picks the wish up.
func (a *app) focusCell(i int) { a.lastCell = i }

// nextEditable is the next cell past i the user may edit, -1 at the end.
func (a *app) nextEditable(i int) int {
	for j := i + 1; j < 8; j++ {
		if !a.cellLocked(j) {
			return j
		}
	}
	return -1
}

// prevEditable is the last cell before i the user may edit, -1 at the
// start.
func (a *app) prevEditable(i int) int {
	for j := i - 1; j >= 0; j-- {
		if !a.cellLocked(j) {
			return j
		}
	}
	return -1
}

// statusBar sits at the bottom, outside the table/detail split, so no
// pane drag can push it out of the window: tallies as segments, the scan
// progress as a thin bar with its percent, errors, the offline filter.
func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Divider(c).Shrink(0)
	ui.Row(c).Height(34).Padding(0, 12).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.stat(c, "在线", a.onlineCount(), true)
		a.vsep(c, t)
		a.stat(c, "已发现", a.discoveredCount(), false)
		a.vsep(c, t)
		a.stat(c, "总数", a.progress.Total, false)
		if a.scanning {
			a.vsep(c, t)
			pct := 0.0
			if a.progress.Total > 0 {
				pct = float64(a.progress.Scanned) / float64(a.progress.Total)
			}
			ui.Progress(c, pct).Width(180).Height(4).Shrink(0)
			ui.Textf(c, "%.0f%%", pct*100).Font("monospace").FontSize(12).FontWeight(600)
		}
		if a.scanErr != "" {
			a.vsep(c, t)
			ui.Text(c, a.scanErr).FontSize(12).TextColor(t.Danger)
		}
		ui.Box(c).Grow(1)
		ui.Toggle(c, &a.showOffline, "显示离线").FontSize(12)
	})
}

// stat is one tally segment: an optional dot, a muted label, then the
// number, bold and equal-width so digits do not jitter.
func (a *app) stat(c *ui.Context, label string, n int, dot bool) {
	t := c.Theme()
	ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		if dot {
			ui.Box(c).Size(8, 8).Radius(4).Background(t.Success).Shrink(0)
		}
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		ui.Textf(c, "%d", n).Font("monospace").FontSize(12).FontWeight(600)
	})
}

// vsep is the thin vertical line between the status bar's segments.
func (a *app) vsep(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Size(1, 14).Background(t.Border).Shrink(0)
}

func (a *app) hostTable(c *ui.Context) {
	visible := a.visibleHosts()
	a.syncSelection(visible)
	a.menuHosts = visible
	// Ping follows the selection.
	if a.pingCancel != nil && a.pingIP != a.selIP {
		a.stopPing()
	}
	a.tableState.CellMenu = a.rowMenu
	ui.Table(c, &a.tableState, hostTableColumns, len(visible), func(row, col int) {
		a.hostCell(c, visible[row], col)
	}).Grow(1)

}

func (a *app) hostCell(c *ui.Context, h model.Host, col int) {
	t := c.Theme()
	offline := h.State == model.StateOffline
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
		ui.Textf(c, "%d", h.LastOctet()).TextColor(cellText(t, offline))
	case 2:
		ui.Text(c, h.IP.String()).Font("monospace").TextColor(cellText(t, offline))
	case 3:
		ui.Text(c, orDash(h.Vendor)).SingleLine().TextColor(cellText(t, offline))
	case 4:
		ui.Text(c, orDash(h.MAC)).Font("monospace").TextColor(cellText(t, offline))
	case 5:
		ui.Text(c, orDash(h.Hostname)).SingleLine().TextColor(cellText(t, offline))
	case 6:
		ui.Text(c, portsSummary(h.Ports)).SingleLine().TextColor(t.TextMuted)
	}
}

// cellText grays an offline row's text down.
func cellText(t *ui.Theme, offline bool) ui.Color {
	if offline {
		return t.TextMuted
	}
	return t.Text
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
