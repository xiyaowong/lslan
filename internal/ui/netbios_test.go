package ui

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// nbnsReply builds a Node Status Response holding the named entries:
// 15 name bytes, two type/flag bytes each.
func nbnsReply(names ...struct {
	name  string
	flags uint16
}) []byte {
	var p []byte
	p = binary.BigEndian.AppendUint16(p, 0x1337) // tid
	p = binary.BigEndian.AppendUint16(p, 0x8400) // response, authoritative
	p = binary.BigEndian.AppendUint16(p, 0)      // qdcount
	p = binary.BigEndian.AppendUint16(p, uint16(len(names)))
	p = binary.BigEndian.AppendUint16(p, 0) // nscount
	p = binary.BigEndian.AppendUint16(p, 0) // arcount
	// The question echo: 32 encoded name bytes, then null, type, class.
	p = append(p, make([]byte, 32+4)...)
	// The RR: a name pointer, type NBSTAT, class IN, ttl, rdlength.
	p = binary.BigEndian.AppendUint16(p, 0xC00C)
	p = binary.BigEndian.AppendUint16(p, 0x0021)
	p = binary.BigEndian.AppendUint16(p, 0x0001)
	p = binary.BigEndian.AppendUint32(p, 0) // ttl
	rdAt := len(p)
	p = binary.BigEndian.AppendUint16(p, 0)
	rd := 1 + 18*len(names)
	p = append(p, byte(len(names))) // the count opens the table
	for _, n := range names {
		var raw [15]byte
		copy(raw[:], n.name)
		p = append(p, raw[:]...)
		p = append(p, 0x00) // type <00>
		p = binary.BigEndian.AppendUint16(p, n.flags)
	}
	p[rdAt], p[rdAt+1] = byte(rd>>8), byte(rd)
	return p
}

// TestNBNSReplyName picks the machine's <00> UNIQUE entry, skipping
// group names and the browse master.
func TestNBNSReplyName(t *testing.T) {
	unique := nbnsReply(
		struct {
			name  string
			flags uint16
		}{"MYPRINTER", 0x0400}, // a registered unique name
		struct {
			name  string
			flags uint16
		}{"WORKGROUP", 0x8400}, // a group name
	)
	if got, ok := nbnsReplyName(unique); !ok || got != "MYPRINTER" {
		t.Errorf("nbnsReplyName = %q,%v, want MYPRINTER", got, ok)
	}
	// Only a group: no name to pick.
	group := nbnsReply(struct {
		name  string
		flags uint16
	}{"WORKGROUP", 0x8400})
	if _, ok := nbnsReplyName(group); ok {
		t.Error("a group-only table gave a name")
	}
	// A short or negative packet parses to nothing.
	if _, ok := nbnsReplyName([]byte{0x13, 0x37}); ok {
		t.Error("a stub packet gave a name")
	}
}

// TestNBNSQueryShape checks the wire format of the question: the
// wildcard name encoded, the NBSTAT type, one question.
func TestNBNSQueryShape(t *testing.T) {
	q := nbnsQuery()
	if len(q) != 50 {
		t.Fatalf("query is %d bytes, want 50", len(q))
	}
	if qd := binary.BigEndian.Uint16(q[4:6]); qd != 1 {
		t.Errorf("qdcount = %d, want 1", qd)
	}
	if typ := binary.BigEndian.Uint16(q[46:48]); typ != 0x21 {
		t.Errorf("qtype = %#x, want NBSTAT 0x21", typ)
	}
	// The encoded name starts with the two letters of '*'.
	if q[13] != 'A' || q[14] != 'K' { // 0x2A = 'A'<<4 | 'A' -> "AK"
		t.Errorf("name encoding starts %q, want AK", q[13:15])
	}
}

// TestNetBIOSAutoAsk: opening a device's details asks by itself, the
// name landing in its own fact row; returning does not ask again.
func TestNetBIOSAutoAsk(t *testing.T) {
	host := ip("192.168.1.9")
	a := newApp()
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	calls := 0
	a.netbios.starter = func(context.Context, netip.Addr) (string, error) {
		calls++
		return "MYPRINTER", nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host
	tt.Frame() // the details open, which asks by itself

	deadline := time.Now().Add(time.Second)
	for calls == 0 && time.Now().Before(deadline) {
		a.runPending()
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	if got := a.netbios.names[host]; got != "MYPRINTER" {
		t.Fatalf("name = %q after the ask, want MYPRINTER", got)
	}
	tt.Frame()
	if !tt.HasText("MYPRINTER") {
		t.Errorf("name not shown in the facts, texts %q", tt.Texts())
	}

	// Returning to the device does not ask again.
	a.selRow, a.selIP = -1, netip.Addr{}
	tt.Frame()
	a.selRow, a.selIP = 0, host
	tt.Frame()
	a.runPending()
	if calls != 1 {
		t.Errorf("asked %d times, want 1", calls)
	}
}

// TestNetBIOSNameStaysWithItsDevice: an answer belongs to the address it
// was asked for. A slow answer for one device must never show under
// another.
func TestNetBIOSNameStaysWithItsDevice(t *testing.T) {
	hostA, hostB := ip("192.168.1.9"), ip("192.168.1.10")
	a := newApp()
	a.hosts[hostA] = model.Host{IP: hostA, State: model.StateOnline}
	a.hosts[hostB] = model.Host{IP: hostB, State: model.StateOnline}
	a.netbios.starter = func(_ context.Context, ip netip.Addr) (string, error) {
		if ip == hostB {
			return "DEVICE-B", nil
		}
		return "DEVICE-A", nil
	}
	tt := ui.NewTester(a.View, 1040, 720)

	ask := func(i int, host netip.Addr) {
		t.Helper()
		a.selRow, a.selIP = i, host
		tt.Frame()
		deadline := time.Now().Add(time.Second)
		for a.netbios.names[host] == "" && time.Now().Before(deadline) {
			a.runPending()
			time.Sleep(time.Millisecond)
		}
		a.runPending()
		tt.Frame()
	}
	ask(0, hostA)
	ask(1, hostB)

	// Back on A, its own name shows — not B's.
	a.selRow, a.selIP = 0, hostA
	tt.Frame()
	if !tt.HasText("DEVICE-A") || tt.HasText("DEVICE-B") {
		t.Errorf("A's details show %q, want DEVICE-A", tt.Texts())
	}
}

// TestNetBIOSMissShowsDash: a silent device leaves the dash; no words
// about asking or failing.
func TestNetBIOSMissShowsDash(t *testing.T) {
	host := ip("192.168.1.9")
	a := newApp()
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	asked := make(chan struct{})
	a.netbios.starter = func(context.Context, netip.Addr) (string, error) {
		close(asked)
		return "", nil
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.selRow, a.selIP = 0, host
	tt.Frame()

	select {
	case <-asked:
	case <-time.After(time.Second):
		t.Fatal("the ask never ran")
	}
	a.runPending()
	tt.Frame()
	if !tt.HasText("NetBIOS 名") {
		t.Fatal("fact row missing")
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "—") {
		t.Errorf("no dash shown, texts %q", tt.Texts())
	}
}
