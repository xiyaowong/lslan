package ui

import (
	"fmt"
	"net/netip"
	"os/exec"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// portActionKind says what running a port action does.
type portActionKind int

const (
	portCopy   portActionKind = iota // put Arg on the clipboard
	portOpen                         // open Arg in the browser
	portLaunch                       // start the system program Arg with Args
	portTelnet                       // open the in-app terminal on the port
)

// portAction is one next step for an open port. All steps are
// side-effect free: they only open something or copy text, never install
// or change the system; connections stay in the user's hands.
type portAction struct {
	Kind  portActionKind
	Label string
	Arg   string
	Args  []string
}

// portActions maps an open port to its next steps, generic last.
func portActions(ip netip.Addr, p model.Port) []portAction {
	var acts []portAction
	share := `\\` + ip.String()
	switch {
	case p.Number == 80 || p.Number == 8080 || p.Service == "http":
		acts = append(acts, portAction{portOpen, "打开", serviceURL("http", ip, p.Number), nil})
	case p.Number == 443 || p.Number == 8443 || p.Service == "https":
		acts = append(acts, portAction{portOpen, "打开", serviceURL("https", ip, p.Number), nil})
	case p.Number == 554 || p.Number == 8554 || p.Number == 10554 || p.Service == "rtsp":
		acts = append(acts, portAction{portCopy, "复制流地址", serviceURL("rtsp", ip, p.Number), nil})
	case p.Number == 445 || p.Number == 139:
		acts = append(acts,
			portAction{portLaunch, "打开", "explorer.exe", []string{share}},
			portAction{portCopy, "复制路径", share, nil},
		)
	case p.Number == 21 || p.Service == "ftp":
		acts = append(acts, portAction{portCopy, "复制地址", serviceURL("ftp", ip, p.Number), nil})
	case p.Number == 22 || p.Service == "ssh":
		acts = append(acts, portAction{portCopy, "复制命令", "ssh " + ip.String(), nil})
	case p.Number == 23 || p.Service == "telnet":
		acts = append(acts, portAction{portTelnet, "连接测试", ip.String(), nil})
	case p.Number == 3389:
		acts = append(acts, portAction{portLaunch, "远程桌面", "mstsc.exe", []string{"/v:" + ip.String()}})
	}
	return acts
}

// runPortAction performs one mapped action, saying what it did; a
// failed launch is silent, there is nothing to clean up.
func runPortAction(c *ui.Context, act portAction) {
	switch act.Kind {
	case portCopy:
		c.WriteClipboard(act.Arg)
		c.Toast("已复制：" + act.Arg)
	case portOpen:
		c.OpenURL(act.Arg)
		c.Toast("已在浏览器打开 " + act.Arg)
	case portLaunch:
		if err := exec.Command(act.Arg, act.Args...).Start(); err != nil {
			c.Toast("没能打开,可以复制地址后手动打开")
			return
		}
		c.Toast("已交给系统打开")
	}
}

// serviceURL builds scheme://ip, leaving out the port when it is the
// scheme's default. RTSP gets the trailing slash stream players expect.
func serviceURL(scheme string, ip netip.Addr, port int) string {
	u := scheme + "://" + ip.String()
	def := map[string]int{"http": 80, "https": 443, "ftp": 21, "rtsp": 554}[scheme]
	if port != def {
		u += fmt.Sprintf(":%d", port)
	}
	if scheme == "rtsp" {
		u += "/"
	}
	return u
}

// portTip keeps the protocol name available on hover, where the plain
// label replaces it.
func portTip(p model.Port) string {
	if p.Service == "" {
		return ""
	}
	return p.Service
}

// portLabel is the service in words a home user knows; a service with no
// plain name still gets a word rather than an English protocol name.
func portLabel(p model.Port) string {
	switch {
	case p.Number == 21 || p.Service == "ftp":
		return "文件传输"
	case p.Number == 22 || p.Service == "ssh":
		return "远程登录"
	case p.Number == 23 || p.Service == "telnet":
		return "远程终端"
	case p.Number == 80 || p.Number == 8080 || p.Number == 443 || p.Number == 8443 ||
		p.Service == "http" || p.Service == "https":
		return "网页"
	case p.Number == 139 || p.Number == 445 || p.Service == "smb" || p.Service == "netbios-ssn":
		return "文件共享"
	case p.Number == 554 || p.Number == 8554 || p.Number == 10554 || p.Service == "rtsp":
		return "视频流"
	case p.Number == 3389:
		return "远程桌面"
	case p.Number == 9100 || p.Number == 515 || p.Number == 631 || p.Service == "printer" || p.Service == "ipp":
		return "打印服务"
	case p.Number == 1433 || p.Number == 3306 || p.Number == 5432 || p.Number == 6379 ||
		p.Service == "mssql" || p.Service == "mysql" || p.Service == "postgresql" || p.Service == "redis":
		return "数据库"
	case p.Number == 53 || p.Service == "dns":
		return "域名服务"
	case p.Number == 1900 || p.Service == "upnp":
		return "设备发现"
	}
	return "其他服务"
}
