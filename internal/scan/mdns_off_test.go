//go:build windows

package scan

import "testing"

// TestMDNSOff: the switch that keeps a development run from binding the
// mDNS port another copy holds.
func TestMDNSOff(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"1":     false,
		"on":    false,
		"0":     true,
		"off":   true,
		"false": true,
		"NO":    true,
		" 0 ":   true,
	}
	for v, want := range cases {
		t.Setenv("LSLAN_MDNS", v)
		if got := mdnsOff(); got != want {
			t.Errorf("LSLAN_MDNS=%q: mdnsOff = %v, want %v", v, got, want)
		}
	}
}
