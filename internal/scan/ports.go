package scan

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"lslan/internal/model"
)

var errInvalidAddress = errors.New("scan: invalid IPv4 address")

// Preset is a named port list for scenario-based scanning.
type Preset struct {
	Name        string // e.g. "摄像头", "路由器", "NAS"
	Description string
	Ports       []int
}

// PortPresets returns the built-in port presets.
func PortPresets() []Preset {
	// Fresh slices every call so the caller cannot mutate shared state.
	return []Preset{
		{Name: "常用", Description: "常用服务端口", Ports: []int{
			21, 22, 23, 53, 80, 110, 135, 139, 443, 445, 1433, 3306, 3389, 5432, 5555, 8080, 8443, 9100,
		}},
		{Name: "摄像头", Description: "网络摄像头与录像机端口", Ports: []int{
			80, 443, 554, 1935, 8000, 8080, 8554, 8899, 10554, 34567, 49152,
		}},
		{Name: "路由器", Description: "路由器管理端口", Ports: []int{
			53, 80, 443, 1900, 5000, 7547, 8080, 8443, 9000,
		}},
		{Name: "NAS", Description: "NAS 存储设备端口", Ports: []int{
			21, 22, 139, 445, 2049, 3260, 5000, 5001, 5005, 5006, 8080, 8090, 8443, 9090,
		}},
	}
}

var serviceNames = map[int]string{
	21:    "ftp",
	22:    "ssh",
	23:    "telnet",
	25:    "smtp",
	53:    "dns",
	69:    "tftp",
	80:    "http",
	110:   "pop3",
	111:   "rpcbind",
	123:   "ntp",
	135:   "msrpc",
	137:   "netbios-ns",
	138:   "netbios-dgm",
	139:   "netbios-ssn",
	143:   "imap",
	161:   "snmp",
	389:   "ldap",
	443:   "https",
	445:   "smb",
	515:   "printer",
	548:   "afp",
	554:   "rtsp",
	587:   "smtp",
	631:   "ipp",
	993:   "imaps",
	995:   "pop3s",
	1080:  "socks",
	1433:  "mssql",
	1521:  "oracle",
	1900:  "upnp",
	1935:  "rtmp",
	2049:  "nfs",
	2222:  "ssh",
	3260:  "iscsi",
	3306:  "mysql",
	3389:  "rdp",
	5000:  "http-alt",
	5001:  "https-alt",
	5005:  "http-alt",
	5006:  "https-alt",
	5432:  "postgresql",
	5555:  "adb",
	5900:  "vnc",
	6379:  "redis",
	7547:  "cwmp",
	8000:  "http-alt",
	8009:  "ajp",
	8080:  "http-alt",
	8081:  "http-alt",
	8090:  "http-alt",
	8443:  "https-alt",
	8554:  "rtsp",
	8899:  "rtsp-alt",
	9000:  "http-alt",
	9090:  "http-alt",
	9100:  "printer",
	10554: "rtsp",
	34567: "dvr",
	49152: "rpc",
}

const (
	portTimeout = 500 * time.Millisecond
	portWorkers = 32
)

// ScanPorts TCP-connect-probes the given ports on ip, returning the
// results sorted by port number (open first is NOT applied here).
// It blocks; ctx cancellation stops it.
func ScanPorts(ctx context.Context, ip netip.Addr, ports []int) ([]model.Port, error) {
	return ScanPortsFrom(ctx, netip.Addr{}, ip, ports)
}

// ScanPortsFrom is ScanPorts with the connections made from src, the
// adapter's own address. Binding the source matters wherever a VPN's TUN
// adapter routes the LAN too: an unbound dial is then taken by the tunnel,
// which accepts every connection at once, and every port of every address
// reads as open. LAN traffic bound to the adapter's address goes out of
// it, so the answer is the device's.
func ScanPortsFrom(ctx context.Context, src, ip netip.Addr, ports []int) ([]model.Port, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !ip.IsValid() {
		return nil, errInvalidAddress
	}
	results := make([]model.Port, len(ports))
	sem := make(chan struct{}, portWorkers)
	var wg sync.WaitGroup
	for i, p := range ports {
		wg.Add(1)
		go func(i, p int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			c, err := dialFrom(ctx, src, ip, p, portTimeout)
			if err == nil {
				c.Close()
			}
			results[i] = model.Port{Number: p, Service: serviceNames[p], Open: err == nil}
		}(i, p)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(results, func(a, b model.Port) int { return a.Number - b.Number })
	return results, nil
}

// dialFrom opens one TCP connection to ip:port, from src when it is a
// usable local address: a tunnel adapter would otherwise take the dial.
func dialFrom(ctx context.Context, src, ip netip.Addr, port int, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	if src.IsValid() && src.Is4() && !src.IsUnspecified() && !src.IsLoopback() {
		d.LocalAddr = &net.TCPAddr{IP: src.AsSlice()}
	}
	return d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
}
