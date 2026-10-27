//go:build windows

package scan

import (
	"context"
	"net/netip"
	"testing"

	"github.com/miekg/dns"

	"lslan/internal/model"
)

func TestAppendServiceDedupes(t *testing.T) {
	list := appendService(nil, model.Service{Name: "_http._tcp", Port: 8080})
	list = appendService(list, model.Service{Name: "_http._tcp", Port: 80})
	if len(list) != 1 || list[0].Port != 80 {
		t.Fatalf("duplicate kept or port not updated: %+v", list)
	}
	// An answer without a port never blanks a known one.
	list = appendService(list, model.Service{Name: "_http._tcp"})
	if len(list) != 1 || list[0].Port != 80 {
		t.Fatalf("a missing port clobbered the known one: %+v", list)
	}
	list = appendService(list, model.Service{Name: "_rtsp._tcp", Port: 0})
	list = appendService(list, model.Service{Name: "_rtsp._tcp", Port: 554})
	if len(list) != 2 || list[1].Port != 554 {
		t.Fatalf("port not filled in: %+v", list)
	}
}

// handle is the whole merge path: response packet in, facts out.
func TestMDNSHandleMerges(t *testing.T) {
	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{Response: true},
		Answer: []dns.RR{
			&dns.PTR{Hdr: dns.RR_Header{Name: "_http._tcp.local.", Rrtype: dns.TypePTR, Class: dns.ClassINET},
				Ptr: "MyBox._http._tcp.local."},
			&dns.SRV{Hdr: dns.RR_Header{Name: "MyBox._http._tcp.local.", Rrtype: dns.TypeSRV, Class: dns.ClassINET},
				Port: 8080, Target: "mybox.local."},
		},
	}
	pkt, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ip   netip.Addr
		host string
		svcs []model.Service
	}
	r := &mdnsRunner{ctx: context.Background(), merge: func(ip netip.Addr, host string, svcs []model.Service) {
		got.ip, got.host, got.svcs = ip, host, svcs
	}}
	src := netip.MustParseAddr("192.168.1.5")
	r.handle(src, pkt)
	if got.ip != src || got.host != "mybox.local" {
		t.Fatalf("merge got ip=%s host=%q", got.ip, got.host)
	}
	if len(got.svcs) != 1 || got.svcs[0].Name != "_http._tcp" || got.svcs[0].Port != 8080 {
		t.Fatalf("services: %+v", got.svcs)
	}
}

func TestMDNSHandleMetaAddsType(t *testing.T) {
	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{Response: true},
		Answer: []dns.RR{
			&dns.PTR{Hdr: dns.RR_Header{Name: mdnsMetaName, Rrtype: dns.TypePTR, Class: dns.ClassINET},
				Ptr: "_xbox._tcp.local."},
		},
	}
	pkt, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	r := &mdnsRunner{ctx: context.Background(), merge: func(netip.Addr, string, []model.Service) {}}
	r.handle(netip.MustParseAddr("192.168.1.5"), pkt)
	r.mu.Lock()
	_, ok := r.types["_xbox._tcp"]
	r.mu.Unlock()
	if !ok {
		t.Fatal("meta type not registered")
	}
}

func TestMDNSHandleRejectsJunk(t *testing.T) {
	src := netip.MustParseAddr("1.2.3.4")
	r := &mdnsRunner{ctx: context.Background(), merge: func(netip.Addr, string, []model.Service) {}}
	for _, pkt := range [][]byte{nil, {1, 2, 3}} {
		r.handle(src, pkt)
	}
	// A query (not a response) must yield nothing.
	m := &dns.Msg{Question: []dns.Question{{Name: "_http._tcp.local.", Qtype: dns.TypePTR}}}
	pkt, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	called := false
	r.merge = func(netip.Addr, string, []model.Service) { called = true }
	r.handle(src, pkt)
	if called {
		t.Fatal("query packet was treated as facts")
	}
}
