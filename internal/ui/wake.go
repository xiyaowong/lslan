package ui

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// Wake on LAN is the one tool not about a device already on the screen: a
// sleeping device is by definition absent from the sweep, so it works from
// what the user knows — an address and the device's hardware address — and
// then watches that address come back.
const (
	wakeWaitEvery = 5 * time.Second
	wakeWaitFor   = 30 * time.Second
	// wakePickLimit caps the quick picks of devices this run has seen.
	wakePickLimit = 6
)

// wakeState is the 远程唤醒 dialog and the wait that follows a send: what
// was typed, and the address being watched. One wait at a time; wakeGen
// drops the answers of a wait that has since been replaced.
type wakeState struct {
	open   bool
	ipText string // the address typed or picked, "" for none
	mac    string // the hardware address for it
	// filled is the MAC the lookup put in: while mac still holds it (or
	// nothing), a new address refills; once the user types another, the
	// typed one stays.
	filled string
	// errMsg is why the last send did not go, "" when none was tried.
	errMsg string
	// wakeErr is the scan's last failure, kept inside the dialog: the
	// status bar that normally shows it is behind the dialog's backdrop.
	wakeErr string

	waiting    bool
	waitIP     netip.Addr
	waitCancel context.CancelFunc
	waitGen    int
}

// wakeDevices lists the devices that are, or were this run, known
// hardware: the offline ones first — the ones worth waking — then the
// online, each in address order. An address the sweep found nothing at is
// not a device: every subnet has empty ones, and offering them all would
// bury the picks that matter.
func (a *app) wakeDevices() []model.Host {
	out := make([]model.Host, 0, len(a.hosts))
	for _, h := range a.hosts {
		if h.MAC == "" && h.State != model.StateOnline {
			continue
		}
		out = append(out, h)
	}
	slices.SortStableFunc(out, func(a, b model.Host) int {
		oa, ob := a.State == model.StateOffline, b.State == model.StateOffline
		if oa != ob {
			if oa {
				return -1
			}
			return 1
		}
		return a.IP.Compare(b.IP)
	})
	return out
}

// wakePicks are the quick picks: the devices that went offline since the
// last sweep saw them — waking one is what the tool is for — plus, when
// there are none, the ones online now.
func (a *app) wakePicks() []model.Host {
	devices := a.wakeDevices()
	var out []model.Host
	for _, h := range devices {
		if h.State != model.StateOffline {
			continue
		}
		out = append(out, h)
		if len(out) == wakePickLimit {
			return out
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, h := range devices {
		if h.State != model.StateOnline {
			continue
		}
		out = append(out, h)
		if len(out) == wakePickLimit {
			break
		}
	}
	return out
}

// wakePickLabel names a pick: the address, then the name or vendor when
// one is known, and 离线 for one that went dark — the usual case, and the
// reason to open this at all.
func wakePickLabel(h model.Host) string {
	label := h.IP.String()
	if name := wakeName(h); name != "" {
		label += " " + name
	}
	if h.State == model.StateOffline {
		label += " 离线"
	}
	return label
}

func wakeName(h model.Host) string {
	return cmp.Or(h.Hostname, h.Vendor)
}

// pickWakeDevice sets the address from a pick and fills its MAC, the
// pick being an explicit choice of device.
func (a *app) pickWakeDevice(ip netip.Addr) {
	a.wake.ipText, a.wake.errMsg = ip.String(), ""
	a.wakeFill()
}

// wakeFill fills the MAC for the address now entered, from the sweep's
// record or the system's ARP table; a MAC the user typed over the filled
// one stays.
func (a *app) wakeFill() {
	w := &a.wake
	if w.mac != w.filled && strings.TrimSpace(w.mac) != "" {
		return // the user's own
	}
	w.filled = a.wakeMACFor(w.ipText)
	w.mac = w.filled
}

// wakeMACFor is the hardware address known for the typed address: the
// sweep's MAC, else the system's ARP table, else "".
func (a *app) wakeMACFor(text string) string {
	ip, err := netip.ParseAddr(strings.TrimSpace(text))
	if err != nil {
		return ""
	}
	if h, known := a.hosts[ip]; known && h.MAC != "" {
		return h.MAC
	}
	return a.wakeMac(ip)
}

// wakeTarget is the address about to be woken.
func (a *app) wakeTarget() (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(a.wake.ipText))
	if err != nil || !ip.Is4() || ip.IsUnspecified() {
		return netip.Addr{}, false
	}
	return ip, true
}

// startWake sends the frame and watches the device come back. Neither the
// address nor the MAC may be off: a frame built from a mistyped one wakes
// nothing and says nothing, so the dialog stays up with the reason.
func (a *app) startWake() {
	w := &a.wake
	ip, ok := a.wakeTarget()
	if !ok {
		w.errMsg = "IP 地址不对，要像 192.168.1.20 这样"
		return
	}
	mac, ok := scan.ParseMAC(w.mac)
	if !ok {
		w.errMsg = "MAC 地址不对，要有 12 位，像 aa:bb:cc:dd:ee:ff"
		return
	}
	if err := a.wakeSend(a.currentNIC(), mac); err != nil {
		w.errMsg = "信号没能发出去，请检查网卡后重试"
		return
	}
	w.mac = mac
	w.open, w.errMsg = false, ""
	a.waitWake(ip)
}

// waitWake probes ip every few seconds for a short while, marking it
// online the moment it answers; the wait ends quietly otherwise, and each
// outcome is said once.
func (a *app) waitWake(ip netip.Addr) {
	a.stopWakeWait()
	ctx, cancel := context.WithTimeout(context.Background(), a.wakeFor)
	w := &a.wake
	w.waitCancel = cancel
	w.waiting, w.waitIP = true, ip
	w.waitGen++
	gen := w.waitGen
	src := a.sourceAddr()
	go func() {
		defer cancel()
		for {
			if a.wakeProbe(src, ip) {
				a.dispatch(func() {
					if w.waitGen == gen {
						a.wakeHeard(ip)
					}
				})
				return
			}
			select {
			case <-ctx.Done():
				a.dispatch(func() {
					if w.waitGen == gen {
						// The status bar stops waiting; that is the news.
						w.waiting = false
					}
				})
				return
			case <-time.After(a.wakeEvery):
			}
		}
	}()
}

// wakeHeard marks the woken device online, so the row it left as offline
// lights up. That row is the result; no toast repeats it.
func (a *app) wakeHeard(ip netip.Addr) {
	w := &a.wake
	w.waiting = false
	h := a.hosts[ip]
	h.IP, h.State = ip, model.StateOnline
	h.LastSeen = time.Now()
	a.hosts[ip] = h
}

// stopWakeWait ends the wait and clears it; safe with none running.
func (a *app) stopWakeWait() {
	w := &a.wake
	w.waitGen++
	if w.waitCancel != nil {
		w.waitCancel()
		w.waitCancel = nil
	}
	w.waiting, w.waitIP = false, netip.Addr{}
}

// wakeButton is the status bar's 远程唤醒: the tool's entry, in the
// status bar rather than a device's details, since it is not about the
// device on screen.
func (a *app) wakeButton(c *ui.Context) {
	if flatButton(c, "远程唤醒").Clicked() {
		a.openWake()
	}
}

// openWake opens the dialog on the device shown, when one is; what was
// typed last otherwise stays. Nothing has been swept yet means there is no
// device to pick from, so the sweep starts by itself; a run that already
// has results keeps them — sweeping again would empty the table under the
// user for a dialog they may only be looking at.
func (a *app) openWake() {
	w := &a.wake
	w.open, w.errMsg = true, ""
	if a.selIP.IsValid() && strings.TrimSpace(w.ipText) == "" {
		w.ipText = a.selIP.String()
	}
	a.wakeFill()
	if !a.swept() && !a.scanning {
		a.startScan()
	}
}

// swept reports whether a sweep has been started this run, so the wake
// dialog knows whether it has devices to offer.
func (a *app) swept() bool { return a.progress.Total > 0 }

// currentNIC is the adapter the frame goes out of: the toolbar's choice,
// else the first physical one.
func (a *app) currentNIC() model.NIC {
	if nic := a.selectedNIC(); nic != nil {
		return *nic
	}
	for _, n := range a.nics {
		if !n.Virtual {
			return n
		}
	}
	return model.NIC{}
}

// wakeDialog is the 远程唤醒 dialog: the devices this run saw offline as
// one-click picks, an address to type instead, the hardware address
// filled in by itself and editable, and the send. The sweep it opens with
// is under way or done, so a device it found but the user cannot name yet
// is one button, 重新扫描， away.
func (a *app) wakeDialog(c *ui.Context, t *ui.Theme) {
	w := &a.wake
	ui.Modal(c, &w.open, func() {
		ui.Column(c).Gap(14).Width(380).Children(func() {
			ui.Text(c, "远程唤醒").FontSize(15).FontWeight(600)
			ui.Text(c, "给关机的设备发一个唤醒信号，发完会盯一会儿，看它有没有上线").
				FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).Gap(space(2)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.wakeScanNote()).FontSize(12).TextColor(t.TextMuted).Shrink(1).SingleLine()
				ui.Box(c).Grow(1)
				// The scan's own errors show in the status bar, which this
				// dialog covers; say why here instead.
				if btn := smallButton(c, a.wakeScanLabel()).
					Disabled(a.scanning || a.wakeScanBlocker() != ""); !a.scanning && btn.Clicked() {
					a.wake.wakeErr = ""
					a.startScan()
					a.wake.wakeErr = a.scanErr
				}
			})
			if a.wake.wakeErr != "" {
				ui.Text(c, a.wake.wakeErr).FontSize(12).TextColor(t.Danger).SingleLine()
			} else if why := a.wakeScanBlocker(); why != "" {
				ui.Text(c, why).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}

			if picks := a.wakePicks(); len(picks) > 0 {
				ui.Column(c).Gap(space(1)).Children(func() {
					ui.Text(c, "选一台设备").FontSize(12).TextColor(t.TextMuted)
					// Six devices at a comfortable size, and the window's
					// least height leaves the fields and the buttons below
					// theirs: more picks than that scroll rather than run
					// off the bottom.
					ui.Scroll(c).Height(120).Clip().Children(func() {
						ui.Row(c).Gap(space(1)).Wrap().Children(func() {
							for _, h := range picks {
								if btn := smallButton(c, wakePickLabel(h)); btn.Clicked() {
									a.pickWakeDevice(h.IP)
								}
							}
						})
					})
				})
			}

			ui.Column(c).Gap(space(1)).Children(func() {
				ui.Text(c, "设备地址").FontSize(12).TextColor(t.TextMuted)
				if field := ui.TextInput(c, &w.ipText).Label("设备地址").
					Placeholder("也可以直接填，如 192.168.1.20"); field.Changed() {
					w.errMsg = ""
					a.wakeFill()
				}
			})

			ui.Column(c).Gap(space(1)).Children(func() {
				ui.Text(c, "MAC 地址").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &w.mac).Label("MAC 地址").
					Placeholder("设备网卡地址，如 aa:bb:cc:dd:ee:ff")
				ui.Text(c, "找到就自动填好了，找不到请照设备上的标签填写").
					FontSize(11).TextColor(t.TextMuted)
			})

			if msg := a.wakeProblem(); msg != "" {
				ui.Text(c, msg).FontSize(12).TextColor(t.Danger)
			}

			ui.Row(c).Gap(space(2)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1)
				if ui.Button(c, "取消").Clicked() {
					w.open = false
				}
				if btn := ui.PrimaryButton(c, "发送唤醒信号").Disabled(!a.wakeReady()); btn.Clicked() {
					a.startWake()
				}
			})
		})
	})
}

// wakeScanLabel names the dialog's scan button: a sweep under way cannot
// start another, so the button keeps its word and is disabled.
func (a *app) wakeScanLabel() string {
	if a.scanning {
		return "正在扫描…"
	}
	if a.swept() {
		return "重新扫描"
	}
	return "扫描地址"
}

// wakeScanBlocker says what keeps the dialog's scan from starting, "" when
// nothing does: the same checks startScan makes, said in the dialog rather
// than in the status bar behind it.
func (a *app) wakeScanBlocker() string {
	switch {
	case a.selectedNIC() == nil:
		return "先选一块网卡，才能扫描地址"
	case !a.rangeFits():
		return "范围太大或没填完，请先调整上面的地址范围"
	}
	return ""
}

// wakeScanNote says where the picks come from: the sweep's progress, or
// that none has run yet.
func (a *app) wakeScanNote() string {
	switch {
	case a.scanning:
		return fmt.Sprintf("正在扫描 %d/%d…", a.progress.Scanned, a.progress.Total)
	case a.swept():
		return "从这次扫描的结果里选，也可以直接填地址"
	case len(a.nics) == 0:
		return "先选一块网卡，才能扫描地址"
	}
	return "还没扫描过地址，点右边的按钮扫一次"
}

// wakeReady reports whether the send can go: an address and a MAC.
func (a *app) wakeReady() bool {
	w := &a.wake
	if _, ok := a.wakeTarget(); !ok {
		return false
	}
	_, ok := scan.ParseMAC(w.mac)
	return ok
}

// wakeProblem says why the send cannot go, "" when nothing stops it: the
// reason of a send that was tried, else what is still missing, so the
// disabled button never leaves the user guessing.
func (a *app) wakeProblem() string {
	w := &a.wake
	if w.errMsg != "" {
		return w.errMsg
	}
	if _, ok := a.wakeTarget(); !ok {
		if strings.TrimSpace(w.ipText) == "" {
			return ""
		}
		return "IP 地址不对，要像 192.168.1.20 这样"
	}
	if strings.TrimSpace(w.mac) == "" {
		return "没有找到这台设备的 MAC 地址，请照设备上的标签填上"
	}
	if _, ok := scan.ParseMAC(w.mac); !ok {
		return "MAC 地址不对，要有 12 位，像 aa:bb:cc:dd:ee:ff"
	}
	if !a.currentNIC().IP.IsValid() {
		return "先选一块网卡，才知道从哪发出去"
	}
	return ""
}

// wakeStatus is the status bar's word on a wait: a spinner and the device
// being waited on.
func (a *app) wakeStatus(c *ui.Context, t *ui.Theme) {
	w := &a.wake
	if !w.waiting {
		return
	}
	ui.Row(c).Gap(space(1)).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Spinner(c)
		ui.Text(c, "正在等 "+w.waitIP.String()+" 上线").FontSize(12).TextColor(t.TextMuted)
		if flatButton(c, "不等了").Clicked() {
			a.stopWakeWait()
		}
	})
}
