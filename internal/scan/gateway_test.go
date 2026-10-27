package scan

import "testing"

// TestGatewayLive: GetAdaptersAddresses is asked for the gateways with the
// right flag. It read them never while the flag said WINS info, and an
// interface with a default route always reports one.
func TestGatewayLive(t *testing.T) {
	nics, err := ListNICs()
	if err != nil {
		t.Fatal(err)
	}
	if len(nics) == 0 {
		t.Skip("no NIC on this machine")
	}
	found := false
	for _, n := range nics {
		if n.Gateway.IsValid() {
			found = true
			if !n.Gateway.Is4() {
				t.Errorf("%s: gateway %v is not IPv4", n.Name, n.Gateway)
			}
		}
	}
	if !found {
		t.Error("no NIC reported a gateway; check gaaIncludeGateways")
	}
}
