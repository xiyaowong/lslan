//go:build windows

package scan

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"

	"lslan/internal/model"
)

const (
	mdnsGroup    = "224.0.0.251"
	mdnsPort     = 5353
	mdnsMetaName = "_services._dns-sd._udp.local."
	mdnsMaxTypes = 24
	// probeGap paces one host's query burst from the next.
	probeGap = 50 * time.Millisecond
	// readTick is how long a read waits before the loop re-checks ctx.
	readTick = 300 * time.Millisecond
)

// mdnsServiceTypes are the types asked proactively, on top of the meta
// query whose answer names whatever else the host announces.
var mdnsServiceTypes = []string{
	"_http._tcp", "_device-info._tcp", "_workstation._tcp", "_ipp._tcp",
	"_smb._tcp", "_rtsp._tcp", "_airplay._tcp", "_raop._tcp",
	"_googlecast._tcp", "_hap._tcp", "_spotify-connect._tcp", "_ssh._tcp",
	"_ftp._tcp", "_printer._tcp", "_scanner._tcp", "_afpovertcp._tcp",
}

// mdnsMerge receives one host's mDNS facts; either argument may be
// empty. Called from mdns goroutines.
type mdnsMerge func(ip netip.Addr, hostname string, svcs []model.Service)

// mdnsRunner runs the two mDNS routes until ctx is done: unicast
// queries to hosts the sweep finds — unicast crosses wireless relays
// where multicast may not — and a passive ear on the multicast group.
// Every failure here is silent: losing mDNS must never break the sweep.
type mdnsRunner struct {
	ctx     context.Context
	merge   mdnsMerge
	probeQ  chan netip.Addr
	conn    *net.UDPConn // active route: ephemeral port, queries out
	passive *net.UDPConn

	mu     sync.Mutex
	probed map[uint32]struct{}
	types  map[string]struct{}
	id     atomic.Uint32
}

// mdnsOff reports whether mDNS discovery is turned off, by LSLAN_MDNS=0
// or a value starting with "n": it binds a UDP port and a multicast group
// that a second copy of the app cannot share, so a development run that
// may sit beside another one on the same machine turns it off.
func mdnsOff() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LSLAN_MDNS")))
	return v == "0" || v == "off" || v == "false" || v == "no"
}

// startMDNS launches mDNS discovery for one sweep. It returns a runner
// even when nothing could be started; probe then drops everything.
func startMDNS(ctx context.Context, nic model.NIC, merge mdnsMerge) *mdnsRunner {
	m := &mdnsRunner{
		ctx:    ctx,
		merge:  merge,
		probeQ: make(chan netip.Addr, 128),
		probed: map[uint32]struct{}{},
		types:  map[string]struct{}{},
	}
	if mdnsOff() {
		return m // probes and answers go nowhere
	}
	for _, t := range mdnsServiceTypes {
		m.types[t] = struct{}{}
	}
	if conn, err := net.ListenUDP("udp4", &net.UDPAddr{}); err == nil {
		m.conn = conn
		go m.readLoop(conn, false)
	}
	if conn := listenPassive(nic); conn != nil {
		m.passive = conn
		go m.readLoop(conn, true)
	}
	go m.probeLoop()
	go func() {
		<-ctx.Done()
		if m.conn != nil {
			m.conn.Close()
		}
		if m.passive != nil {
			m.passive.Close()
		}
	}()
	return m
}

// listenPassive binds the multicast group and joins it on the sweep's
// NIC. Go binds multicast with SO_REUSEADDR, so sharing port 5353 with
// the system's own mDNS responder works; any failure just skips the
// passive route.
func listenPassive(nic model.NIC) *net.UDPConn {
	var iface *net.Interface
	if nic.Index > 0 {
		iface, _ = net.InterfaceByIndex(nic.Index)
	}
	conn, err := net.ListenMulticastUDP("udp4", iface,
		&net.UDPAddr{IP: net.ParseIP(mdnsGroup), Port: mdnsPort})
	if err != nil {
		return nil
	}
	return conn
}

// probe queues a unicast query round against ip; best effort, it may
// be dropped when the queue is backed up.
func (m *mdnsRunner) probe(ip netip.Addr) {
	select {
	case m.probeQ <- ip:
	default:
	}
}

// probeLoop drains the probe queue, one query burst per host.
func (m *mdnsRunner) probeLoop() {
	for {
		var ip netip.Addr
		select {
		case <-m.ctx.Done():
			return
		case ip = <-m.probeQ:
		}
		m.mu.Lock()
		key := be32(ip)
		_, seen := m.probed[key]
		m.probed[key] = struct{}{}
		types := make([]string, 0, len(m.types))
		for t := range m.types {
			types = append(types, t)
		}
		m.mu.Unlock()
		if seen {
			continue
		}
		for _, t := range types {
			m.sendQuery(ip, t)
		}
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(probeGap):
		}
	}
}

// sendQuery sends one PTR question to the host. Queries leave from an
// ephemeral port, so responders reply unicast (legacy query).
func (m *mdnsRunner) sendQuery(ip netip.Addr, typ string) {
	if m.conn == nil {
		return
	}
	q := &dns.Msg{
		MsgHdr: dns.MsgHdr{Id: uint16(m.id.Add(1))},
		Question: []dns.Question{
			{Name: dns.Fqdn(typ + ".local"), Qtype: dns.TypePTR, Qclass: dns.ClassINET},
		},
	}
	pkt, err := q.Pack()
	if err != nil {
		return
	}
	_, _ = m.conn.WriteToUDP(pkt, &net.UDPAddr{IP: ip.AsSlice(), Port: mdnsPort})
}

// readLoop reads one socket until ctx cancellation closes it.
func (m *mdnsRunner) readLoop(conn *net.UDPConn, passive bool) {
	buf := make([]byte, 9000)
	for {
		if m.ctx.Err() != nil {
			return
		}
		conn.SetReadDeadline(time.Now().Add(readTick))
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		if passive && raddr.Port != mdnsPort {
			continue
		}
		ip, ok := netip.AddrFromSlice(raddr.IP)
		if !ok || !ip.Is4() {
			continue
		}
		m.handle(ip.Unmap(), buf[:n])
	}
}

// handle folds one response packet into facts for its source.
func (m *mdnsRunner) handle(src netip.Addr, pkt []byte) {
	var msg dns.Msg
	if msg.Unpack(pkt) != nil {
		return
	}
	var ptrs []*dns.PTR
	var srvs []*dns.SRV
	for _, rr := range msg.Answer {
		switch r := rr.(type) {
		case *dns.PTR:
			ptrs = append(ptrs, r)
		case *dns.SRV:
			srvs = append(srvs, r)
		}
	}
	if len(ptrs) == 0 && len(srvs) == 0 {
		return
	}
	for _, p := range ptrs {
		if strings.EqualFold(p.Hdr.Name, mdnsMetaName) {
			m.addType(mdnsType(p.Ptr))
		}
	}
	byInstance := make(map[string]*dns.SRV, len(srvs))
	for _, s := range srvs {
		byInstance[strings.ToLower(s.Hdr.Name)] = s
	}
	var svcs []model.Service
	var hostname string
	for _, p := range ptrs {
		if strings.EqualFold(p.Hdr.Name, mdnsMetaName) {
			continue
		}
		s := byInstance[strings.ToLower(p.Ptr)]
		if s == nil {
			continue
		}
		if hostname == "" {
			hostname = strings.TrimSuffix(s.Target, ".")
		}
		svcs = append(svcs, model.Service{
			Name: mdnsType(p.Hdr.Name),
			Port: int(s.Port),
		})
	}
	if hostname == "" && len(srvs) > 0 {
		hostname = strings.TrimSuffix(srvs[0].Target, ".")
	}
	if len(svcs) == 0 && hostname == "" {
		return
	}
	m.merge(src, hostname, svcs)
}

// addType remembers a type learned from a meta answer and asks every
// host probed so far about it.
func (m *mdnsRunner) addType(t string) {
	if !strings.HasSuffix(t, "._tcp") && !strings.HasSuffix(t, "._udp") {
		return
	}
	m.mu.Lock()
	if m.types == nil {
		m.types = map[string]struct{}{}
	}
	if _, ok := m.types[t]; ok || len(m.types) >= mdnsMaxTypes {
		m.mu.Unlock()
		return
	}
	m.types[t] = struct{}{}
	ips := make([]netip.Addr, 0, len(m.probed))
	for k := range m.probed {
		ips = append(ips, addrFromUint32(k))
	}
	m.mu.Unlock()
	for _, ip := range ips {
		m.sendQuery(ip, t)
	}
}

// mdnsType strips a DNS-SD record name to its type, e.g.
// "_http._tcp.local." → "_http._tcp".
func mdnsType(name string) string {
	t := strings.ToLower(strings.TrimSuffix(name, "."))
	return strings.TrimSuffix(t, ".local")
}

// appendService adds s to list unless a same-named one is already
// there, in which case it only fills in a missing port.
func appendService(list []model.Service, s model.Service) []model.Service {
	for i := range list {
		if list[i].Name == s.Name {
			if list[i].Port == 0 && s.Port > 0 {
				list[i].Port = s.Port
			}
			return list
		}
	}
	return append(list, s)
}
