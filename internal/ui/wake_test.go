package ui

import (
	"net/netip"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// wakeApp is an app with the wake side faked: the send recorded, the ARP
// lookup a set map, the probe answering from a function.
type wakeApp struct {
	*app
	sent   []string
	arp    map[string]string
	probes []netip.Addr
	answer func(netip.Addr) bool
}

func newWakeApp() *wakeApp {
	a := newApp()
	wa := &wakeApp{app: a, arp: map[string]string{}}
	a.wakeSend = func(nic model.NIC, mac string) error {
		wa.sent = append(wa.sent, mac)
		return nil
	}
	a.wakeMac = func(ip netip.Addr) string { return wa.arp[ip.String()] }
	a.wakeProbe = func(src, ip netip.Addr) bool {
		wa.probes = append(wa.probes, ip)
		if wa.answer != nil {
			return wa.answer(ip)
		}
		return false
	}
	a.wakeEvery = time.Millisecond
	a.wakeFor = 50 * time.Millisecond
	return wa
}

// scanableWakeApp adds a NIC and a fake scanner, so the dialog's sweep can
// be counted without touching the network.
func newScanableWakeApp(t *testing.T) (*wakeApp, *int) {
	t.Helper()
	wa := newWakeApp()
	wa.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	wa.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	starts := new(int)
	wa.newScanner = func() scannerIface {
		return &countingScanner{starts: starts}
	}
	return wa, starts
}

// countingScanner counts the sweeps it was asked to run.
type countingScanner struct{ starts *int }

func (s *countingScanner) Events() <-chan scan.Event { return make(chan scan.Event) }

func (s *countingScanner) Start(model.NIC, netip.Addr, netip.Addr) error {
	*s.starts++
	return nil
}

func (s *countingScanner) Cancel() {}

// TestWakeSweepsWhenNeverSwept: opening 远程唤醒 before anything was
// scanned starts a sweep by itself, so there are devices to pick from.
func TestWakeSweepsWhenNeverSwept(t *testing.T) {
	wa, starts := newScanableWakeApp(t)
	tt := ui.NewTester(wa.View, 1040, 720)
	wa.loadNICs()
	tt.Frame()

	if err := tt.Click("远程唤醒"); err != nil {
		t.Fatalf("click the entry: %v", err)
	}
	if *starts != 1 {
		t.Errorf("sweeps = %d, want 1 — the dialog had nothing to offer", *starts)
	}
	if !wa.scanning {
		t.Error("the sweep did not start")
	}
	tt.Frame()
	if !tt.HasText("正在扫描") {
		t.Errorf("the dialog does not say it is scanning, texts %q", tt.Texts())
	}
}

// TestWakeKeepsSweptResults: once a sweep has run, opening the dialog
// keeps its results rather than emptying the table under the user, and
// its button offers the sweep again.
func TestWakeKeepsSweptResults(t *testing.T) {
	wa, starts := newScanableWakeApp(t)
	tt := ui.NewTester(wa.View, 1040, 720)
	wa.loadNICs()
	host := ip("192.168.1.20")
	wa.hosts[host] = model.Host{IP: host, State: model.StateOffline, MAC: "aa:bb:cc:dd:ee:ff"}
	wa.progress = model.Progress{Total: 254, Scanned: 254} // the sweep ran
	tt.Frame()

	if err := tt.Click("远程唤醒"); err != nil {
		t.Fatalf("click the entry: %v", err)
	}
	if *starts != 0 {
		t.Errorf("sweeps = %d, want none — the results were already there", *starts)
	}
	tt.Frame()
	if !tt.HasText("192.168.1.20 离线") {
		t.Errorf("the swept device is not offered, texts %q", tt.Texts())
	}
	if err := tt.Click("重新扫描"); err != nil {
		t.Fatalf("click the sweep button: %v", err)
	}
	if *starts != 1 {
		t.Errorf("sweeps = %d, want 1 from the dialog's button", *starts)
	}
}

// TestWakeEntryIsGlobal: 远程唤醒 sits in the status bar, not in a
// device's details, stays inside the window's least width, and opens the
// dialog with the fields it asks for.
func TestWakeEntryIsGlobal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1040, 720}, {860, 520}} {
		wa := newWakeApp()
		tt := ui.NewTester(wa.View, size.w, size.h)
		tt.Frame()
		r, ok := tt.Find("远程唤醒")
		if !ok {
			t.Fatalf("%dx%d: the entry is missing", size.w, size.h)
		}
		if r.X+r.W > float32(size.w) || r.Y+r.H > float32(size.h) {
			t.Errorf("%dx%d: the entry at %v runs past the window", size.w, size.h, r)
		}
		if err := tt.Click("远程唤醒"); err != nil {
			t.Fatalf("%dx%d: click the entry: %v", size.w, size.h, err)
		}
		if !wa.wake.open {
			t.Fatalf("%dx%d: the entry did not open the dialog", size.w, size.h)
		}
		tt.Frame()
		for _, s := range []string{"设备地址", "MAC 地址", "发送唤醒信号"} {
			if !tt.HasText(s) {
				t.Errorf("%dx%d: %q missing from the dialog, texts %q", size.w, size.h, s, tt.Texts())
			}
		}
	}
}

// TestWakePicksOfflineDevices: the picks are the devices that went
// offline, never the empty addresses the sweep backfilled; with none of
// those, the devices online stand in.
func TestWakePicksOfflineDevices(t *testing.T) {
	wa := newWakeApp()
	offline := ip("192.168.1.20")
	online := ip("192.168.1.5")
	empty := ip("192.168.1.77") // backfilled: nothing was ever there
	wa.hosts[offline] = model.Host{IP: offline, State: model.StateOffline, Hostname: "nas", MAC: "aa:bb:cc:dd:ee:01"}
	wa.hosts[online] = model.Host{IP: online, State: model.StateOnline, MAC: "aa:bb:cc:dd:ee:02"}
	wa.hosts[empty] = model.Host{IP: empty, State: model.StateOffline}
	wa.arp["192.168.1.20"] = "aa:bb:cc:dd:ee:01"
	tt := ui.NewTester(wa.View, 1000, 680)
	wa.openWake()
	tt.Frame()

	if picks := wa.wakePicks(); len(picks) != 1 || picks[0].IP != offline {
		t.Fatalf("picks = %v, want the device gone offline alone", picks)
	}
	if err := tt.Click("192.168.1.20 nas 离线"); err != nil {
		t.Fatalf("click the pick: %v", err)
	}
	if wa.wake.ipText != "192.168.1.20" || wa.wake.mac != "aa:bb:cc:dd:ee:01" {
		t.Errorf("pick filled %q / %q", wa.wake.ipText, wa.wake.mac)
	}

	// Nothing has gone dark: the online devices are still wakable.
	delete(wa.hosts, offline)
	if picks := wa.wakePicks(); len(picks) != 1 || picks[0].IP != online {
		t.Errorf("picks = %v, want the online device", picks)
	}
}

// TestWakeMACFilledFromARP: an address typed by hand gets its MAC from
// the system's ARP table, and a MAC typed over the filled one is kept.
func TestWakeMACFilledFromARP(t *testing.T) {
	wa := newWakeApp()
	wa.arp["192.168.1.30"] = "00:11:22:33:44:55"
	wa.openWake()
	wa.wake.ipText = "192.168.1.30"
	wa.wakeFill()
	if wa.wake.mac != "00:11:22:33:44:55" {
		t.Errorf("MAC not filled from ARP: %q", wa.wake.mac)
	}
	// Another address with nothing known clears the filled one.
	wa.wake.ipText = "192.168.1.31"
	wa.wakeFill()
	if wa.wake.mac != "" {
		t.Errorf("MAC kept across addresses: %q", wa.wake.mac)
	}
	// The user's own MAC outlives the address.
	wa.wake.mac = "aa:bb:cc:dd:ee:ff"
	wa.wake.ipText = "192.168.1.30"
	wa.wakeFill()
	if wa.wake.mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("a typed MAC was overwritten: %q", wa.wake.mac)
	}
}

// TestWakeSendAndWatch: a send closes the dialog, the device answers, and
// its offline row lights up as online.
func TestWakeSendAndWatch(t *testing.T) {
	wa := newWakeApp()
	target := ip("192.168.1.20")
	wa.hosts[target] = model.Host{IP: target, State: model.StateOffline}
	wa.wake.ipText, wa.wake.mac = target.String(), "aa-bb-cc-dd-ee-ff"
	wa.answer = func(netip.Addr) bool { return true }
	tt := ui.NewTester(wa.View, 1000, 680)
	tt.Frame()

	wa.startWake()
	if len(wa.sent) != 1 || wa.sent[0] != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("sent %q, want the normalized MAC", wa.sent)
	}
	if wa.wake.open {
		t.Error("the dialog stayed open after the send")
	}
	deadline := time.Now().Add(time.Second)
	for wa.wake.waiting && time.Now().Before(deadline) {
		wa.runPending()
		time.Sleep(time.Millisecond)
	}
	wa.runPending()
	if h := wa.hosts[target]; h.State != model.StateOnline {
		t.Errorf("the woken device is %v, want online", h.State)
	}
	if len(wa.probes) == 0 || wa.probes[0] != target {
		t.Errorf("probed %v, want %v", wa.probes, target)
	}
	tt.Frame()
	if wa.wakeNote != "" {
		t.Errorf("the note %q was not said", wa.wakeNote)
	}
}

// TestWakeGiveUp: an address that never answers ends the wait, and
// nothing lights up.
func TestWakeGiveUp(t *testing.T) {
	wa := newWakeApp()
	target := ip("192.168.1.21")
	wa.hosts[target] = model.Host{IP: target, State: model.StateOffline}
	wa.wake.ipText, wa.wake.mac = target.String(), "aa:bb:cc:dd:ee:ff"
	tt := ui.NewTester(wa.View, 1000, 680)
	tt.Frame()

	wa.startWake()
	deadline := time.Now().Add(2 * time.Second)
	for wa.wake.waiting && time.Now().Before(deadline) {
		wa.runPending()
		time.Sleep(time.Millisecond)
	}
	wa.runPending()
	if wa.wake.waiting {
		t.Fatal("the wait never ended")
	}
	if h := wa.hosts[target]; h.State != model.StateOffline {
		t.Errorf("a silent device is %v, want still offline", h.State)
	}
}

// TestWakeRefusesBadInput: a malformed address or MAC keeps the dialog up
// and sends nothing, saying what is wrong.
func TestWakeRefusesBadInput(t *testing.T) {
	wa := newWakeApp()
	wa.openWake()
	for _, c := range []struct{ ip, mac string }{
		{"", "aa:bb:cc:dd:ee:ff"},
		{"192.168.1.20", ""},
		{"192.168.1.20", "aa:bb:cc"},
		{"not an ip", "aa:bb:cc:dd:ee:ff"},
	} {
		wa.wake.ipText, wa.wake.mac, wa.wake.errMsg = c.ip, c.mac, ""
		if wa.wakeReady() {
			t.Errorf("%q / %q reads as ready", c.ip, c.mac)
		}
		if wa.wakeProblem() == "" && c.ip != "" {
			t.Errorf("%q / %q gives no reason", c.ip, c.mac)
		}
		wa.startWake()
		if len(wa.sent) != 0 {
			t.Fatalf("%q / %q sent %q", c.ip, c.mac, wa.sent)
		}
		if wa.wake.errMsg == "" {
			t.Errorf("%q / %q gave no reason when tried", c.ip, c.mac)
		}
	}
}

// TestWakeStatusInsideWindow: while a wait runs, the status bar says so
// and everything on it stays within the window's least width.
func TestWakeStatusInsideWindow(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1040, 720}, {860, 520}} {
		wa := newWakeApp()
		wa.listNICs = func() ([]model.NIC, error) {
			return []model.NIC{{Name: "WLAN (192.168.3.205)", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
		}
		wa.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
			return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
		}
		wa.wakeEvery, wa.wakeFor = time.Hour, 2*time.Hour // the wait stays up for the frame
		tt := ui.NewTester(wa.View, size.w, size.h)
		wa.loadNICs()
		target := ip("192.168.3.20")
		wa.hosts[target] = model.Host{IP: target, State: model.StateOffline}
		wa.wake.ipText, wa.wake.mac = target.String(), "aa:bb:cc:dd:ee:ff"
		tt.Frame()

		wa.startWake()
		defer wa.stopWakeWait()
		tt.Frame()
		for _, s := range []string{"正在等 192.168.3.20 上线", "显示离线地址", "远程唤醒", "关于", "在线"} {
			r, ok := tt.Find(s)
			if !ok {
				t.Fatalf("%dx%d: %q missing from the status bar", size.w, size.h, s)
			}
			if r.X+r.W > float32(size.w) || r.Y+r.H > float32(size.h) {
				t.Errorf("%dx%d: %q at %v runs past the window", size.w, size.h, s, r)
			}
		}
	}
}
