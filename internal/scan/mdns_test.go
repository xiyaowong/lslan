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
		Response: true,
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
	r := &mdnsRunner{ctx: context.Background(), merge: func(ip netip.Addr, host, kind string, svcs []model.Service) {
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

// TestMDNSHandleTXT: a device names itself in TXT records, and a TV or a
// printer's model is the only way to tell two identical rows apart. The
// records may sit in Answer or in Additional, and the model is spelled
// differently per vendor.
func TestMDNSHandleTXT(t *testing.T) {
	cases := []struct {
		name string
		rrs  []dns.RR
		want string
	}{
		{
			name: "apple model in answer",
			rrs: []dns.RR{
				&dns.PTR{Hdr: dns.RR_Header{Name: "_device-info._tcp.local.", Rrtype: dns.TypePTR, Class: dns.ClassINET},
					Ptr: "Mac._device-info._tcp.local."},
				&dns.SRV{Hdr: dns.RR_Header{Name: "Mac._device-info._tcp.local.", Rrtype: dns.TypeSRV, Class: dns.ClassINET},
					Port: 0, Target: "mac.local."},
				&dns.TXT{Hdr: dns.RR_Header{Name: "Mac._device-info._tcp.local.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
					Txt: []string{"model=MacBookPro18,3"}},
			},
			want: "MacBookPro18,3",
		},
		{
			name: "printer model in additional",
			rrs: []dns.RR{
				&dns.PTR{Hdr: dns.RR_Header{Name: "_ipp._tcp.local.", Rrtype: dns.TypePTR, Class: dns.ClassINET},
					Ptr: "Office._ipp._tcp.local."},
				&dns.SRV{Hdr: dns.RR_Header{Name: "Office._ipp._tcp.local.", Rrtype: dns.TypeSRV, Class: dns.ClassINET},
					Port: 631, Target: "office.local."},
				&dns.TXT{Hdr: dns.RR_Header{Name: "Office._ipp._tcp.local.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
					Txt: []string{"txtvers=1", "md=OfficeJet Pro 9010"}},
			},
			want: "OfficeJet Pro 9010",
		},
		{
			name: "no txt at all",
			rrs: []dns.RR{
				&dns.PTR{Hdr: dns.RR_Header{Name: "_http._tcp.local.", Rrtype: dns.TypePTR, Class: dns.ClassINET},
					Ptr: "Box._http._tcp.local."},
				&dns.SRV{Hdr: dns.RR_Header{Name: "Box._http._tcp.local.", Rrtype: dns.TypeSRV, Class: dns.ClassINET},
					Port: 80, Target: "box.local."},
			},
			want: "",
		},
	}
	for _, c := range cases {
		// The last record goes to Additional, the rest to Answer.
		answer, extra := c.rrs, []dns.RR(nil)
		if len(c.rrs) == 3 {
			answer, extra = c.rrs[:2], c.rrs[2:]
		}
		m := &dns.Msg{Response: true, Answer: answer, Extra: extra}
		pkt, err := m.Pack()
		if err != nil {
			t.Fatal(err)
		}
		var got string
		r := &mdnsRunner{ctx: context.Background(),
			merge: func(_ netip.Addr, _, kind string, _ []model.Service) { got = kind }}
		r.handle(netip.MustParseAddr("192.168.1.5"), pkt)
		if got != c.want {
			t.Errorf("%s: model = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMDNSHandleMetaAddsType(t *testing.T) {
	m := &dns.Msg{
		Response: true,
		Answer: []dns.RR{
			&dns.PTR{Hdr: dns.RR_Header{Name: mdnsMetaName, Rrtype: dns.TypePTR, Class: dns.ClassINET},
				Ptr: "_xbox._tcp.local."},
		},
	}
	pkt, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	r := &mdnsRunner{ctx: context.Background(), merge: func(netip.Addr, string, string, []model.Service) {}}
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
	r := &mdnsRunner{ctx: context.Background(), merge: func(netip.Addr, string, string, []model.Service) {}}
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
	r.merge = func(netip.Addr, string, string, []model.Service) { called = true }
	r.handle(src, pkt)
	if called {
		t.Fatal("query packet was treated as facts")
	}
}
