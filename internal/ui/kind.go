package ui

import (
	"net/netip"
	"strings"

	"lslan/internal/model"
)

// What a device is, guessed from what the sweep already learned. The
// question "which one is 192.168.1.7" is the first one a home user asks,
// and the raw hostname, vendor and port list do not answer it. Anything
// uncertain says 未知 rather than guessing.
type deviceKind int

const (
	kindUnknown deviceKind = iota
	kindRouter
	kindPrinter
	kindCamera
	kindNAS
	kindTV
	kindPhone
	kindComputer
)

// kindLabel is the kind in plain words.
func (k deviceKind) Label() string {
	switch k {
	case kindRouter:
		return "路由器"
	case kindPrinter:
		return "打印机"
	case kindCamera:
		return "摄像头"
	case kindNAS:
		return "存储设备"
	case kindTV:
		return "电视 / 投屏"
	case kindPhone:
		return "手机 / 平板"
	case kindComputer:
		return "电脑"
	}
	return "未知"
}

// kindRule is one way to recognize a kind from what a device does: the
// services it advertises and the ports it answers on.
//
// A vendor is not a clue here on purpose. The OUI table says who made a
// device, not what it is — TP-Link makes routers, access points, cameras
// and plugs — so a vendor rule mislabels the majority to catch a few. The
// device's own model is the clue that carries: it comes from the device
// describing itself, and it names the product.
type kindRule struct {
	kind     deviceKind
	services []string // DNS-SD types, e.g. "_ipp._tcp"
	ports    []int    // open TCP ports
}

// kindRules is the whole classification, in one table: adding a kind or
// a clue means adding a rule here, not touching the inference. The order
// is the strength: a camera before a computer, since many cameras also
// answer as workstations.
var kindRules = []kindRule{
	{kind: kindCamera, services: []string{"_rtsp._tcp", "_hap._camera._tcp"}},
	{kind: kindPrinter,
		services: []string{"_ipp._tcp", "_printer._tcp", "_scanner._tcp"},
		ports:    []int{9100, 515, 631}},
	{kind: kindTV, services: []string{"_airplay._tcp", "_googlecast._tcp"}},
	{kind: kindPhone, services: []string{"_raop._tcp", "_spotify-connect._tcp"}},
	{kind: kindNAS,
		services: []string{"_smb._tcp", "_afpovertcp._tcp"},
		ports:    []int{445, 548, 2049}},
	{kind: kindComputer,
		services: []string{"_workstation._tcp", "_ssh._tcp"},
		ports:    []int{3389}},
}

// modelHints name the product in a name a device goes by. Matched as
// lowercased substrings, against what a device announced about itself
// (its model) or the name it answers to — never against the vendor, which
// says who made it, not what it is.
//
// The words are the ones products actually carry: "IPC" is what cameras
// are called in their model numbers, "DS-2CD" and "IPC-HDW" are Hikvision
// cameras, "OMADA" and "DECO" are TP-Link's network gear. Domestic boxes
// announce bare internal codes — "TL-XDR3010", "WS7200", "MJSXJ22CM" — so
// the codes that have stayed stable are listed too, along with the few
// Chinese words a box that names itself in Chinese would use.
var modelHints = []struct {
	kind  deviceKind
	words []string
}{
	{kindCamera, []string{
		"camera", "ipcam", "ipc", "nvr", "dvr", "ds-2", "dh-", "reolink",
		"hikvision", "dahua", "foscam", "amcrest", "wyze cam",
		"tapo c", "mjsxj", "摄像头", "摄像机",
	}},
	{kindPrinter, []string{
		"printer", "print", "laserjet", "officejet", "deskjet", "envy",
		"pixma", "ecotank", "workforce", "imageclass", "brother hl",
		"brother dcp", "brother mfc", "打印机",
	}},
	{kindRouter, []string{
		"router", "gateway", "access point", "archer", "deco", "omada",
		"orbi", "velop", "eero", "amplifi", "ax3000", "ax1800", "ac1200",
		"ac1750", "ax6000", "be3600", "mesh", "repeater", "extender",
		"tl-xdr", "tl-7dr", "ws5200", "ws7100", "ws7200", "ws8700",
		"miwifi", "mi router", "mi-router", "路由",
	}},
	{kindNAS, []string{
		"diskstation", "rackstation", "synology", "qnap", "readynas",
		"terramaster", "nas",
	}},
	{kindTV, []string{
		"tv", "bravia", "chromecast", "apple tv", "roku", "fire tv",
		"shield", "mi box", "电视",
	}},
	{kindPhone, []string{
		"iphone", "ipad", "galaxy", "pixel", "redmi", "oneplus", "sm-",
		"mate ", "mi 1", "honor",
	}},
	{kindComputer, []string{
		"macbook", "imac", "thinkpad", "ideapad", "surface", "desktop-",
		"laptop", "pc-",
	}},
}

// deviceKindOf infers the kind from the strongest signals the sweep has:
// the gateway flag, a model the device announced, what it serves, then
// the name it answers to. A device that matches none says 未知 rather
// than a guess.
func deviceKindOf(h model.Host, ports []model.Port) deviceKind {
	if h.Gateway {
		return kindRouter
	}
	// The model is the device talking about itself, so it outranks
	// everything inferred from what it serves.
	if kind, ok := kindFromModel(h.Model); ok {
		return kind
	}
	for _, r := range kindRules {
		if r.matches(h, ports) {
			return r.kind
		}
	}
	// A name the device answered to can still say it, when it says
	// "camera" or "printer" in the hostname.
	if kind, ok := kindFromModel(h.Hostname); ok {
		return kind
	}
	return kindUnknown
}

// kindFromModel reads a product name for the kinds whose names are
// distinctive.
func kindFromModel(name string) (deviceKind, bool) {
	if name == "" {
		return kindUnknown, false
	}
	n := " " + strings.ToLower(name) + " "
	for _, h := range modelHints {
		for _, w := range h.words {
			if strings.Contains(n, w) {
				return h.kind, true
			}
		}
	}
	return kindUnknown, false
}

// matches reports whether the host shows a clue the rule looks for.
func (r kindRule) matches(h model.Host, ports []model.Port) bool {
	return hasService(h, r.services...) || hasPort(ports, r.ports...)
}

func hasService(h model.Host, names ...string) bool {
	for _, s := range h.Services {
		for _, n := range names {
			if s.Name == n {
				return true
			}
		}
	}
	return false
}

func hasPort(ports []model.Port, numbers ...int) bool {
	for _, p := range ports {
		if !p.Open {
			continue
		}
		for _, n := range numbers {
			if p.Number == n {
				return true
			}
		}
	}
	return false
}

// isSelf reports whether ip is this machine's own address on the adapter
// the sweep used, so the table can say which row is you.
func (a *app) isSelf(ip netip.Addr) bool {
	nic := a.currentNIC()
	return nic.IP.IsValid() && nic.IP == ip
}
