package ui

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// inventoryTable renders the rows on screen as a table for the clipboard:
// the router's MAC bindings, a note to a family member, a record for
// later — none of which needs a file, which the app does not write.
func (a *app) inventoryTable() string {
	hs := a.visibleHosts()
	if len(hs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("IP\t主机名\t厂商\tMAC 地址\t状态\n")
	for _, h := range hs {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", h.IP, h.Hostname, h.Vendor, h.MAC, stateLabel(h.State))
	}
	return b.String()
}

// macSet is the MACs of the devices seen this run, the identity a change
// note compares by: an address can move between devices, a MAC cannot.
func (a *app) macSet() map[string]bool {
	out := make(map[string]bool, len(a.hosts))
	for _, h := range a.hosts {
		if h.MAC != "" && h.State != model.StateOffline {
			out[h.MAC] = true
		}
	}
	return out
}

// noteChanges says what the sweep found that the last one did not, and
// what it lost. Only a sweep that ran to the end may draw conclusions:
// a cancelled one proves nothing about who is gone, and the first sweep
// has nothing to compare with — it records who is here for the next one.
func (a *app) noteChanges() {
	if !a.compared {
		// First completed sweep: keep who is here as the baseline, and
		// say nothing about changes.
		a.lastSeen = a.macSet()
		a.compared = true
		return
	}
	now := a.macSet()
	added, gone := 0, 0
	for mac := range now {
		if !a.lastSeen[mac] {
			added++
		}
	}
	for mac := range a.lastSeen {
		if !now[mac] {
			gone++
		}
	}
	a.lastSeen = now
	if added == 0 && gone == 0 {
		return
	}
	a.scanNote = "比上次多 " + strconv.Itoa(added) + " 台、少 " + strconv.Itoa(gone) + " 台"
}

// sourceAddr is the address TCP probes bind to: the selected adapter's,
// so a VPN's TUN adapter cannot take the LAN's connections and answer for
// devices that are not there (see scan.ScanPortsFrom).
func (a *app) sourceAddr() netip.Addr { return a.currentNIC().IP }

// visibleHosts lists the table's rows, filtered and sorted per state.
func (a *app) visibleHosts() []model.Host {
	out := make([]model.Host, 0, len(a.hosts))
	query := strings.ToLower(strings.TrimSpace(a.search))
	for _, h := range a.hosts {
		if !a.showOffline && h.State == model.StateOffline {
			continue
		}
		if query != "" && !a.matches(h, query) {
			continue
		}
		out = append(out, h)
	}
	sortHosts(out, a.sort)
	return out
}

// matches reports whether a host is worth showing for a search: the
// fields on screen, the type it was guessed to be, and the ports found on
// it, so a user can look for "打印机" or for a port number as well as an
// address.
func (a *app) matches(h model.Host, query string) bool {
	if deviceKindOf(h, a.ports[h.IP]).Label() == query {
		return true
	}
	for _, s := range []string{h.IP.String(), h.Hostname, h.Vendor, h.MAC} {
		if strings.Contains(strings.ToLower(s), query) {
			return true
		}
	}
	for _, p := range a.ports[h.IP] {
		if p.Open && strconv.Itoa(p.Number) == query {
			return true
		}
	}
	return false
}

func sortHosts(hs []model.Host, so ui.SortOrder) {
	// Every comparator ends in the IP, so rows keep one order frame to
	// frame no matter how the map iterates; ties must never flip or the
	// table jumps under a resting pointer. Descending negates instead of
	// reversing, keeping that order.
	desc := 1
	if so.Descending {
		desc = -1
	}
	var cmp func(x, y model.Host) int
	switch so.Column {
	case "state":
		cmp = func(x, y model.Host) int { return desc * (int(x.State) - int(y.State)) }
	case "vendor":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.Vendor, y.Vendor) }
	case "mac":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.MAC, y.MAC) }
	case "hostname":
		cmp = func(x, y model.Host) int { return desc * strings.Compare(x.Hostname, y.Hostname) }
	default:
		cmp = func(x, y model.Host) int { return desc * x.IP.Compare(y.IP) }
	}
	slices.SortFunc(hs, func(x, y model.Host) int {
		if c := cmp(x, y); c != 0 {
			return c
		}
		return x.IP.Compare(y.IP)
	})
}

func (a *app) onlineCount() int {
	n := 0
	for _, h := range a.hosts {
		if h.State == model.StateOnline {
			n++
		}
	}
	return n
}

// discoveredCount counts hosts the sweep found responding; the offline
// backfill of #21 does not count, the status bar shows it under 总数.
func (a *app) discoveredCount() int {
	n := 0
	for _, h := range a.hosts {
		if h.State != model.StateOffline {
			n++
		}
	}
	return n
}
