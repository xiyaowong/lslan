package ui

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// treeKey is one row of the host tree: a host, and the port of one of its
// branches. Port 0 is the host's own row.
type treeKey struct {
	IP   netip.Addr
	Port int
}

// hostTreeColumns is the set the card shows while it has the room; the
// narrower card keeps only what identifies a device. Every column is as
// wide as what it holds, and the last takes the room left and shows
// nothing, so a wide window leaves the table at its own width. The IP
// column also carries the 网关 and 服务发现 badges, which is why it is
// wider than an address alone.
var (
	hostTreeColumns = []ui.TableColumn{
		{Title: "IP 地址", ID: "ip", Width: 232, MinWidth: 212, Sortable: true},
		{Title: "主机名", ID: "hostname", Width: 170, MinWidth: 116, Sortable: true},
		{Title: "厂商", ID: "vendor", Width: 140, MinWidth: 110, Sortable: true},
		{Title: "MAC 地址", ID: "mac", Width: 140, MinWidth: 136},
		{Title: "状态", ID: "state", Width: 56, Sortable: true},
		{Title: "", ID: "space", MinWidth: 0},
	}
	hostTreeColumnsNarrow = []ui.TableColumn{
		{Title: "IP 地址", ID: "ip", Width: 232, MinWidth: 212, Sortable: true},
		{Title: "主机名", ID: "hostname", Width: 160, MinWidth: 100, Sortable: true},
		{Title: "状态", ID: "state", Width: 56, Sortable: true},
		{Title: "", ID: "space", MinWidth: 0},
	}
)

// View rebuilds the whole interface each frame: title bar, the content
// card, the status bar.
func (a *app) View(c *ui.Context) {
	t := appTheme(a.dark)
	c.SetTheme(t)
	a.runPending()
	a.presets()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.titleBar(c, t)
		a.body(c, t)
		a.statusBar(c, t)
	})
}

// The host card only needs the width of the columns that identify a
// device; the rest of the window is better spent on the details beside it.
const (
	cardWidthDefault = 470
	cardWidthMin     = 400
	cardWidthMax     = 760
)

// body lays the workspace out as a modern app window: the host card takes
// a width of its own, the selected device's details take the rest, and
// the line between them drags.
func (a *app) body(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Grow(1).Shrink(1).Padding(gutter, gutter, gutter, gutter).Children(func() {
		if !a.selIP.IsValid() {
			a.workspaceCard(c, t).Fill()
			return
		}
		w, _ := c.Size()
		// The card keeps its width, but leaves the panel its least and the
		// card its own, so a small window squeezes the card first. Both
		// gutters and the split's own 1px divider come off the room.
		a.cardWidth = min(max(a.cardWidth, cardWidthMin), cardWidthMax,
			max(w-2*gutter-1-detailMinWidth, cardWidthMin))
		ui.Split(c, &a.cardWidth, func() {
			a.workspaceCard(c, t).Fill()
		}, func() {
			a.detailPanel(c, t).Fill()
		}).Fill()
	})
}

// detailMinWidth is the least room the detail panel takes well: its facts
// and the actions beside them.
const detailMinWidth = 380

// gutter is the window's side and top margin: the title bar, the cards and
// the status bar all line up on it.
const gutter = 16

// rangeLimit is the largest sweep offered: every address the sweep did not
// find is reported as offline, so a /8 would allocate gigabytes and feed
// the table sixteen million rows.
const rangeLimit = 65536

// cardWidth is the host card's width this frame: its own beside the
// detail panel, the window less its margins otherwise.
func cardWidth(c *ui.Context, a *app) float32 {
	if a.selIP.IsValid() {
		return a.cardWidth
	}
	w, _ := c.Size()
	return w - 32
}

func (a *app) workspaceCard(c *ui.Context, t *ui.Theme) ui.Element {
	return ui.Column(c).Background(t.Surface).Border(1, cardBorder(t)).
		Radius(8).Shadow(0, 1, 4, 0, cardShadow(t)).Clip().Children(func() {
		a.commandBar(c, t)
		ui.Divider(c).Shrink(0)
		a.tablePane(c, t)
	})
}

func (a *app) detailPanel(c *ui.Context, t *ui.Theme) ui.Element {
	return ui.Column(c).Background(t.Surface).Border(1, cardBorder(t)).
		Radius(8).Shadow(0, 1, 4, 0, cardShadow(t)).Clip().Children(func() {
		a.detail(c)
	})
}

func (a *app) commandBar(c *ui.Context, t *ui.Theme) {
	// A sweep runs against the range and adapter it started with; letting
	// them change mid-sweep would leave the screen showing one range and
	// the progress another.
	locked := a.scanning
	ui.Column(c).Shrink(0).Padding(14, 16, 12, 16).Gap(10).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			fieldLabel(c, t, "网卡")
			switch {
			case len(a.nicOptions) > 0:
				sel := ui.Select(c, &a.nicSel, a.nicOptions).
					Grow(1).MinWidth(160).MaxWidth(320).Shrink(1).Disabled(locked)
				if sel.Changed() && !locked {
					if i := slices.Index(a.nicOptions, a.nicSel); i >= 0 {
						a.nicIdx = i
						a.fillRange()
					}
				}
			case a.nics == nil:
				ui.Text(c, "正在加载网卡…").FontSize(13).TextColor(t.TextMuted)
			case a.nicErr != "":
				ui.Text(c, "没读到网卡").FontSize(13).TextColor(t.Danger)
				if flatButton(c, "重试").Clicked() {
					a.nicErr = ""
					a.nics = nil // show the load again while it retries
					go a.loadNICs()
				}
			default:
				ui.Text(c, "未找到可用网卡").FontSize(13).TextColor(t.Danger)
			}
			ui.Box(c).Grow(1)
			switch {
			case a.scanStopping:
				ui.Button(c, "正在停止…").Disabled(true)
			case a.scanning:
				if ui.Button(c, "取消扫描").Clicked() {
					a.cancelScan()
				}
			default:
				if btn := ui.PrimaryButton(c, "开始扫描").Disabled(!a.rangeFits()); btn.Clicked() {
					a.startScan()
				}
			}
		})
		// The sweep range and its options share the second line: on the
		// first, the adapter would have to give up too much room.
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			fieldLabel(c, t, "范围")
			a.ipField(c, t)
			if ui.Button(c, "整个网段").Disabled(locked).Clicked() && !locked {
				a.fillRange()
			}
			ui.Box(c).Grow(1)
			// Both options are about the sweep, so they sit together.
			ui.Checkbox(c, &a.withPorts, "顺带扫描端口").FontSize(12).
				Tooltip("扫描时一并查看在线地址开放的端口")
			ui.Checkbox(c, &a.showOffline, "显示离线地址").FontSize(12).
				Tooltip("把这次没扫到响应的地址也列出来")
		})
		a.rangeNote(c, t)
	})
}

// fieldLabel captions a control, in one width for both rows so the
// controls below each other line up.
func fieldLabel(c *ui.Context, t *ui.Theme, s string) {
	ui.Text(c, s).FontSize(12).Width(36).TextColor(t.TextMuted).Shrink(0)
}

// rangeNote is the note under the range: the narrowing offer for a wide
// span, and the limit past which a sweep is refused.
func (a *app) rangeNote(c *ui.Context, t *ui.Theme) {
	n, ok := a.rangeTotal()
	if !ok {
		return
	}
	switch {
	case n > rangeLimit:
		ui.Textf(c, "一次最多扫 %d 个地址，请先收窄范围", rangeLimit).
			FontSize(12).TextColor(t.Danger).SingleLine()
	case n > 256:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "收窄到本网段").Disabled(a.scanning).Clicked() && !a.scanning {
				a.narrowRange()
			}
			ui.Textf(c, "共 %d 个地址，扫描会比较慢", n).FontSize(12).TextColor(t.TextMuted).SingleLine()
		})
	}
}

// ipField is one field holding both addresses: eight octet cells with
// dots between the octets, and 至 between the two addresses, which reads
// as the span it is where a dash reads as a minus.
func (a *app) ipField(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(1).AlignItems(ui.Center).Padding(0, 12).Height(34).
		Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Shrink(0).Children(func() {
		for i := range 8 {
			if i == 4 {
				ui.Text(c, "至").FontSize(12).TextColor(t.TextMuted).MarginX(8).Shrink(0)
			} else if i%4 != 0 {
				ui.Text(c, ".").FontSize(13).TextColor(t.TextMuted).Shrink(0)
			}
			a.ipCell(c, t, i)
		}
	})
}

// cellLabel names one octet cell so assistive technology can tell which
// address and which octet it holds.
func cellLabel(i int) string {
	if i < 4 {
		return "开始 IP 第 " + strconv.Itoa(i+1) + " 段"
	}
	return "结束 IP 第 " + strconv.Itoa(i-3) + " 段"
}

// ipCell builds one octet cell: figures in equal width, centered so a
// short octet sits in the middle of its cell rather than hugging an edge.
// A locked cell shows a network octet of the NIC, disabled and grayed. In
// an editable cell the third digit moves on to the next one, and
// Backspace on an empty one goes back a cell; the moves land through
// a.focusCell, whose wish ipCell picks up as it builds.
func (a *app) ipCell(c *ui.Context, t *ui.Theme, i int) {
	if a.cellLocked(i) {
		ui.TextInputBase(c, &a.ipCells[i]).Width(26).FontSize(13).FontFeatures("tnum").
			TextAlign(ui.Center).Disabled(true).TextColor(t.TextMuted).
			Label(cellLabel(i) + "（网段，自动）")
		return
	}
	cell := ui.TextInputBase(c, &a.ipCells[i]).Width(26).FontSize(13).FontFeatures("tnum").
		TextAlign(ui.Center).Padding(2, 0).Label(cellLabel(i)).Disabled(a.scanning)
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

// statusBarHeight is the bar's height, shared by the bar and the room the
// body leaves it.
const statusBarHeight = 32

// statusBar sits at the bottom, outside the cards, so no panel drag can
// push it out of the window: tallies, the scan progress, errors, and the
// offline filter.
func (a *app) statusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(statusBarHeight).Padding(0, gutter).Gap(18).AlignItems(ui.Center).Shrink(0).Children(func() {
		// The green dot is about who is here now; it should not stay lit
		// when nobody is.
		a.stat(c, t, "在线", a.onlineCount(), a.onlineCount() > 0)
		a.stat(c, t, "已发现", a.discoveredCount(), false)
		a.stat(c, t, "总数", a.progress.Total, false)
		ui.Box(c).Grow(1)
		switch {
		case a.scanStopping:
			ui.Spinner(c)
			ui.Text(c, "正在停止…").FontSize(12).TextColor(t.TextMuted)
		case a.scanning:
			pct := 0.0
			if a.progress.Total > 0 {
				pct = float64(a.progress.Scanned) / float64(a.progress.Total)
			}
			ui.Progress(c, pct).Width(140).Height(3).Shrink(0)
			ui.Textf(c, "已扫描 %d/%d", a.progress.Scanned, a.progress.Total).
				FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
		case a.sweepLeft > 0:
			ui.Spinner(c)
			ui.Textf(c, "正在查看 %d 个地址的端口", a.sweepLeft).
				FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
		}
		if a.scanErr != "" {
			ui.Text(c, a.scanErr).FontSize(12).TextColor(t.Danger).SingleLine()
		}
		ui.Box(c).Grow(1)
		a.wakeStatus(c, t)
		// The bar carries the controls, so they carry no frame of their own:
		// the toolbar style drops the border and leaves only the hover tint,
		// as a status bar's do. The gap keeps them off the tallies beside.
		ui.Toolbar(c, func() {
			ui.Box(c).MarginX(8)
			// Offered only when there is something to copy; a button that
			// says "nothing here" is a wasted click.
			if len(a.visibleHosts()) > 0 {
				if flatButton(c, "复制清单").Clicked() {
					a.copyInventory(c)
				}
			}
			a.wakeButton(c)
			a.aboutButton(c, t)
		})
	})
	// Brief outcomes are said once, as a toast, so the bar keeps its width
	// however long the words are.
	if a.wakeNote != "" {
		c.Toast(a.wakeNote)
		a.wakeNote = ""
	}
	if a.scanNote != "" {
		c.Toast(a.scanNote)
		a.scanNote = ""
	}
	a.wakeDialog(c, t)
}

// aboutButton is the status bar's quiet 关于: the version and the update
// check in a popover above it.
func (a *app) aboutButton(c *ui.Context, t *ui.Theme) {
	b := flatButton(c, "关于")
	if b.Clicked() {
		a.aboutOpen = !a.aboutOpen
	}
	ui.Popover(c, b, &a.aboutOpen, func() {
		ui.Column(c).Gap(10).Padding(14).Width(280).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "局域网 IP 扫描").FontSize(13).FontWeight(600).Shrink(0)
				ui.Text(c, mygo.App.Version()).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Text(c, "作者 xiyaowong · MIT 许可证").FontSize(12).TextColor(t.TextMuted)
			a.updateRow(c, t)
		})
	})
}

// stat is one tally: an optional dot, a muted caption, then the number,
// semibold and in equal-width figures so it does not jitter.
func (a *app) stat(c *ui.Context, t *ui.Theme, label string, n int, dot bool) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		if dot {
			ui.Box(c).Size(7, 7).Radius(3.5).Background(t.Success).Shrink(0)
		}
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		ui.Textf(c, "%d", n).FontSize(12).FontWeight(600).FontFeatures("tnum")
	})
}

// tablePane is the host tree, or what stands in for it while there is
// nothing to show.
func (a *app) tablePane(c *ui.Context, t *ui.Theme) {
	visible := a.visibleHosts()
	if len(visible) == 0 {
		a.emptyTable(c, t, len(a.hosts) > 0)
		return
	}
	a.hostTree(c, t, visible)
}

// emptyTable fills the table's room with the one thing to do next.
func (a *app) emptyTable(c *ui.Context, t *ui.Theme, hidden bool) {
	ui.Column(c).Fill().Center().Gap(10).Children(func() {
		switch {
		case a.scanning:
			ui.Spinner(c)
			ui.Text(c, "正在扫描地址…").FontSize(13).TextColor(t.TextMuted)
		case hidden:
			ui.Text(c, "这个范围里没有在线的地址").FontSize(15).FontWeight(600)
			ui.Text(c, "打开右下角的「显示离线地址」，可以看到上次扫描过的地址").FontSize(13).TextColor(t.TextMuted)
		default:
			ui.Text(c, "还没有扫描").FontSize(15).FontWeight(600)
			ui.Text(c, "点右上角的「开始扫描」，检查这个范围里哪些地址在线").FontSize(13).TextColor(t.TextMuted)
		}
	})
}

// hostTree shows the hosts, each with its open ports as branches below
// it once a scan of the device found any.
func (a *app) hostTree(c *ui.Context, t *ui.Theme, visible []model.Host) {
	roots := make([]treeKey, len(visible))
	for i, h := range visible {
		roots[i] = treeKey{IP: h.IP}
	}
	// The list lights a chosen row with the theme's accent, which floods it;
	// Fluent selects with a tint and keeps the text as it is, so the tree is
	// built with an accent that is that tint, and with the muted ink and
	// badges deepened enough to stay legible on it.
	tint := *t
	tint.Accent, tint.AccentText = chosenRow(t), t.Text
	tint.TextMuted = chosenMuted(t)
	c.SetTheme(&tint)
	// Beside the detail panel the card keeps only what identifies a device;
	// the rest of the columns go where they would be cut off.
	cols := hostTreeColumns
	if cardWidth(c, a) < 600 {
		cols = hostTreeColumnsNarrow
	}
	ui.OutlineTable(c, &a.tree, cols, roots, a.portBranches,
		func(k treeKey, col int) { a.treeCell(c, cols[col].ID, k) }).Grow(1)
	c.SetTheme(t)
	a.readTreeSelection()
}

// readTreeSelection points selIP at the host of the chosen row, a port
// row choosing its host, and drops the selection when the row is gone.
// Picking another host opens its details with a port scan of its own,
// once per sweep until the user scans again.
func (a *app) readTreeSelection() {
	if a.selRow >= 0 && a.selRow < a.tree.Rows() {
		ip := a.tree.Item(a.selRow).IP
		if ip != a.selIP {
			a.selIP = ip
			a.autoPortScan(ip)
		}
		return
	}
	a.selRow = -1
	if !a.selIP.IsValid() {
		return
	}
	for i := range a.tree.Rows() {
		if a.tree.Item(i).IP == a.selIP && a.tree.Item(i).Port == 0 {
			return // still listed; the row index settles next frame
		}
	}
	a.selIP = netip.Addr{}
}

// autoPortScan probes a host the user just opened, unless it was asked
// already this sweep: the details are worth the wait the first time a
// device is looked at, not on every return to it.
func (a *app) autoPortScan(ip netip.Addr) {
	if a.portScanning {
		return
	}
	if _, done := a.ports[ip]; done {
		return
	}
	a.startPortScan(ip)
}

// portBranches lists the open ports of a host as the tree's child rows;
// nil for a host without any, and for a port row.
func (a *app) portBranches(k treeKey) []treeKey {
	if k.Port != 0 {
		return nil
	}
	ports := openPorts(a.ports[k.IP])
	if len(ports) == 0 {
		return nil
	}
	out := make([]treeKey, len(ports))
	for i, p := range ports {
		out[i] = treeKey{IP: k.IP, Port: p.Number}
	}
	return out
}

// treeCell builds one cell of the host tree.
func (a *app) treeCell(c *ui.Context, col string, k treeKey) {
	t := c.Theme()
	h, known := a.hosts[k.IP]
	text := cellText(t, known && h.State == model.StateOffline)
	port := k.Port != 0
	switch col {
	case "ip":
		if port {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "%d", k.Port).FontSize(12).FontWeight(600).FontFeatures("tnum").TextColor(text)
				ui.Text(c, portLabel(model.Port{Number: k.Port, Service: serviceName(a.ports[k.IP], k.Port)})).
					FontSize(12).TextColor(t.TextMuted)
			})
			return
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(stateDot(t, h)).Shrink(0)
			ui.Text(c, h.IP.String()).FontFeatures("tnum").TextColor(text)
			if a.isSelf(h.IP) {
				bg, fg := softPill(t)
				pill(c, "本机", bg, fg)
			}
			if h.Gateway {
				bg, fg := gatewayPill(t)
				pill(c, "网关", bg, fg)
			}
			if h.ViaMDNS {
				bg, fg := softPill(t)
				pill(c, "服务发现", bg, fg)
			}
		})
	case "hostname":
		// The port's own name reads already in the first cell.
		if !port {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if kind := deviceKindOf(h, a.ports[k.IP]); kind != kindUnknown {
					ui.Text(c, kind.Label()).FontSize(12).TextColor(t.Accent).Shrink(0)
				}
				ui.Text(c, orDash(h.Hostname)).SingleLine().TextColor(text).Tooltip(h.Hostname)
			})
		}
	case "vendor":
		if !port {
			ui.Text(c, orDash(h.Vendor)).SingleLine().TextColor(text).Tooltip(h.Vendor)
		}
	case "mac":
		if !port {
			ui.Text(c, orDash(h.MAC)).FontFeatures("tnum").TextColor(text)
		}
	case "state":
		if port {
			ui.Text(c, "开放").FontSize(12).TextColor(t.Success)
			return
		}
		ui.Text(c, stateLabel(h.State)).FontSize(12).TextColor(t.TextMuted)
	}
	// "space" holds nothing: it is the room left over.
}

// stateDot is the status dot of a host row.
func stateDot(t *ui.Theme, h model.Host) ui.Color {
	switch h.State {
	case model.StateOnline:
		return t.Success
	case model.StateOffline:
		return t.TextMuted
	}
	return t.TextMuted
}

// stateLabel is a host's reachability in words.
func stateLabel(s model.HostState) string {
	switch s {
	case model.StateOnline:
		return "在线"
	case model.StateOffline:
		return "离线"
	}
	return "未知"
}

// cellText grays an offline row's text down.
func cellText(t *ui.Theme, offline bool) ui.Color {
	if offline {
		return t.TextMuted
	}
	return t.Text
}

// serviceName is the well-known name of a port in a host's results.
func serviceName(ports []model.Port, n int) string {
	for _, p := range ports {
		if p.Number == n {
			return p.Service
		}
	}
	return ""
}

// openPorts lists the open ports of a scan by number.
func openPorts(ports []model.Port) []model.Port {
	out := make([]model.Port, 0, len(ports))
	for _, p := range ports {
		if p.Open {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(x, y model.Port) int { return x.Number - y.Number })
	return out
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
