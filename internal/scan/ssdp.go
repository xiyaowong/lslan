//go:build windows

package scan

import (
	"bufio"
	"context"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// SSDP is where a device says what it is in words: a TV names its model,
// a router its product, a printer its family. mDNS only covers devices
// that speak it — Apple, Chromecast, printers — while most routers, TVs
// and NAS boxes answer UPnP and nothing else. For a domestic box, whose
// model is an internal code, this is often the only name it gives at all.
const (
	ssdpGroup = "239.255.255.250:1900"
	// A search is this header with the search type appended.
	ssdpHeader = "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: "
	ssdpWait     = 2 * time.Second
	ssdpQuiet    = 400 * time.Millisecond // stop after this long with nothing new
	ssdpFetchMax = 8                      // descriptions fetched at once
	ssdpDocLimit = 64 << 10
)

// ssdpSearches are the search types asked for. ssdp:all is what every
// device must answer, but a few only answer the type they are, so the
// root device and the gateway type — the two that name a router — are
// asked for too. Answers are unicast back to the search's source, so
// these cost two more multicast packets and no more.
var ssdpSearches = []string{
	"ssdp:all",
	"upnp:rootdevice",
	"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
}

// ssdpDevice is what one device's description adds.
type ssdpDevice struct {
	Model  string // the product name the device gives, in that order of trust
	Vendor string // manufacturer, when the ARP table had no OUI name
}

// ssdpDiscover asks every device on the network what it is, and reports
// each description as it arrives. Unicast to each host already found is
// cheaper and surer than relying on the multicast reply, so it does both:
// the searches multicast once each, then one per host.
//
// Every failure is silent: a network without UPnP, or a device that
// answers but has no description, must not break the sweep.
func ssdpDiscover(ctx context.Context, hosts []netip.Addr,
	report func(ip netip.Addr, d ssdpDevice)) {
	ssdpDiscoverAt(ctx, ssdpGroup, hosts, report)
}

// ssdpDiscoverAt is ssdpDiscover with the multicast group to search, so a
// test can put the device where no real one listens.
func ssdpDiscoverAt(ctx context.Context, group string, hosts []netip.Addr,
	report func(ip netip.Addr, d ssdpDevice)) {

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	defer conn.Close()

	send := func(dst *net.UDPAddr, st string) {
		_, _ = conn.WriteToUDP([]byte(ssdpHeader+st+"\r\n\r\n"), dst)
	}
	g, err := net.ResolveUDPAddr("udp4", group)
	if err != nil {
		return
	}
	// Every search type goes to the group — three multicast packets in
	// all — while the per-host unicast asks only the catch-all type: a
	// network of 254 addresses would otherwise carry three times as many
	// packets to find the handful of devices that ignore ssdp:all.
	for _, st := range ssdpSearches {
		send(g, st)
	}
	for _, ip := range hosts {
		if ctx.Err() != nil {
			return
		}
		// The port comes from the group so one call decides both.
		send(&net.UDPAddr{IP: ip.AsSlice(), Port: g.Port}, ssdpSearches[0])
	}

	// Collect the LOCATION of each answer, keyed by that URL: a device
	// answers once per service it offers, all pointing at one document.
	type found struct {
		ip  netip.Addr
		url string
	}
	seen := map[string]bool{}
	var locations []found
	buf := make([]byte, 4096)
	// Read until the network falls quiet or the whole window is spent:
	// devices that answer at all do so within a moment.
	var lastReply time.Time
	deadline := time.Now().Add(ssdpWait)
	for {
		if ctx.Err() != nil {
			break
		}
		now := time.Now()
		if now.After(deadline) {
			break
		}
		if !lastReply.IsZero() && now.Sub(lastReply) > ssdpQuiet {
			break
		}
		_ = conn.SetReadDeadline(minTime(now.Add(ssdpQuiet), deadline))
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			break
		}
		ip, ok := netip.AddrFromSlice(raddr.IP)
		if !ok || !ip.Is4() {
			continue
		}
		loc := ssdpLocation(string(buf[:n]))
		if loc == "" || seen[loc] {
			continue
		}
		seen[loc] = true
		lastReply = now
		locations = append(locations, found{ip.Unmap(), loc})
	}

	// Fetch the descriptions a few at a time: each is a small HTTP GET on
	// the LAN, but a big house can have dozens of devices.
	client := &http.Client{Timeout: time.Second}
	var wg sync.WaitGroup
	sem := make(chan struct{}, ssdpFetchMax)
	for _, f := range locations {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			if d, ok := fetchDescription(ctx, client, f.url); ok {
				report(f.ip, d)
			}
		})
	}
	wg.Wait()
}

// minTime is the earlier of two times.
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// ssdpLocation reads the LOCATION header out of an SSDP reply.
func ssdpLocation(reply string) string {
	for line := range strings.SplitSeq(reply, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(k), "location") {
			continue
		}
		if u := strings.TrimSpace(v); strings.HasPrefix(u, "http://") {
			return u
		}
	}
	return ""
}

// upnpDevice is the part of a UPnP description worth keeping; the
// document nests them, and encoding/xml finds them by name at any depth.
type upnpDevice struct {
	FriendlyName string `xml:"friendlyName"`
	Manufacturer string `xml:"manufacturer"`
	ModelName    string `xml:"modelName"`
	ModelNumber  string `xml:"modelNumber"`
	ModelDesc    string `xml:"modelDescription"`
}

// fetchDescription reads and parses one device description.
func fetchDescription(ctx context.Context, client *http.Client, url string) (ssdpDevice, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ssdpDevice{}, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return ssdpDevice{}, false
	}
	defer resp.Body.Close()
	return parseDescription(io.LimitReader(resp.Body, ssdpDocLimit))
}

// parseDescription reads the device element out of a UPnP description.
// modelName is the model proper; a domestic box often leaves it out and
// puts the product's name in friendlyName instead, which is the one place
// it names itself in words.
func parseDescription(r io.Reader) (ssdpDevice, bool) {
	var root struct {
		Device upnpDevice `xml:"device"`
	}
	if err := xml.NewDecoder(bufio.NewReader(r)).Decode(&root); err != nil {
		return ssdpDevice{}, false
	}
	d := root.Device
	model := d.ModelName
	for _, alt := range []string{d.ModelNumber, d.ModelDesc, d.FriendlyName} {
		if strings.TrimSpace(model) != "" {
			break
		}
		model = alt
	}
	return ssdpDevice{
		Model:  strings.TrimSpace(model),
		Vendor: strings.TrimSpace(d.Manufacturer),
	}, true
}
