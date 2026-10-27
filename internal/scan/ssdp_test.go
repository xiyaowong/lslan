//go:build windows

package scan

import (
	"strings"
	"testing"
)

// TestSSDPLocation: the reply is a bare HTTP response; the LOCATION
// header is the only part worth keeping, and header names are not
// case-sensitive.
func TestSSDPLocation(t *testing.T) {
	reply := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=1800\r\n" +
		"location: http://192.168.1.1:5000/rootDesc.xml\r\n" +
		"ST: upnp:rootdevice\r\n\r\n"
	if got := ssdpLocation(reply); got != "http://192.168.1.1:5000/rootDesc.xml" {
		t.Errorf("location = %q", got)
	}
	// A reply with no location, and a junk one, give nothing.
	if got := ssdpLocation("HTTP/1.1 200 OK\r\nST: x\r\n\r\n"); got != "" {
		t.Errorf("a reply without a location gave %q", got)
	}
	if got := ssdpLocation("not a reply at all"); got != "" {
		t.Errorf("junk gave %q", got)
	}
}

// TestUPNPShortType: the device type is a URN; the class is the part
// that says what the device is.
func TestUPNPShortType(t *testing.T) {
	cases := map[string]string{
		"urn:schemas-upnp-org:device:MediaRenderer:1":         "MediaRenderer",
		"urn:schemas-upnp-org:device:InternetGatewayDevice:2": "InternetGatewayDevice",
		"upnp:rootdevice": "",
		"":                "",
	}
	for in, want := range cases {
		if got := upnpShortType(in); got != want {
			t.Errorf("upnpShortType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDescriptionParse: a real description nests the device element; the
// model is what tells two identical rows apart.
func TestDescriptionParse(t *testing.T) {
	doc := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
    <friendlyName>Living Room TV</friendlyName>
    <manufacturer>Samsung Electronics</manufacturer>
    <modelName>UE55TU8000</modelName>
  </device>
</root>`
	d, ok := parseDescription(strings.NewReader(doc))
	if !ok {
		t.Fatal("a valid description did not parse")
	}
	if d.Model != "UE55TU8000" || d.Vendor != "Samsung Electronics" || d.Type != "MediaRenderer" {
		t.Errorf("parsed %+v", d)
	}
}
