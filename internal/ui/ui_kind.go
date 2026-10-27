package ui

import (
	"net/netip"

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

// deviceKindOf infers the kind from the strongest signals the sweep has:
// the gateway flag, the services a device advertises, its open ports and
// its vendor. The order matters — a gateway is a router whatever else it
// also prints or shares.
func deviceKindOf(h model.Host, ports []model.Port) deviceKind {
	switch {
	case h.Gateway:
		return kindRouter
	case hasService(h, "_ipp._tcp", "_printer._tcp", "_scanner._tcp") ||
		hasPort(ports, 9100, 515, 631):
		return kindPrinter
	case hasService(h, "_rtsp._tcp", "_hap._tcp") || isCameraVendor(h.Vendor):
		return kindCamera
	case hasService(h, "_airplay._tcp", "_googlecast._tcp"):
		return kindTV
	case hasService(h, "_smb._tcp", "_afpovertcp._tcp") || hasPort(ports, 445, 548, 2049):
		return kindNAS
	case hasService(h, "_workstation._tcp"):
		return kindComputer
	}
	return kindUnknown
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

// isCameraVendor spots the makers whose devices are nearly always cameras.
func isCameraVendor(vendor string) bool {
	switch vendor {
	case "Hikvision", "Dahua", "Axis", "Reolink", "TP-Link":
		return true
	}
	return false
}

// isSelf reports whether ip is this machine's own address on the adapter
// the sweep used, so the table can say which row is you.
func (a *app) isSelf(ip netip.Addr) bool {
	nic := a.currentNIC()
	return nic.IP.IsValid() && nic.IP == ip
}
