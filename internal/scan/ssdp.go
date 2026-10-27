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
	"strconv"
	"strings"
	"sync"
	"time"

	"lslan/internal/model"
)

// SSDP is where a device says what it is in words: a TV names its model,
// a router its product, a printer its family. mDNS only covers devices
// that speak it — Apple, Chromecast, printers — while most routers, TVs
// and NAS boxes answer UPnP and nothing else.
const (
	ssdpGroup  = "239.255.255.250:1900"
	ssdpSearch = "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: ssdp:all\r\n\r\n"
	ssdpWait     = 2 * time.Second
	ssdpQuiet    = 400 * time.Millisecond // stop after this long with nothing new
	ssdpFetchMax = 8                      // descriptions fetched at once
	ssdpDocLimit = 64 << 10
)

// ssdpDevice is what one device's description adds.
type ssdpDevice struct {
	Model  string // modelName, else modelNumber
	Vendor string // manufacturer, when the ARP table had no OUI name
	Type   string // the UPnP device type, e.g. "MediaRenderer"
}

// ssdpDiscover asks every device on the network what it is, and reports
// each description as it arrives. Unicast to each host already found is
// cheaper and surer than relying on the multicast reply, so it does both:
// one multicast M-SEARCH, then one per host.
//
// Every failure is silent: a network without UPnP, or a device that
// answers but has no description, must not break the sweep.
func ssdpDiscover(ctx context.Context, nic model.NIC, hosts []netip.Addr,
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

	send := func(dst *net.UDPAddr) {
		_, _ = conn.WriteToUDP([]byte(ssdpSearch), dst)
	}
	if g, err := net.ResolveUDPAddr("udp4", group); err == nil {
		send(g)
	}
	for _, ip := range hosts {
		if ctx.Err() != nil {
			return
		}
		// The port comes from the group so one call decides both.
		if g, err := net.ResolveUDPAddr("udp4", group); err == nil {
			send(&net.UDPAddr{IP: ip.AsSlice(), Port: g.Port})
		}
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
	for _, line := range strings.Split(reply, "\n") {
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
	DeviceType   string `xml:"deviceType"`
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
func parseDescription(r io.Reader) (ssdpDevice, bool) {
	var root struct {
		Device upnpDevice `xml:"device"`
	}
	if err := xml.NewDecoder(bufio.NewReader(r)).Decode(&root); err != nil {
		return ssdpDevice{}, false
	}
	d := root.Device
	model := strings.TrimSpace(d.ModelName)
	if model == "" {
		model = strings.TrimSpace(d.ModelNumber)
	}
	if model == "" {
		model = strings.TrimSpace(d.ModelDesc)
	}
	return ssdpDevice{
		Model:  model,
		Vendor: strings.TrimSpace(d.Manufacturer),
		Type:   upnpShortType(d.DeviceType),
	}, true
}

// upnpShortType turns "urn:schemas-upnp-org:device:MediaRenderer:1" into
// "MediaRenderer".
func upnpShortType(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) < 4 {
		return ""
	}
	// Drop the version suffix when there is one.
	name := parts[len(parts)-1]
	if _, err := strconv.Atoi(name); err == nil && len(parts) >= 5 {
		name = parts[len(parts)-2]
	}
	return name
}
