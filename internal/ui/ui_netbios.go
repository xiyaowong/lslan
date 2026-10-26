package ui

import (
	"context"
	"net/netip"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// netbiosStarter runs one NetBIOS name query against ip; tests replace
// it.
type netbiosStarter func(ctx context.Context, ip netip.Addr) (stdout string, err error)

// realNetBIOS queries nbtstat -A. Its output is in the machine's OEM
// code page (GBK on Chinese Windows) whatever chcp says; the name table
// itself is ASCII, so the lines the parse needs read the same either
// way and undecodable bytes just stay raw.
func realNetBIOS(ctx context.Context, ip netip.Addr) (string, error) {
	cmd := exec.CommandContext(ctx, "cmd", "/c", "nbtstat -A "+ip.String())
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// netbiosRecord is one <00> UNIQUE Registered line of the name table,
// whose name column is up to 15 wide and may hold spaces inside.
var netbiosRecord = regexp.MustCompile(`(?m)^\s{4}(\S(?:.{0,13}\S)?)\s{2,}<00>\s*UNIQUE\s+Registered`)

// netbiosName picks the machine's NetBIOS name out of nbtstat output.
// Empty when the query found nothing or the text does not parse.
func netbiosName(out string) string {
	m := netbiosRecord.FindStringSubmatch(out)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// netbiosState is the one-at-a-time query for the selected device's
// NetBIOS name; it fills a missing hostname and never overwrites one.
type netbiosState struct {
	starter netbiosStarter
	busy    bool
	gen     int
}

// netbiosLookup asks ip for its NetBIOS name off-thread; the answer
// lands in the host when it has no hostname yet.
func (a *app) netbiosLookup(ip netip.Addr) {
	nb := &a.netbios
	if nb.busy {
		return
	}
	if nb.starter == nil {
		nb.starter = realNetBIOS
	}
	nb.busy = true
	nb.gen++
	gen := nb.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := nb.starter(ctx, ip)
		a.dispatch(func() {
			nb.busy = false
			if nb.gen != gen || err != nil {
				return
			}
			name := netbiosName(out)
			if name == "" {
				return
			}
			if h, ok := a.hosts[ip]; ok && h.Hostname == "" {
				h.Hostname = name
				a.hosts[ip] = h
			}
		})
	}()
}

// netbiosAsk is the detail panel's way to ask a hostname-less device
// for its NetBIOS name: the ask button at the facts' hostname row.
func (a *app) netbiosAsk(c *ui.Context, h model.Host) {
	if h.Hostname != "" {
		return
	}
	if a.netbios.busy {
		return
	}
	if flatButton(c, "NetBIOS 问名").Clicked() {
		a.netbiosLookup(h.IP)
	}
}
