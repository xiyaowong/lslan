//go:build windows

package scan

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSSDPDiscoverEndToEnd: a device that answers the search and serves a
// description is reported with its model. The module talks to a local
// fake at 239.255.255.250's place, so nothing leaves the machine.
func TestSSDPDiscoverEndToEnd(t *testing.T) {
	// The description the fake device serves.
	doc := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
    <friendlyName>Living Room TV</friendlyName>
    <manufacturer>Samsung Electronics</manufacturer>
    <modelName>UE55TU8000</modelName>
  </device>
</root>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(doc))
	}))
	defer srv.Close()

	// A fake device: listens on a free UDP port and answers with the
	// description's URL. Port 1900 itself is taken by Windows' own SSDP
	// service, so the search goes to a port of the test's choosing.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	stop := make(chan struct{})
	go func() {
		buf := make([]byte, 2048)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, raddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-stop:
					return
				default:
					continue
				}
			}
			reply := "HTTP/1.1 200 OK\r\n" +
				"CACHE-CONTROL: max-age=1800\r\n" +
				"LOCATION: " + srv.URL + "/desc.xml\r\n" +
				"ST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n\r\n"
			_, _ = conn.WriteToUDP([]byte(reply), raddr)
		}
	}()
	defer close(stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got []ssdpDevice
	group := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ssdpDiscoverAt(ctx, group, []netip.Addr{netip.MustParseAddr("127.0.0.1")},
		func(_ netip.Addr, d ssdpDevice) { got = append(got, d) })

	if len(got) == 0 {
		t.Fatal("the fake device was not reported")
	}
	d := got[0]
	if d.Model != "UE55TU8000" || d.Vendor != "Samsung Electronics" || d.Type != "MediaRenderer" {
		t.Errorf("reported %+v", d)
	}
	if !strings.Contains(srv.URL, "http://") {
		t.Log("the description URL is served over plain HTTP, as devices do")
	}
}
