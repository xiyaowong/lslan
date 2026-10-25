// Command lslan is a Windows LAN IP scanner desktop app built on mygo.
package ui

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// scannerIface and pingSessionIface exist so test fakes can replace the
// engine's concrete types.
type scannerIface interface {
	Events() <-chan scan.Event
	Start(nic model.NIC, start, end netip.Addr) error
	Cancel()
}

type pingSessionIface interface {
	Samples() <-chan model.PingSample
	Cancel()
}

// pingStats is the running tally of the current ping session.
type pingStats struct {
	sent, recv int
	rttTotal   time.Duration
}

func (s pingStats) lossPct() float64 {
	if s.sent == 0 {
		return 0
	}
	return float64(s.sent-s.recv) / float64(s.sent) * 100
}

func (s pingStats) avgRTT() time.Duration {
	if s.recv == 0 {
		return 0
	}
	return s.rttTotal / time.Duration(s.recv)
}

// app is all state the view shows. Goroutines change it only through
// dispatch (main thread + repaint).
type app struct {
	// Engine entry points; fakes in tests.
	listNICs    func() ([]model.NIC, error)
	rangeForNIC func(model.NIC) (netip.Addr, netip.Addr)
	newScanner  func() scannerIface
	portPresets func() []scan.Preset
	scanPorts   func(ctx context.Context, ip netip.Addr, ports []int) ([]model.Port, error)
	startPingFn func(ctx context.Context, ip netip.Addr) (pingSessionIface, error)

	// win is nil in tests, where dispatch queues onto pending, drained at
	// the view's start (goroutine updates must not touch the test goroutine).
	win       *mygo.Window
	pendingMu sync.Mutex
	pending   []func()

	// Scan state.
	scanner  scannerIface
	pumpOnce sync.Once
	scanning bool
	scanErr  string
	progress model.Progress
	hosts    map[netip.Addr]model.Host

	// Toolbar state.
	nics       []model.NIC
	nicOptions []string
	nicSel     string
	nicIdx     int
	startIP    string
	endIP      string

	// Table state.
	tableState  ui.ListState
	sort        ui.SortOrder
	showOffline bool
	selRow      int
	selIP       netip.Addr

	// Ping state; pingGen drops samples of a stopped session.
	pingGen    int
	pingCancel context.CancelFunc
	pingIP     netip.Addr
	pingLines  []string
	pingStats  pingStats
	pingList   ui.ListState

	// Port scan state.
	presetNames  []string
	presetSel    string
	presetCache  []scan.Preset
	portPreset   int
	portScanning bool
	portIP       netip.Addr
	portResults  []model.Port
	portErr      string
}

func newApp() *app {
	a := &app{
		listNICs:    scan.ListNICs,
		rangeForNIC: scan.RangeForNIC,
		newScanner:  func() scannerIface { return scan.NewScanner() },
		portPresets: scan.PortPresets,
		scanPorts:   scan.ScanPorts,
		startPingFn: func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) {
			return scan.StartPing(ctx, ip)
		},
		hosts:  map[netip.Addr]model.Host{},
		selRow: -1,
		sort:   ui.SortOrder{Column: "ip"},
	}
	a.tableState = ui.ListState{Selected: &a.selRow, Sort: &a.sort}
	a.pingList = ui.ListState{FollowEnd: true}
	return a
}

// dispatch runs fn on the main thread and repaints; safe from any goroutine.
func (a *app) dispatch(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	a.pendingMu.Lock()
	a.pending = append(a.pending, fn)
	a.pendingMu.Unlock()
}

// runPending drains queued dispatch fns; the view calls it first in tests.
func (a *app) runPending() {
	a.pendingMu.Lock()
	fns := a.pending
	a.pending = nil
	a.pendingMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// NewApp creates the app state.
func NewApp() *app { return newApp() }

// AttachWindow binds the app to the main window and starts NIC loading.
func (a *app) AttachWindow(opts mygo.WindowOptions) {
	opts.Content = ui.View(a.View)
	a.win = mygo.NewWindow(opts)
	go a.loadNICs()
}

// loadNICs reads the machine's adapters and selects the first one.
func (a *app) loadNICs() {
	nics, err := a.listNICs()
	a.dispatch(func() {
		if err != nil {
			a.scanErr = "读取网卡失败: " + err.Error()
			return
		}
		a.setNICs(nics)
	})
}

func (a *app) setNICs(nics []model.NIC) {
	a.nics = nics
	a.nicOptions = a.nicOptions[:0]
	for _, n := range nics {
		a.nicOptions = append(a.nicOptions, fmt.Sprintf("%s (%s)", n.Name, n.IP))
	}
	if len(nics) > 0 {
		a.nicIdx = 0
		a.nicSel = a.nicOptions[0]
		a.fillRange()
	}
}

func (a *app) selectedNIC() *model.NIC {
	if a.nicIdx < 0 || a.nicIdx >= len(a.nics) {
		return nil
	}
	return &a.nics[a.nicIdx]
}

// fillRange fills the sweep range from the NIC, trimmed to usable hosts.
func (a *app) fillRange() {
	nic := a.selectedNIC()
	if nic == nil {
		return
	}
	start, end := a.rangeForNIC(*nic)
	start, end = trimRangeEdges(start, end)
	a.startIP, a.endIP = start.String(), end.String()
}

// trimRangeEdges drops network/broadcast addresses of a subnet-sized range.
func trimRangeEdges(start, end netip.Addr) (netip.Addr, netip.Addr) {
	n, ok := rangeCount(start, end)
	if !ok || n > 256 {
		return start, end
	}
	sb, eb := start.As4(), end.As4()
	if sb[3] == 0 {
		start = netip.AddrFrom4([4]byte{sb[0], sb[1], sb[2], 1})
	}
	if eb[3] == 255 {
		end = netip.AddrFrom4([4]byte{eb[0], eb[1], eb[2], 254})
	}
	return start, end
}

// rangeCount is the number of addresses from start through end.
func rangeCount(start, end netip.Addr) (uint64, bool) {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() || end.Compare(start) < 0 {
		return 0, false
	}
	s, e := binary.BigEndian.Uint32(start.AsSlice()), binary.BigEndian.Uint32(end.AsSlice())
	return uint64(e) - uint64(s) + 1, true
}

// parseRange validates the two range inputs as an IPv4 span.
func (a *app) parseRange() (start, end netip.Addr, ok bool) {
	start, err1 := netip.ParseAddr(strings.TrimSpace(a.startIP))
	end, err2 := netip.ParseAddr(strings.TrimSpace(a.endIP))
	ok = err1 == nil && err2 == nil && start.IsValid() && end.IsValid() &&
		start.Is4() && end.Is4() && end.Compare(start) >= 0
	return start, end, ok
}

// startScan begins one sweep on the selected range.
func (a *app) startScan() {
	if a.scanning {
		return
	}
	nic := a.selectedNIC()
	if nic == nil {
		a.scanErr = "请先选择网卡"
		return
	}
	start, end, ok := a.parseRange()
	if !ok {
		a.scanErr = "IP 范围无效"
		return
	}
	if a.scanner == nil {
		a.scanner = a.newScanner()
	}
	a.startPump()
	if err := a.scanner.Start(*nic, start, end); err != nil {
		a.scanErr = err.Error()
		return
	}
	n, _ := rangeCount(start, end)
	a.scanning = true
	a.scanErr = ""
	a.progress = model.Progress{Total: int(n)}
	a.hosts = map[netip.Addr]model.Host{}
	a.selRow, a.selIP = -1, netip.Addr{}
}

// startPump drains the scanner's event stream once, for its lifetime: the
// channel stays open across sweeps.
func (a *app) startPump() {
	a.pumpOnce.Do(func() {
		events := a.scanner.Events()
		go func() {
			for ev := range events {
				a.dispatch(func() { a.applyEvent(ev) })
			}
		}()
	})
}

// applyEvent folds one engine event into the state; main thread only.
func (a *app) applyEvent(ev scan.Event) {
	switch ev.Kind {
	case scan.EventProgress:
		a.progress = ev.Progress
	case scan.EventHost:
		if ev.Host != nil {
			a.hosts[ev.Host.IP] = *ev.Host
		}
	case scan.EventDone:
		a.scanning = false
		a.progress = ev.Progress
	case scan.EventError:
		a.scanning = false
		a.scanErr = ev.Err.Error()
	}
}

// stopPing ends the current session; its in-flight samples are dropped by
// pingGen.
func (a *app) stopPing() {
	if a.pingCancel == nil {
		return
	}
	a.pingGen++
	cancel := a.pingCancel
	a.pingCancel = nil
	cancel()
}

// startPing pings ip, stopping any session still running.
func (a *app) startPing(ip netip.Addr) {
	a.stopPing()
	ctx, cancel := context.WithCancel(context.Background())
	a.pingGen++
	gen := a.pingGen
	a.pingCancel = cancel
	a.pingIP = ip
	a.pingLines = nil
	a.pingStats = pingStats{}
	sess, err := a.startPingFn(ctx, ip)
	if err != nil {
		a.pingLines = append(a.pingLines, "启动失败: "+err.Error())
		a.pingCancel = nil
		cancel()
		return
	}
	go func() {
		for s := range sess.Samples() {
			a.dispatch(func() {
				if a.pingGen == gen {
					a.applyPingSample(s)
				}
			})
		}
		a.dispatch(func() {
			if a.pingGen == gen {
				a.pingCancel = nil
			}
		})
		cancel()
	}()
}

func (a *app) applyPingSample(s model.PingSample) {
	st := &a.pingStats
	st.sent++
	line := "请求超时。"
	if s.OK {
		st.recv++
		st.rttTotal += s.RTT
		line = fmt.Sprintf("来自 %s: 字节=32 时间=%.1fms TTL=%d", a.pingIP, s.RTT.Seconds()*1000, s.TTL)
	}
	a.pingLines = append(a.pingLines, line)
	if len(a.pingLines) > 400 {
		a.pingLines = append([]string(nil), a.pingLines[len(a.pingLines)-300:]...)
	}
}

// presets caches the engine's port presets.
func (a *app) presets() []scan.Preset {
	if a.presetCache == nil {
		a.presetCache = a.portPresets()
		a.presetNames = make([]string, len(a.presetCache))
		for i, p := range a.presetCache {
			a.presetNames[i] = p.Name
		}
		if len(a.presetCache) > 0 {
			a.portPreset = 0
			a.presetSel = a.presetCache[0].Name
		}
	}
	return a.presetCache
}

// startPortScan probes ip with the chosen preset.
func (a *app) startPortScan(ip netip.Addr) {
	if a.portScanning {
		return
	}
	ps := a.presets()
	if a.portPreset < 0 || a.portPreset >= len(ps) {
		return
	}
	preset := ps[a.portPreset]
	ctx, cancel := context.WithCancel(context.Background())
	a.portScanning = true
	a.portIP = ip
	a.portResults = nil
	a.portErr = ""
	go func() {
		ports, err := a.scanPorts(ctx, ip, preset.Ports)
		cancel()
		a.dispatch(func() {
			a.portScanning = false
			if err != nil {
				a.portErr = err.Error()
				return
			}
			a.portResults = ports
		})
	}()
}

// visibleHosts lists the table's rows, filtered and sorted per state.
func (a *app) visibleHosts() []model.Host {
	out := make([]model.Host, 0, len(a.hosts))
	for _, h := range a.hosts {
		if !a.showOffline && h.State == model.StateOffline {
			continue
		}
		out = append(out, h)
	}
	sortHosts(out, a.sort)
	return out
}

func sortHosts(hs []model.Host, so ui.SortOrder) {
	var cmp func(x, y model.Host) int
	switch so.Column {
	case "state":
		cmp = func(x, y model.Host) int { return int(x.State) - int(y.State) }
	case "host":
		cmp = func(x, y model.Host) int { return x.LastOctet() - y.LastOctet() }
	case "vendor":
		cmp = func(x, y model.Host) int { return strings.Compare(x.Vendor, y.Vendor) }
	case "mac":
		cmp = func(x, y model.Host) int { return strings.Compare(x.MAC, y.MAC) }
	case "hostname":
		cmp = func(x, y model.Host) int { return strings.Compare(x.Hostname, y.Hostname) }
	case "ports":
		cmp = func(x, y model.Host) int { return openPortCount(x) - openPortCount(y) }
	default:
		cmp = func(x, y model.Host) int { return x.IP.Compare(y.IP) }
	}
	slices.SortStableFunc(hs, cmp)
	if so.Descending {
		slices.Reverse(hs)
	}
}

func openPortCount(h model.Host) int {
	n := 0
	for _, p := range h.Ports {
		if p.Open {
			n++
		}
	}
	return n
}

// portsSummary renders the port column, e.g. "80, 443, +2".
func portsSummary(ports []model.Port) string {
	var open []string
	for _, p := range ports {
		if p.Open {
			open = append(open, fmt.Sprintf("%d", p.Number))
		}
	}
	switch {
	case len(open) == 0:
		return ""
	case len(open) <= 3:
		return strings.Join(open, ", ")
	default:
		return fmt.Sprintf("%s +%d", strings.Join(open[:3], ", "), len(open)-3)
	}
}

// syncSelection keeps selIP pointing at the same host as rows move.
func (a *app) syncSelection(visible []model.Host) {
	if a.selRow >= 0 && a.selRow < len(visible) {
		a.selIP = visible[a.selRow].IP
		return
	}
	a.selRow = -1
	if a.selIP.IsValid() {
		for i, h := range visible {
			if h.IP == a.selIP {
				a.selRow = i
				break
			}
		}
	}
	if a.selRow < 0 {
		a.selIP = netip.Addr{}
	}
}

func (a *app) onlineCount() int {
	n := 0
	for _, h := range a.hosts {
		if h.State == model.StateOnline {
			n++
		}
	}
	return n
}

// onlineIPs lists online hosts' addresses, one per line.
func (a *app) onlineIPs() string {
	var ips []netip.Addr
	for _, h := range a.hosts {
		if h.State == model.StateOnline {
			ips = append(ips, h.IP)
		}
	}
	slices.SortFunc(ips, netip.Addr.Compare)
	parts := make([]string, len(ips))
	for i, ip := range ips {
		parts[i] = ip.String()
	}
	return strings.Join(parts, "\n")
}
