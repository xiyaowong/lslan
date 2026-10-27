package scan

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestDialFromBindsSource: a probe made from a given adapter address has
// that address as its source, so a tunnel adapter cannot take the
// connection and answer for a device that is not there.
func TestDialFromBindsSource(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	src := netip.MustParseAddr("127.0.0.1")
	c, err := dialFrom(t.Context(), src, src, port, time.Second)
	if err != nil {
		t.Fatalf("dial from loopback: %v", err)
	}
	defer c.Close()
	got, _, _ := net.SplitHostPort(c.LocalAddr().String())
	if got != "127.0.0.1" {
		t.Errorf("source = %s, want 127.0.0.1", got)
	}

	// An address that is not a local one fails the dial rather than
	// falling back to the default source.
	if _, err := dialFrom(t.Context(), netip.MustParseAddr("192.0.2.1"), src, port, time.Second); err == nil {
		t.Error("a dial from a foreign address succeeded")
	}
}

// TestScanPortsFromNoSourceFallsBack: an invalid source, as when no
// adapter is chosen, leaves the dial as ScanPorts makes it.
func TestScanPortsFromNoSourceFallsBack(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	got, err := ScanPortsFrom(context.Background(), netip.Addr{}, netip.MustParseAddr("127.0.0.1"), []int{port})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Open {
		t.Errorf("listening port reads %+v, want open", got)
	}
}
