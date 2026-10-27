// Command lslan is a Windows LAN IP scanner desktop app built on mygo.
package ui

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
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
	// lastSeen holds the MACs of the previous completed sweep, for the
	// note that says who came and who went; compared says that sweep has
	// happened, so the first one stays quiet about changes.
	lastSeen map[string]bool
	compared bool
	progress model.Progress
	hosts    map[netip.Addr]model.Host
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
	// search narrows the table to the devices it matches; "" shows all.
	search string
	selRow int
	selIP  netip.Addr

	// cardWidth is the host card's width in DIPs while the detail panel
	// shows; the panel takes the rest, so a wide window widens the details
	// rather than the table.
	cardWidth float32

	// Ping state; the run's generation drops samples of a stopped session.
	// pingTerm is the terminal plugin view of the session, nil when it
	// could not be created (the plain line list stands in).
	ping      toolRun
	pingLines []string
	pingStats pingStats
	pingList  ui.ListState
	pingTerm  pingTerminalIface

	// Telnet test state (#54): one interactive session at a time, shared
	// with ping — starting either ends the other, and the pane at the
	// panel's foot shows the one running for this host. The run's
	// generation drops a dial the user has since ended.
	telnet        toolRun
	telnetPending bool
	telnetPort    int
	telnetTerm    telnetTermIface
	telnetErr     string

	// Route trace state: one run at a time, its process killed on stop.
	// The pane shows the hops as they land and the ✕ once the run ends.
	trace        toolRun
	traceLines   []string
	traceList    ui.ListState
	traceStarter traceStarter

	// netbios is the one-at-a-time name query for a device whose
	// hostname is missing; the answer fills the facts, never overwrites.
	netbios netbiosState

	// diagnose is the 查一查 tool: ping, then ports, then a sentence.
	diagnose diagnoseState
	// diagnosePingFn is one echo; a fake in tests.
	diagnosePingFn func(ctx context.Context, ip netip.Addr) bool

	// wake is the 远程唤醒 tool: a dialog off the status bar, then the
	// wait for the woken device to answer.
	wake     wakeState
	wakeNote string

	// Port scan state.
	presetCache  []scan.Preset
	portScanning bool
	portIP       netip.Addr
	portErr      string
	// portWanted is the host the user asked to scan while another was
	// being scanned; it starts when that one ends.
	portWanted       netip.Addr
	portWantedPreset int
	portScroll       ui.ScrollState
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
		diagnosePingFn: func(ctx context.Context, ip netip.Addr) bool {
			sess, err := scan.StartPing(ctx, ip)
			if err != nil {
				return false
			}
			defer sess.Cancel()
			select {
			case s, ok := <-sess.Samples():
				return ok && s.OK
			case <-ctx.Done():
				return false
			}
		},
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

// setDark shows the given appearance; the terminals paint themselves, so
// they are told apart from the theme.
func (a *app) setDark(dark bool) {
	a.dark = dark
	if a.pingTerm != nil {
		a.pingTerm.setDark(dark)
	}
	if a.telnetTerm != nil {
		a.telnetTerm.setDark(dark)
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
