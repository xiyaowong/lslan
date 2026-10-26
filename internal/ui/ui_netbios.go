package ui

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// NBNS over UDP: one Node Status Request to port 137 asks a device for
// its name table straight, where nbtstat spends seconds starting up and
// retrying; a device that has a name answers in tens of milliseconds.

// netbiosStarter asks ip for its NetBIOS name; tests replace it.
type netbiosStarter func(ctx context.Context, ip netip.Addr) (string, error)

// realNetBIOS sends the status request and reads the reply: one query,
// one retry, the whole ask done inside a second on a silent device.
func realNetBIOS(ctx context.Context, ip netip.Addr) (string, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return "", err
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		return "", errors.New("nbns: no deadline on the query")
	}
	_ = conn.SetDeadline(deadline)

	dst := &net.UDPAddr{IP: ip.AsSlice(), Port: nbnsPort}
	for range 2 {
		if _, err := conn.WriteToUDP(nbnsQuery(), dst); err != nil {
			return "", err
		}
		buf := make([]byte, 1500)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				break // this round's wait is over; try again
			}
			if name, ok := nbnsReplyName(buf[:n]); ok {
				return name, nil
			}
			// Not a reply to ours (or a negative one): keep reading
			// within the round.
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(nbnsRetryGap):
		}
	}
	return "", nil // a silent device is a "no name", not an error
}

const (
	nbnsPort     = 137
	nbnsRetryGap = 200 * time.Millisecond
)

// nbnsQuery builds the Node Status Request: a header naming the wildcard
// "*", the name a status request carries, and no resource records.
func nbnsQuery() []byte {
	var p [50]byte
	binary.BigEndian.PutUint16(p[0:2], 0x1337) // transaction id, fixed
	p[5] = 0x01                                // one question, no answers
	// The question name: "*" padded to 15, encoded per RFC 1001
	// first-level encoding, then the length octet and the suffix.
	enc := make([]byte, 32)
	for i, c := range []byte(strings.Repeat(" ", 15)) {
		enc[i*2] = 'A' + c>>4
		enc[i*2+1] = 'A' + c&0x0F
	}
	enc[0], enc[1] = 'A', 'A'+0x0A // '*' = 0x2A
	copy(p[13:], enc)
	p[45] = byte('A')
	p[46], p[47] = 0x00, 0x21 // NBSTAT
	p[48], p[49] = 0x00, 0x01 // IN
	return p[:]
}

// nbnsReplyName reads the device's name out of a Node Status Response:
// the resource record holds a count, then per name 15 raw bytes, a type
// byte, and flags — the machine's own is the <00> UNIQUE one.
func nbnsReplyName(p []byte) (string, bool) {
	if len(p) < 13 {
		return "", false
	}
	flags := binary.BigEndian.Uint16(p[2:4]) & 0x8000
	answers := binary.BigEndian.Uint16(p[6:8])
	if flags == 0 || answers == 0 { // the reply bit set, names present
		return "", false
	}
	// Skip the question section: one encoded name (32 bytes + null +
	// type + class = 36), then the RR: name (compressed to a pointer),
	// type, class, ttl, rdlength, and the names.
	body := p[12:]
	if len(body) < 36+12 {
		return "", false
	}
	body = body[36:]
	if len(body) < 12 {
		return "", false
	}
	rd := binary.BigEndian.Uint16(body[10:12])
	body = body[12:]
	if int(rd) > len(body) || len(body) < 1 {
		return "", false
	}
	count := int(body[0])
	entries := body[1:]
	for ; count > 0 && len(entries) >= 18; count-- {
		name := strings.TrimRight(string(entries[:15]), " \x00")
		entryFlags := binary.BigEndian.Uint16(entries[16:18])
		unique := entryFlags&0x8000 == 0x0000 // the top bit is the group bit
		if unique && name != "" && name != "__MSBROWSE__" && nbnsIsPrintable(name) {
			return name, true
		}
		entries = entries[18:]
	}
	return "", false
}

// nbnsIsPrintable keeps the parse honest: a name table entry the device
// actually set reads as visible text, not control bytes.
func nbnsIsPrintable(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// netbiosState is the one-at-a-time NetBIOS name query, run by itself
// when a device's details open: the name is its own fact, apart from
// the hostname the network reported, shown as its value or a dash.
type netbiosState struct {
	starter netbiosStarter
	busy    bool
	gen     int
	// asked holds the ips already asked this run, so returning to a
	// device does not ask it again.
	asked map[netip.Addr]bool
	// name is the answer for the ip of the pane, "" while none.
	name   string
	failed bool
}

// netbiosAskFor asks ip when it has not been asked yet; safe to call
// on every open of the details.
func (a *app) netbiosAskFor(ip netip.Addr) {
	nb := &a.netbios
	if nb.asked == nil {
		nb.asked = map[netip.Addr]bool{}
	}
	if nb.asked[ip] || nb.busy {
		return
	}
	if nb.starter == nil {
		nb.starter = realNetBIOS
	}
	nb.asked[ip] = true
	nb.busy = true
	nb.name, nb.failed = "", false
	nb.gen++
	gen := nb.gen
	go func() {
		// A name answers within the first exchange; a silent device is
		// given one retry before the ask gives up.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, err := nb.starter(ctx, ip)
		a.dispatch(func() {
			nb.busy = false
			if nb.gen != gen {
				return
			}
			switch {
			case err != nil:
				nb.failed = true
			case out == "":
				nb.failed = true
			default:
				nb.name = out
			}
		})
	}()
}

// netbiosFact is the NetBIOS 名 line of the facts: what the device
// itself reported over NetBIOS, which may differ from the hostname the
// network gave it. Asked when the details open; a dash while nothing
// came back — no words about asking or failing.
func (a *app) netbiosFact(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, "NetBIOS 名").FontSize(12).Width(64).TextColor(t.TextMuted).Shrink(0)
		var value string
		if a.netbios.name != "" {
			value = a.netbios.name
		}
		ui.Text(c, orDash(value)).FontSize(13).SingleLine()
	})
}
