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
// NetBIOS name. The outcome — the name, a miss, or the failure — shows
// at the ask button until it is answered by a hostname, so a click is
// never without an answer (#83).
type netbiosState struct {
	starter netbiosStarter
	busy    bool
	gen     int
	// note says how the last ask went, in words a user can act on.
	note   string
	failed bool
}

// netbiosLookup asks ip for its NetBIOS name off-thread; the answer
// fills a missing hostname and reports itself at the button.
func (a *app) netbiosLookup(ip netip.Addr) {
	nb := &a.netbios
	if nb.busy {
		return
	}
	if nb.starter == nil {
		nb.starter = realNetBIOS
	}
	nb.busy = true
	nb.note, nb.failed = "", false
	nb.gen++
	gen := nb.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := nb.starter(ctx, ip)
		a.dispatch(func() {
			nb.busy = false
			if nb.gen != gen {
				return
			}
			switch {
			case err != nil:
				nb.note, nb.failed = "没有问到（设备可能不响应 NetBIOS）", true
			case netbiosName(out) == "":
				nb.note, nb.failed = "设备没有报出名字", true
			default:
				name := netbiosName(out)
				nb.note, nb.failed = name, false
				if h, ok := a.hosts[ip]; ok && h.Hostname == "" {
					h.Hostname = name
					a.hosts[ip] = h
				}
			}
		})
	}()
}

// netbiosAsk is the detail panel's way to ask a hostname-less device
// for its NetBIOS name: the ask at the facts' hostname row, with the
// query running, its outcome, or the ask itself beside the name.
func (a *app) netbiosAsk(c *ui.Context, t *ui.Theme, h model.Host) {
	if h.Hostname != "" {
		return
	}
	nb := &a.netbios
	switch {
	case nb.busy:
		ui.Spinner(c)
		ui.Text(c, "正在询问…").FontSize(12).TextColor(t.TextMuted)
	default:
		if flatButton(c, "NetBIOS 问名").Clicked() {
			a.netbiosLookup(h.IP)
		}
		if nb.note != "" {
			color := t.TextMuted
			if nb.failed {
				color = t.Danger
			}
			ui.Text(c, nb.note).FontSize(12).TextColor(color)
		}
	}
}
