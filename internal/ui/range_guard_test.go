package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// --- unit tests of the guard logic ---

func TestDigitsOnly(t *testing.T) {
	cases := map[string]string{
		"123":   "123",
		"1a2.3": "123",
		"中文":    "",
		"":      "",
	}
	for in, want := range cases {
		if got := digitsOnly(in); got != want {
			t.Errorf("digitsOnly(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClampOctet(t *testing.T) {
	cases := map[string]string{
		"1":     "1",
		"25":    "25",
		"255":   "255",
		"256":   "25",
		"999":   "99",
		"1234":  "123",
		"":      "",
		"abc":   "",
		"2a5b9": "25",
		"02a55": "025",
	}
	for in, want := range cases {
		if got := clampOctet(in); got != want {
			t.Errorf("clampOctet(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitRange(t *testing.T) {
	s := netip.MustParseAddr("192.168.1.10")
	e := netip.MustParseAddr("192.168.1.250")
	prefix, sh, eh, ok := splitRange(s, e)
	if !ok || prefix != [3]byte{192, 168, 1} || sh != 10 || eh != 250 {
		t.Errorf("splitRange = %v,%d,%d,%v", prefix, sh, eh, ok)
	}
	if _, _, _, ok := splitRange(s, netip.MustParseAddr("10.0.0.1")); ok {
		t.Error("cross-network range should not split")
	}
	if _, _, _, ok := splitRange(netip.Addr{}, e); ok {
		t.Error("invalid start should not split")
	}
}

func TestCellsRoundTrip(t *testing.T) {
	a := newApp()
	a.setRange(netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254"))
	if !a.cellLock {
		t.Fatalf("cellLock = false, want true")
	}
	start, end, ok := a.fullRange()
	if !ok || start != netip.MustParseAddr("192.168.1.1") || end != netip.MustParseAddr("192.168.1.254") {
		t.Fatalf("fullRange = %s..%s,%v", start, end, ok)
	}

	// Editing host cells shows in fullRange.
	a.ipCells[3], a.ipCells[7] = "20", "30"
	start, end, _ = a.fullRange()
	if start != netip.MustParseAddr("192.168.1.20") || end != netip.MustParseAddr("192.168.1.30") {
		t.Errorf("fullRange = %s..%s", start, end)
	}

	// An empty cell holds the range back until typing finishes.
	a.ipCells[3] = ""
	if _, _, ok := a.fullRange(); ok {
		t.Error("empty cell should fail fullRange")
	}

	// Ends swapped sort themselves out.
	a.ipCells[3], a.ipCells[7] = "100", "5"
	start, end, _ = a.fullRange()
	if start != netip.MustParseAddr("192.168.1.5") || end != netip.MustParseAddr("192.168.1.100") {
		t.Errorf("swapped = %s..%s", start, end)
	}
}

func TestCrossNetworkUnlocks(t *testing.T) {
	a := newApp()
	a.setRange(netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.9.254"))
	if a.cellLock {
		t.Fatal("cellLock = true, want false")
	}
	start, end, ok := a.fullRange()
	if !ok || start.String() != "192.168.1.1" || end.String() != "192.168.9.254" {
		t.Errorf("fullRange = %s..%s,%v", start, end, ok)
	}
}

func TestNarrowRange(t *testing.T) {
	a := newApp()
	a.setRange(netip.MustParseAddr("192.168.5.1"), netip.MustParseAddr("192.168.9.254"))
	a.narrowRange()
	start, end, ok := a.fullRange()
	if !ok || start != netip.MustParseAddr("192.168.5.1") || end != netip.MustParseAddr("192.168.5.254") {
		t.Errorf("after narrow = %s..%s,%v", start, end, ok)
	}
	if a.cellLock != true {
		t.Errorf("cellLock = %v, want true after narrowing", a.cellLock)
	}
}

func TestEditableNeighbors(t *testing.T) {
	a := newApp()
	a.cellLock = true
	if got := a.nextEditable(2); got != 3 {
		t.Errorf("nextEditable(2) = %d", got)
	}
	if got := a.nextEditable(7); got != -1 {
		t.Errorf("nextEditable(7) = %d", got)
	}
	if got := a.prevEditable(3); got != -1 {
		t.Errorf("prevEditable(3) = %d", got)
	}
	if got := a.prevEditable(4); got != 3 {
		t.Errorf("prevEditable(4) = %d", got)
	}
	if got := a.nextEditable(6); got != 7 {
		t.Errorf("nextEditable(6) = %d, want skip to the host cell", got)
	}
	// Unlocked, every cell is editable.
	a.cellLock = false
	if got := a.nextEditable(0); got != 1 {
		t.Errorf("unlocked nextEditable(0) = %d", got)
	}
}

// --- view tests ---

// typeInto focuses cell i and types s the way a keyboard does, one key
// per rune, replacing what the cell held.
func typeInto(t *testing.T, tt *ui.Tester, a *app, i int, s string) {
	t.Helper()
	if a.cellLocked(i) {
		t.Fatalf("cell %d is locked", i)
	}
	if err := tt.Click(cellLabel(i)); err != nil {
		t.Fatalf("focus cell %d: %v", i, err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyDelete)
	tt.Frame()
	for _, r := range s {
		tt.Type(string(r))
	}
	tt.Frame()
}

func cellLabel(i int) string {
	if i < 3 {
		return "IP 第 " + itoa(i+1) + " 段（网段，自动）"
	}
	return "IP 第 " + itoa(i+1) + " 段"
}

func itoa(n int) string { return string(rune('0' + n)) }

// TestRangeBarView drives the segmented control in the tester: the NIC
// locks the network cells, typing digits caps and jumps, Backspace on an
// empty cell goes back, and the buttons work.
func TestRangeBarView(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.0"), netip.MustParseAddr("192.168.1.255")
	}
	tt := ui.NewTester(a.View, 1000, 680)
	a.loadNICs()
	tt.Frame()

	if !a.cellLock {
		t.Fatalf("cellLock = false, want true")
	}
	if a.ipCells[0] != "192" || a.ipCells[3] != "1" || a.ipCells[7] != "254" {
		t.Fatalf("cells = %v", a.ipCells)
	}
	if !tt.HasText("整个网段") {
		t.Fatalf("restore button missing, texts %q", tt.Texts())
	}

	// A locked network cell takes no focus and no input.
	if err := tt.Click(cellLabel(0)); err != nil {
		t.Fatalf("click locked cell: %v", err)
	}
	tt.Frame()
	if tt.Focused(cellLabel(0)) {
		t.Error("locked cell took the focus")
	}
	if a.ipCells[0] != "192" {
		t.Errorf("locked cell changed to %q", a.ipCells[0])
	}

	// Typing past 255 caps the cell digit by digit: 259 becomes 25.
	typeInto(t, tt, a, 3, "259")
	if a.ipCells[3] != "25" {
		t.Errorf("cell 3 = %q, want 25", a.ipCells[3])
	}

	// The third digit of a full cell moves on to the next cell, here the
	// second address's host cell across the locked prefix.
	typeInto(t, tt, a, 3, "255")
	if !tt.Focused(cellLabel(7)) {
		t.Errorf("focus after jump = %v, want cell 8", tt.Focused(cellLabel(7)))
	}

	// Typing into the jumped-to cell caps the same way.
	tt.Type("9")
	tt.Frame()
	if a.ipCells[7] != "254" {
		t.Errorf("cell 7 = %q, want 254", a.ipCells[7])
	}

	// Backspace on an empty cell steps back to the previous editable one.
	typeInto(t, tt, a, 7, "")
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if !tt.Focused(cellLabel(3)) {
		t.Errorf("backspace retreat: focus on cell 4 = %v, want cell 4", tt.Focused(cellLabel(3)))
	}

	// "整个网段" restores the NIC default.
	if err := tt.Click("整个网段"); err != nil {
		t.Fatalf("click restore: %v", err)
	}
	tt.Frame()
	if a.ipCells[0] != "192" || a.ipCells[3] != "1" || a.ipCells[7] != "254" {
		t.Errorf("after restore cells = %v", a.ipCells)
	}

	// A wide range offers the one-click narrowing; clicking trims it and
	// locks the prefix again.
	a.setRange(netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.9.254"))
	tt.Frame()
	if !tt.HasText("收窄到 C 段") {
		t.Errorf("narrow button missing, texts %q", tt.Texts())
	}
	if err := tt.Click("收窄到 C 段"); err != nil {
		t.Fatalf("click narrow: %v", err)
	}
	tt.Frame()
	if a.ipCells[6] != "1" || a.ipCells[7] != "254" {
		t.Errorf("after narrow cells = %v", a.ipCells)
	}
	if !a.cellLock {
		t.Errorf("cellLock = false, want true after narrowing")
	}
}

// TestRangeBarScan pins that 开始扫描 reads the segmented cells: typed host
// numbers reach the scanner, and an incomplete range is refused plainly.
func TestRangeBarScan(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.0"), netip.MustParseAddr("192.168.1.255")
	}
	sc := &fakeScanner{}
	a.newScanner = func() scannerIface { return sc }
	tt := ui.NewTester(a.View, 1000, 680)
	a.loadNICs()
	tt.Frame()

	typeInto(t, tt, a, 3, "30")
	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click scan: %v", err)
	}
	tt.Frame()
	if !sc.started || sc.start != netip.MustParseAddr("192.168.1.30") || sc.end != netip.MustParseAddr("192.168.1.254") {
		t.Errorf("scanner got %s..%s (started %v)", sc.start, sc.end, sc.started)
	}

	// Halfway through retyping, the scan is refused with a hint.
	a.ipCells[7] = ""
	a.scanning = false
	tt.Frame()
	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click scan: %v", err)
	}
	tt.Frame()
	if !tt.HasText("IP 范围无效") {
		t.Errorf("incomplete-range hint missing, texts %q", tt.Texts())
	}
}

// fakeScanner records what the view asked the engine to scan.
type fakeScanner struct {
	started    bool
	start, end netip.Addr
}

func (f *fakeScanner) Events() <-chan scan.Event { return make(chan scan.Event) }

func (f *fakeScanner) Start(nic model.NIC, start, end netip.Addr) error {
	f.started = true
	f.start, f.end = start, end
	return nil
}

func (f *fakeScanner) Cancel() {}
