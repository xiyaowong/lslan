package ui

import (
	"fmt"
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
func (a *app) serviceSection(c *ui.Context, t *ui.Theme, h model.Host) {
	if len(h.Services) == 0 {
		return
	}
	section(c, t, "网络服务", fmt.Sprintf("自报 %d 项", len(h.Services)), func() {
		for i, sv := range h.Services {
			if i > 0 {
				ui.Divider(c).Shrink(0)
			}
			ui.Row(c).Height(34).Padding(0, 8, 0, 12).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, serviceLabel(sv)).SingleLine().FontSize(13)
				ui.Box(c).Grow(1)
				if sv.Port > 0 {
					ui.Textf(c, "%d", sv.Port).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
					// Not every service has a next step mapped (a Chromecast
					// or an AirPlay speaker, say); it shows without one.
					acts := portActions(h.IP, model.Port{Number: sv.Port, Service: mdnsShort(sv)})
					if len(acts) == 0 {
						return
					}
					if flatButton(c, acts[0].Label).Clicked() {
						if acts[0].Kind == portTelnet {
							a.startTelnet(h.IP, sv.Port)
						} else {
							runPortAction(c, acts[0])
						}
					}
				}
			})
		}
	})
}
