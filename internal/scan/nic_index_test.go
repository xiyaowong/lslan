package scan

import (
	"net"
	"testing"
)

// TestListNICsIndexLive: each adapter carries its interface index, which
// is how passive mDNS joins the group on the adapter the user chose. It
// was never set, so `Index > 0` was always false and the listener fell
// back to the default interface — the tunnel, on a machine with a VPN.
func TestListNICsIndexLive(t *testing.T) {
	nics, err := ListNICs()
	if err != nil {
		t.Fatal(err)
	}
	if len(nics) == 0 {
		t.Skip("no NIC on this machine")
	}
	for _, n := range nics {
		if n.Index <= 0 {
			t.Errorf("%s: index %d, want a positive interface index", n.Name, n.Index)
			continue
		}
		iface, err := net.InterfaceByIndex(n.Index)
		if err != nil {
			t.Errorf("%s: index %d does not resolve to an interface: %v", n.Name, n.Index, err)
			continue
		}
		if iface.Name != n.Name {
			t.Errorf("index %d is %q, but the adapter is %q", n.Index, iface.Name, n.Name)
		}
	}
}
