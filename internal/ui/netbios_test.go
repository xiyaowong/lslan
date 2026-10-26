package ui

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

const netbiosTable = `本地连接:
Node IpAddress: [192.168.1.9] Scope Id: []

           NetBIOS Name Table
       Name               Type         Status
    --------------------------------------------
    MYPRINTER      <00>  UNIQUE      Registered
    WORKGROUP      <00>  GROUP       Registered
    MYPRINTER      <20>  UNIQUE      Registered

    MAC Address = 00-11-22-33-44-55
`

// TestNetBIOSName picks the machine's own <00> UNIQUE record, not the
// group's or another type's.
func TestNetBIOSName(t *testing.T) {
	if got := netbiosName(netbiosTable); got != "MYPRINTER" {
		t.Errorf("netbiosName = %q, want MYPRINTER", got)
	}
	if got := netbiosName("    Host not found.\n"); got != "" {
		t.Errorf("netbiosName of a miss = %q, want empty", got)
	}
	// A padded name keeps its inner spacing but loses the edges.
	if got := netbiosName("    HP  LASER  <00>  UNIQUE      Registered\n"); got != "HP  LASER" {
		t.Errorf("padded name = %q", got)
	}
}

// TestNetBIOSLookup: the answer fills a missing hostname, never an
// existing one; a miss leaves the facts alone.
func TestNetBIOSLookup(t *testing.T) {
	host := ip("192.168.1.9")
	a := newApp()
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	a.netbios.starter = func(context.Context, netip.Addr) (string, error) {
		return netbiosTable, nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host

	tt.Frame()
	if err := tt.Click("NetBIOS 问名"); err != nil {
		t.Fatalf("ask button missing: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for a.netbios.busy && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	if got := a.hosts[host].Hostname; got != "MYPRINTER" {
		t.Errorf("hostname = %q, want MYPRINTER", got)
	}

	// The button is gone once the name is there, and an existing name
	// never gets overwritten.
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline, Hostname: "other"}
	tt.Frame()
	if err := tt.Click("NetBIOS 问名"); err == nil {
		t.Error("ask button still up with a hostname present")
	}
}

// TestNetBIOSMiss: a device without a name table says nothing and the
// facts stay as they were.
func TestNetBIOSMiss(t *testing.T) {
	host := ip("192.168.1.9")
	a := newApp()
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	a.netbios.starter = func(context.Context, netip.Addr) (string, error) {
		return strings.Repeat("Host not found.", 2), nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host

	tt.Frame()
	if err := tt.Click("NetBIOS 问名"); err != nil {
		t.Fatalf("ask button missing: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for a.netbios.busy && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	if got := a.hosts[host].Hostname; got != "" {
		t.Errorf("hostname = %q after a miss, want empty", got)
	}
	// The ask is available again for another try.
	tt.Frame()
	if err := tt.Click("NetBIOS 问名"); err != nil {
		t.Error("ask button not offered again after a miss")
	}
}
