package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// serviceLabels maps DNS-SD service types to plain words; anything
// else falls back to the type with its punctuation stripped.
var serviceLabels = map[string]string{
	"_http._tcp":            "网页服务",
	"_https._tcp":           "网页服务",
	"_ipp._tcp":             "打印服务",
	"_printer._tcp":         "打印服务",
	"_scanner._tcp":         "扫描服务",
	"_smb._tcp":             "文件共享",
	"_afpovertcp._tcp":      "文件共享",
	"_workstation._tcp":     "文件共享",
	"_rtsp._tcp":            "视频流",
	"_airplay._tcp":         "投屏",
	"_googlecast._tcp":      "投屏",
	"_spotify-connect._tcp": "音乐投放",
	"_raop._tcp":            "音频投放",
	"_hap._tcp":             "智能家居",
	"_device-info._tcp":     "设备信息",
	"_ssh._tcp":             "远程登录",
	"_ftp._tcp":             "文件传输",
	"_nvstream._tcp":        "游戏串流",
}

// mdnsShort strips a DNS-SD type to the service word the port action
// mapping matches on, e.g. "_http._tcp" → "http".
func mdnsShort(s model.Service) string {
	name := strings.ToLower(s.Name)
	name = strings.TrimSuffix(name, ".local")
	name = strings.TrimSuffix(name, "._tcp")
	name = strings.TrimSuffix(name, "._udp")
	return strings.TrimPrefix(name, "_")
}

// serviceLabel is the service type in words a home user knows.
func serviceLabel(s model.Service) string {
	if label, ok := serviceLabels[strings.ToLower(s.Name)]; ok {
		return label
	}
	return mdnsShort(s)
}

// serviceSection lists what the selected host advertises over the
// network's service discovery, with the same next steps as port rows.
func (a *app) serviceSection(c *ui.Context, h model.Host) {
	if len(h.Services) == 0 {
		return
	}
	t := c.Theme()
	ui.Textf(c, "这台设备自报了 %d 项服务", len(h.Services)).FontSize(12).TextColor(t.TextMuted).Shrink(0)
	for _, sv := range h.Services {
		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(6).Shrink(0).Children(func() {
			ui.Text(c, serviceLabel(sv)).Width(76).SingleLine().FontSize(12)
			if sv.Port > 0 {
				ui.Textf(c, "%d", sv.Port).Font("monospace").Width(52).TextAlign(ui.End).
					FontSize(12).TextColor(t.Text)
			} else {
				ui.Box(c).Width(52)
			}
			ui.Box(c).Grow(1)
			if sv.Port > 0 {
				for _, act := range portActions(h.IP, model.Port{Number: sv.Port, Service: mdnsShort(sv)}) {
					btn := ui.Button(c, act.Label).FontSize(12)
					if btn.Clicked() {
						runPortAction(c, act)
					}
				}
			}
		})
	}
}
