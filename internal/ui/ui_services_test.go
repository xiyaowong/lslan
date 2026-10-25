package ui

import (
	"testing"

	"lslan/internal/model"
)

func TestServiceLabel(t *testing.T) {
	cases := map[string]string{
		"_http._tcp":        "网页服务",
		"_googlecast._tcp":  "投屏",
		"_printer._tcp":     "打印服务",
		"_workstation._tcp": "文件共享",
		"_weird._udp":       "weird",
	}
	for name, want := range cases {
		if got := serviceLabel(model.Service{Name: name}); got != want {
			t.Errorf("serviceLabel(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestMDNSShort(t *testing.T) {
	cases := map[string]string{
		"_http._tcp":            "http",
		"_rtsp._tcp":            "rtsp",
		"_ipp._tcp.local":       "ipp",
		"_spotify-connect._tcp": "spotify-connect",
	}
	for name, want := range cases {
		if got := mdnsShort(model.Service{Name: name}); got != want {
			t.Errorf("mdnsShort(%q) = %q, want %q", name, got, want)
		}
	}
}
