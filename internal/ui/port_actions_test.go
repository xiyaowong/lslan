package ui

import (
	"fmt"
	"net/netip"
	"testing"

	"lslan/internal/model"
)

func ip(s string) netip.Addr { return netip.MustParseAddr(s) }

// sameAction compares two actions field by field; portAction holds a
// slice and cannot be compared with ==.
func sameAction(got, want portAction) bool {
	if got.Kind != want.Kind || got.Label != want.Label || got.Arg != want.Arg || len(got.Args) != len(want.Args) {
		return false
	}
	for i := range want.Args {
		if got.Args[i] != want.Args[i] {
			return false
		}
	}
	return true
}

// TestPortActions checks the mapping of open ports to next steps: the
// generic copy always comes last, and every action only opens or copies.
func TestPortActions(t *testing.T) {
	cases := []struct {
		port model.Port
		want []portAction // first entries; the generic copy ends every row
	}{
		{model.Port{Number: 80, Service: "http", Open: true},
			[]portAction{{portOpen, "打开管理页", "http://192.168.1.5", nil}}},
		{model.Port{Number: 8080, Service: "http", Open: true},
			[]portAction{{portOpen, "打开管理页", "http://192.168.1.5:8080", nil}}},
		{model.Port{Number: 443, Service: "https", Open: true},
			[]portAction{{portOpen, "打开管理页", "https://192.168.1.5", nil}}},
		{model.Port{Number: 554, Service: "rtsp", Open: true},
			[]portAction{{portCopy, "复制流地址", "rtsp://192.168.1.5/", nil}}},
		{model.Port{Number: 8554, Service: "rtsp", Open: true},
			[]portAction{{portCopy, "复制流地址", "rtsp://192.168.1.5:8554/", nil}}},
		{model.Port{Number: 445, Service: "microsoft-ds", Open: true},
			[]portAction{{portLaunch, "打开文件共享", "explorer.exe", []string{`\\192.168.1.5`}},
				{portCopy, "复制共享路径", `\\192.168.1.5`, nil}}},
		{model.Port{Number: 21, Service: "ftp", Open: true},
			[]portAction{{portCopy, "复制地址", "ftp://192.168.1.5", nil}}},
		{model.Port{Number: 22, Service: "ssh", Open: true},
			[]portAction{{portCopy, "复制 SSH 命令", "ssh 192.168.1.5", nil}}},
		{model.Port{Number: 3389, Service: "ms-wbt-server", Open: true},
			[]portAction{{portLaunch, "远程桌面", "mstsc.exe", []string{"/v:192.168.1.5"}}}},
		{model.Port{Number: 9100, Service: "printer", Open: true}, nil},
	}
	addr := ip("192.168.1.5")
	for _, c := range cases {
		got := portActions(addr, c.port)
		last := len(got) - 1
		if got[last].Label != "复制端口" || got[last].Kind != portCopy {
			t.Errorf("port %d: last action = %+v, want the generic 复制端口", c.port.Number, got[last])
			continue
		}
		if len(got)-1 != len(c.want) {
			t.Errorf("port %d: %d specific actions, want %d (%+v)", c.port.Number, len(got)-1, len(c.want), got)
			continue
		}
		for i, w := range c.want {
			if !sameAction(got[i], w) {
				t.Errorf("port %d action %d = %+v, want %+v", c.port.Number, i, got[i], w)
			}
		}
		if w := fmt.Sprintf("端口 %d（%s）", c.port.Number, portLabel(c.port)); got[last].Arg != w {
			t.Errorf("port %d copy arg = %q, want %q", c.port.Number, got[last].Arg, w)
		}
	}
}

// TestPortLabels checks the plain-words service names.
func TestPortLabels(t *testing.T) {
	cases := []struct {
		port model.Port
		want string
	}{
		{model.Port{Number: 80, Service: "http"}, "网页"},
		{model.Port{Number: 445}, "文件共享"},
		{model.Port{Number: 554, Service: "rtsp"}, "视频流"},
		{model.Port{Number: 22, Service: "ssh"}, "远程登录"},
		{model.Port{Number: 21}, "文件传输"},
		{model.Port{Number: 3389}, "远程桌面"},
		{model.Port{Number: 5432, Service: "postgres"}, "postgres"},
		{model.Port{Number: 1234}, "—"},
	}
	for _, c := range cases {
		if got := portLabel(c.port); got != c.want {
			t.Errorf("portLabel(%d %s) = %q, want %q", c.port.Number, c.port.Service, got, c.want)
		}
	}
}
