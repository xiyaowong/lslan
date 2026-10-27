// Command lslan is a Windows LAN IP scanner desktop app built on mygo.
package ui

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// scannerIface and pingSessionIface let tests swap in fakes.
type scannerIface interface {
	Events() <-chan scan.Event
	Start(nic model.NIC, start, end netip.Addr) error
	Cancel()
}

type pingSessionIface interface {
	Samples() <-chan model.PingSample
	Cancel()
}

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
	scanPorts   func(ctx context.Context, src, ip netip.Addr, ports []int) ([]model.Port, error)
	startPingFn func(ctx context.Context, ip netip.Addr) (pingSessionIface, error)
	newPingTerm func() (pingTerminalIface, error)
	dialTelnet  func(ip netip.Addr, port int) (net.Conn, error)
	newTelnet   func(conn net.Conn) (telnetTermIface, error)
	wakeSend    func(nic model.NIC, mac string) error
	wakeMac     func(ip netip.Addr) string
	wakeProbe   func(src, ip netip.Addr) bool
	wakeEvery   time.Duration
	wakeFor     time.Duration

	// win is nil in tests, where dispatch queues onto pending, drained at
	// the view's start (goroutine updates must not touch the test goroutine).
	win       *mygo.Window
	pendingMu sync.Mutex
	pending   []func()

	// Scan state.
	scanner  scannerIface
	pumpOnce sync.Once
	scanning bool
	// scanStopping is set the moment the user cancels, so the button can
	// say so while the engine unwinds; scanCancelled remembers it was a
	// cancel, so the end of the sweep neither reports a result nor starts
	// the port probes.
	scanStopping  bool
	scanCancelled bool
	scanErr       string
	scanNote      string
	progress      model.Progress
	hosts         map[netip.Addr]model.Host
	// withPorts makes a sweep probe the ports of every host it finds;
	// portGen drops the results of a sweep the user has since restarted,
	// and sweepLeft counts the hosts still to probe in this one.
	withPorts bool
	portGen   int
	sweepLeft int

	// Toolbar state.
	nics       []model.NIC
	nicOptions []string
	nicSel     string
	nicIdx     int
	// nicErr says reading the adapters failed, which the adapter row shows
	// with a retry; it is not the same as a machine without adapters.
	nicErr string

	// ipCells are the segmented control's eight cells: the start address's
	// four octets, then the end's. cellLock marks that both ends share a
	// prefix, so the three network cells of each lock. lastCell is the
	// cell to focus next frame after a jump, -1 for none.
	ipCells  [8]string
	cellLock bool
	lastCell int

	// Host tree: the hosts as roots, each with its open ports as branches
	// below it. selRow is the row chosen and selIP the host it belongs to,
	// a port row belonging to its host, so the detail panel always shows a
	// host.
	tree        ui.OutlineState[treeKey]
	sort        ui.SortOrder
	showOffline bool
	selRow      int
	selIP       netip.Addr

	// cardWidth is the host card's width in DIPs while the detail panel
	// shows; the panel takes the rest, so a wide window widens the details
	// rather than the table.
	cardWidth float32

	// Ping state; pingGen drops samples of a stopped session. pingTerm is
	// the terminal plugin view of the session, nil when it could not be
	// created (the plain line list stands in).
	pingGen    int
	pingCancel context.CancelFunc
	pingIP     netip.Addr
	pingLines  []string
	pingStats  pingStats
	pingList   ui.ListState
	pingTerm   pingTerminalIface

	// Telnet test state (#54): one interactive session at a time, shared
	// with ping — starting either ends the other, and the pane at the
	// panel's foot shows the one running for this host. telnetGen drops
	// a dial the user has since ended.
	telnetGen     int
	telnetPending bool
	telnetIP      netip.Addr
	telnetPort    int
	telnetTerm    telnetTermIface
	telnetErr     string

	// Route trace state: one run at a time, its process killed on stop.
	// traceGen drops output of a run the user has since restarted; the
	// pane shows the hops as they land and the ✕ once the run ends.
	traceGen     int
	traceCancel  context.CancelFunc
	traceIP      netip.Addr
	traceLines   []string
	traceList    ui.ListState
	traceStarter traceStarter

	// netbios is the one-at-a-time name query for a device whose
	// hostname is missing; the answer fills the facts, never overwrites.
	netbios netbiosState

	// wake is the 远程唤醒 tool: a dialog off the status bar, then the
	// wait for the woken device to answer.
	wake     wakeState
	wakeNote string

	// Port scan state.
	presetCache  []scan.Preset
	portScanning bool
	portIP       netip.Addr
	portErr      string
	portScroll   ui.ScrollState
	// ports is the cache of port scan results, by address: a key present
	// means the host has been asked (an empty slice, that nothing was
	// open). The engine knows nothing of ports, so they live apart from
	// hosts and only a new sweep clears them; sweepPending holds those the
	// sweep is probing now, so their details can say so.
	ports        map[netip.Addr][]model.Port
	sweepPending map[netip.Addr]bool
	// moreOpen shows the popover with the port presets beyond the common one.
	moreOpen bool

	// logo is the app icon drawn into the title bar, nil if none given
	// or it failed to decode.
	logo *ui.Bitmap

	// dark is the appearance shown: the system's at launch, flipped by
	// the title bar button after that (#46). The engine side follows
	// through mygo.Theme (AttachWindow).
	dark bool

	// updater is the app's self-update state (mygo.Updater), driven from
	// the about popover.
	updater updateState

	// aboutOpen shows the about popover of the title bar.
	aboutOpen bool
}

func newApp() *app {
	a := &app{
		listNICs:    scan.ListNICs,
		rangeForNIC: scan.RangeForNIC,
		newScanner:  func() scannerIface { return scan.NewScanner() },
		portPresets: scan.PortPresets,
		scanPorts:   scan.ScanPortsFrom,
		startPingFn: func(ctx context.Context, ip netip.Addr) (pingSessionIface, error) {
			return scan.StartPing(ctx, ip)
		},
		newPingTerm: newTerminal,
		dialTelnet: func(ip netip.Addr, port int) (net.Conn, error) {
			return net.DialTimeout("tcp", netip.AddrPortFrom(ip, uint16(port)).String(), 3*time.Second)
		},
		newTelnet: buildTelnetTerm,
		wakeSend:  scan.SendWake,
		wakeMac:   scan.MACFor,
		wakeProbe: scan.Probe,
		wakeEvery: wakeWaitEvery,
		wakeFor:   wakeWaitFor,
		hosts:     map[netip.Addr]model.Host{},
		ports:     map[netip.Addr][]model.Port{},
		selRow:    -1,
		lastCell:  -1,
		sort:      ui.SortOrder{Column: "ip"},
		cardWidth: cardWidthDefault,
	}
	a.tree.List = ui.ListState{Selected: &a.selRow, Sort: &a.sort}
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

// NewApp creates the app state; a nil or undecodable icon just leaves the
// logo out.
func NewApp(iconPNG []byte) *app {
	a := newApp()
	if bmp, err := ui.DecodeBitmap(iconPNG); err == nil {
		a.logo = bmp
	}
	return a
}

func (a *app) AttachWindow(opts mygo.WindowOptions) {
	a.dark = mygo.Theme.IsDark()
	mygo.Theme.OnUpdated(func() {
		if mygo.Theme.Source() != mygo.ThemeSystem {
			return
		}
		a.dispatch(func() { a.setDark(mygo.Theme.IsDark()) })
	})
	opts.Content = ui.View(a.View)
	a.win = mygo.NewWindow(opts)
	a.win.OnClosed(func() { a.stopPing(); a.stopWakeWait() }) // ends the telnet test too
	go a.loadNICs()
}

// toggleTheme flips the appearance and holds it there: the button has the
// two states and no "follow the system" one, so the choice is an explicit
// override from the first click on (#46).
func (a *app) toggleTheme() {
	dark := !a.dark
	a.setDark(dark)
	if dark {
		mygo.Theme.SetSource(mygo.ThemeDark)
	} else {
		mygo.Theme.SetSource(mygo.ThemeLight)
	}
}

// setDark shows the given appearance; the terminal, which paints itself,
// is told apart from the theme.
func (a *app) setDark(dark bool) {
	a.dark = dark
	if a.pingTerm != nil {
		a.pingTerm.setDark(dark)
	}
}

// loadNICs reads the machine's adapters and selects the first one.
func (a *app) loadNICs() {
	nics, err := a.listNICs()
	a.dispatch(func() {
		if err != nil {
			// An empty list, not nil: the difference is "loaded, none
			// found" versus "still loading", and the row offers a retry.
			a.nics, a.nicOptions = []model.NIC{}, nil
			a.nicErr = "没读到网卡"
			return
		}
		a.nicErr = ""
		a.setNICs(nics)
	})
}

func (a *app) setNICs(nics []model.NIC) {
	a.nics = nics
	a.nicOptions = a.nicOptions[:0]
	for _, n := range nics {
		name := fmt.Sprintf("%s (%s)", n.Name, n.IP)
		if n.Virtual {
			name += " 虚拟"
		}
		a.nicOptions = append(a.nicOptions, name)
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
	a.setRange(start, end)
}

func (a *app) setRange(start, end netip.Addr) {
	if !start.IsValid() || !end.IsValid() {
		return
	}
	sb, eb := start.As4(), end.As4()
	for i := range 4 {
		a.ipCells[i], a.ipCells[i+4] = strconv.Itoa(int(sb[i])), strconv.Itoa(int(eb[i]))
	}
	if _, _, _, ok := splitRange(start, end); ok {
		a.cellLock = true
	} else {
		a.cellLock = false
	}
	a.lastCell = -1
}

// cellLocked tells whether cell i shows a locked network octet: in a
// locked control, every cell but the host cell of each address.
func (a *app) cellLocked(i int) bool {
	return a.cellLock && i%4 != 3
}

func (a *app) octet(i int) int {
	n, err := strconv.Atoi(a.ipCells[i])
	if err != nil {
		return -1
	}
	return n
}

// cellAddr builds the address of the first (base 0) or second (base 4)
// cell group; ok is false while any of its cells is not a full octet.
func (a *app) cellAddr(base int) (netip.Addr, bool) {
	var b [4]byte
	for i := range 4 {
		n := a.octet(base + i)
		if n < 0 {
			return netip.Addr{}, false
		}
		b[i] = byte(n)
	}
	return netip.AddrFrom4(b), true
}

// fullRange reads the segmented control as an IPv4 span; it fails while
// the user is mid-edit and an address is incomplete.
func (a *app) fullRange() (start, end netip.Addr, ok bool) {
	start, ok1 := a.cellAddr(0)
	end, ok2 := a.cellAddr(4)
	if !ok1 || !ok2 {
		return start, end, false
	}
	if end.Compare(start) < 0 {
		start, end = end, start
	}
	return start, end, true
}

// parseRange reads the cells as a valid ordered span; startScan goes
// through it.
func (a *app) parseRange() (start, end netip.Addr, ok bool) {
	start, end, ok = a.fullRange()
	return start, end, ok
}

func (a *app) narrowRange() {
	start, _, ok := a.fullRange()
	if !ok {
		return
	}
	b := start.As4()
	a.setRange(
		netip.AddrFrom4([4]byte{b[0], b[1], b[2], 1}),
		netip.AddrFrom4([4]byte{b[0], b[1], b[2], 254}),
	)
}

func (a *app) rangeTotal() (uint64, bool) {
	start, end, ok := a.fullRange()
	if !ok {
		return 0, false
	}
	return rangeCount(start, end)
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

func rangeCount(start, end netip.Addr) (uint64, bool) {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() || end.Compare(start) < 0 {
		return 0, false
	}
	s, e := binary.BigEndian.Uint32(start.AsSlice()), binary.BigEndian.Uint32(end.AsSlice())
	return uint64(e) - uint64(s) + 1, true
}

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
		a.scanErr = "IP 范围无效，请填写完开始和结束 IP"
		return
	}
	n, _ := rangeCount(start, end)
	if n > rangeLimit {
		a.scanErr = "范围太大了，请先收窄"
		return
	}
	if a.scanner == nil {
		a.scanner = a.newScanner()
	}
	a.startPump()
	if err := a.scanner.Start(*nic, start, end); err != nil {
		a.scanErr = "无法开始扫描，请检查网卡后重试"
		return
	}
	a.scanning = true
	a.scanStopping = false
	a.scanCancelled = false
	a.scanErr = ""
	a.progress = model.Progress{Total: int(n)}
	a.hosts = map[netip.Addr]model.Host{}
	a.ports = map[netip.Addr][]model.Port{}
	a.portGen++ // a sweep still probing ports is stale now
	a.sweepLeft = 0
	a.sweepPending = nil
	a.netbiosReset() // a device may have been renamed since
	a.selRow, a.selIP = -1, netip.Addr{}
}

// rangeFits reports whether the range is one the app is willing to scan.
func (a *app) rangeFits() bool {
	n, ok := a.rangeTotal()
	return ok && n <= rangeLimit
}

// cancelScan asks the engine to stop and says so at once, since the sweep
// can take a while to unwind and a dead button reads as a broken one.
func (a *app) cancelScan() {
	if !a.scanning || a.scanStopping {
		return
	}
	a.scanStopping = true
	a.scanCancelled = true
	a.scanner.Cancel()
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
		a.scanning, a.scanStopping = false, false
		a.progress = ev.Progress
		if a.scanCancelled {
			a.scanCancelled = false
			a.scanNote = "已取消扫描"
		} else if a.withPorts {
			a.sweepPorts()
		}
	case scan.EventError:
		a.scanning = false
		a.scanErr = "扫描出了点问题，已停止。可重新开始扫描"
	}
}

// stopPing ends the current session; its in-flight samples are dropped by
// pingGen. Safe to call with no session running. The pane stays up with
// the results until closePing clears it.
func (a *app) stopPing() {
	a.pingGen++
	a.stopTelnet() // the terminal pane holds one interactive session
	a.stopTrace()  // so does the route trace
	if a.pingCancel != nil {
		cancel := a.pingCancel
		a.pingCancel = nil
		cancel()
	}
	if a.pingTerm != nil {
		a.pingTerm.close()
		a.pingTerm = nil
	}
}

// closePing clears the finished session's pane entirely; the pane shows
// the ✕ only once pingCancel is nil, so this never ends a running one.
func (a *app) closePing() {
	a.pingIP = netip.Addr{}
	a.pingLines = nil
	a.pingStats = pingStats{}
	if a.pingTerm != nil {
		a.pingTerm.close()
		a.pingTerm = nil
	}
}

// startPing pings ip, stopping any session still running. The other
// tools' panes close whole: the foot holds one output at a time, and
// two panes of lines would fight for the frame.
func (a *app) startPing(ip netip.Addr) {
	a.stopPing()
	a.closeTrace()
	ctx, cancel := context.WithCancel(context.Background())
	a.pingGen++
	gen := a.pingGen
	a.pingCancel = cancel
	a.pingIP = ip
	a.pingLines = nil
	a.pingStats = pingStats{}
	if term, err := a.newPingTerm(); err == nil {
		a.pingTerm = term
		term.setDark(a.dark)
		term.feed("正在 Ping "+ip.String()+":", true)
	} else {
		// No terminal: say so in the line list, which must then show every
		// line, this one included, before the first sample arrives.
		a.pingLines = append(a.pingLines, "打不开实时输出窗口，下面用文字显示结果")
	}
	sess, err := a.startPingFn(ctx, ip)
	if err != nil {
		a.pingLines = append(a.pingLines, "Ping 没能启动，请稍后再试")
		a.feedPing("Ping 没能启动，请稍后再试", false)
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
	line, ok := "请求超时。", false
	if s.OK {
		st.recv++
		st.rttTotal += s.RTT
		line, ok = fmt.Sprintf("来自 %s: 字节=32 时间=%.1fms TTL=%d", a.pingIP, s.RTT.Seconds()*1000, s.TTL), true
	}
	a.pingLines = append(a.pingLines, line)
	if len(a.pingLines) > 400 {
		a.pingLines = append([]string(nil), a.pingLines[len(a.pingLines)-300:]...)
	}
	a.feedPing(line, ok)
}

func (a *app) feedPing(line string, ok bool) {
	if a.pingTerm != nil {
		a.pingTerm.feed(line, ok)
	}
}

// startTelnet opens the one-click test of a telnet port (#54): the
// device is dialed off the interface so a slow or dead one never freezes
// it, and the session lands in the panel's terminal pane when it answers.
func (a *app) startTelnet(ip netip.Addr, port int) {
	a.stopPing()
	a.stopTrace()
	a.closePing() // the foot holds one output at a time
	a.closeTrace()
	a.telnetGen++
	gen := a.telnetGen
	a.telnetIP, a.telnetPort = ip, port
	a.telnetPending, a.telnetErr = true, ""
	go func() {
		conn, err := a.dialTelnet(ip, port)
		a.dispatch(func() {
			if a.telnetGen != gen {
				if conn != nil {
					conn.Close()
				}
				return // ended while dialing
			}
			a.telnetPending = false
			if err != nil {
				a.telnetErr = telnetDialError(err, ip, port)
				return
			}
			term, err := a.newTelnet(conn)
			if err != nil {
				conn.Close()
				a.telnetErr = "打不开实时输出窗口，无法测试"
				return
			}
			a.telnetTerm = term
			term.setDark(a.dark)
		})
	}()
}

// telnetDialError turns a failed dial into words a home user can act on:
// a timeout means nothing answered, a refusal means the port is closed,
// and anything else is a guess.
func telnetDialError(err error, ip netip.Addr, port int) string {
	where := fmt.Sprintf("%s:%d", ip, port)
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return where + " 没有响应，设备可能已关机"
	case errors.Is(err, syscall.ECONNREFUSED):
		return where + " 拒绝了连接，远程登录可能没有开启"
	default:
		return "连接 " + where + " 失败，设备可能没开远程登录"
	}
}

// stopTelnet ends the test and clears its pane; safe with none running.
func (a *app) stopTelnet() {
	a.telnetGen++
	if a.telnetTerm != nil {
		a.telnetTerm.close()
		a.telnetTerm = nil
	}
	a.telnetPending = false
	a.telnetErr = ""
	a.telnetIP = netip.Addr{}
}

// presets caches the engine's port presets.
func (a *app) presets() []scan.Preset {
	if a.presetCache == nil {
		a.presetCache = a.portPresets()
	}
	return a.presetCache
}

// sweepPorts probes the ports of every host the sweep found online, a
// few at a time, and fills the results in as they come. A later sweep or
// a manual scan leaves the slow ones to their results, which portGen
// drops.
func (a *app) sweepPorts() {
	var ips []netip.Addr
	for ip, h := range a.hosts {
		if h.State != model.StateOffline {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return
	}
	a.sweepLeft = len(ips)
	a.sweepPending = make(map[netip.Addr]bool, len(ips))
	for _, ip := range ips {
		a.sweepPending[ip] = true
	}
	a.portGen++
	gen := a.portGen
	preset := a.presets()[0]
	src := a.sourceAddr()
	sem := make(chan struct{}, 4)
	go func() {
		var wg sync.WaitGroup
		for _, ip := range ips {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				ports, err := a.scanPorts(context.Background(), src, ip, preset.Ports)
				a.dispatch(func() {
					if a.portGen != gen {
						return
					}
					a.sweepLeft = max(a.sweepLeft-1, 0)
					delete(a.sweepPending, ip)
					if err != nil {
						return
					}
					// A scan the user asked for meanwhile is the newer word.
					if _, done := a.ports[ip]; !done {
						a.ports[ip] = ports
					}
				})
			}()
		}
		wg.Wait()
	}()
}

// startPortScan probes ip with the common preset.
func (a *app) startPortScan(ip netip.Addr) { a.startPortScanPreset(ip, 0) }

// startPortScanPreset probes ip with the preset at index i of presets().
func (a *app) startPortScanPreset(ip netip.Addr, i int) {
	if a.portScanning {
		return
	}
	ps := a.presets()
	if i < 0 || i >= len(ps) {
		return
	}
	preset := ps[i]
	ctx, cancel := context.WithCancel(context.Background())
	src := a.sourceAddr()
	a.portScanning = true
	a.portIP = ip
	a.portErr = ""
	go func() {
		ports, err := a.scanPorts(ctx, src, ip, preset.Ports)
		cancel()
		a.dispatch(func() {
			a.portScanning = false
			if err != nil {
				a.portErr = "端口扫描没跑完，请稍后再试"
				return
			}
			a.ports[ip] = ports
		})
	}()
}

// sourceAddr is the address TCP probes bind to: the selected adapter's,
// so a VPN's TUN adapter cannot take the LAN's connections and answer for
// devices that are not there (see scan.ScanPortsFrom).
func (a *app) sourceAddr() netip.Addr { return a.currentNIC().IP }

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
	// Every comparator ends in the IP, so rows keep one order frame to
	// frame no matter how the map iterates; ties must never flip or the
	// table jumps under a resting pointer. Descending negates instead of
	// reversing, keeping that order.
	desc := 1
	if so.Descending {
		desc = -1
	}
	var cmp func(x, y model.Host) int
	switch so.Column {
	case "state":
		cmp = func(x, y model.Host) int { return desc * (int(x.State) - int(y.State)) }
	case "vendor":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.Vendor, y.Vendor) }
	case "mac":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.MAC, y.MAC) }
	case "hostname":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.Hostname, y.Hostname) }
	default:
		cmp = func(x, y model.Host) int { return desc * x.IP.Compare(y.IP) }
	}
	slices.SortFunc(hs, func(x, y model.Host) int {
		if c := cmp(x, y); c != 0 {
			return c
		}
		return x.IP.Compare(y.IP)
	})
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

// discoveredCount counts hosts the sweep found responding; the offline
// backfill of #21 does not count, the status bar shows it under 总数.
func (a *app) discoveredCount() int {
	n := 0
	for _, h := range a.hosts {
		if h.State != model.StateOffline {
			n++
		}
	}
	return n
}
