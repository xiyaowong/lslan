// Package model defines the shared data types exchanged between the
// scanning engine (internal/scan) and the UI (package main).
package model

import (
	"net/netip"
	"time"
)

// NIC describes one usable IPv4 network adapter.
type NIC struct {
	Index     int
	Name      string // human-readable adapter name
	IP        netip.Addr
	PrefixLen int
	MAC       string
	Gateway   netip.Addr
	Virtual   bool // TUN/TAP/tunnel rather than physical LAN
}

// HostState is the reachability of a host within the last sweep.
type HostState int

const (
	StateUnknown HostState = iota
	StateOnline
	StateOffline
)

// Port is the result of probing one TCP port.
type Port struct {
	Number  int
	Service string // well-known service name, e.g. "http"
	Open    bool
}

// Service is one service a host advertises over the network's service
// discovery (mDNS). Name is the DNS-SD type, e.g. "_http._tcp".
type Service struct {
	Name string
	Port int
}

// Host is one address in the scanned range.
type Host struct {
	IP       netip.Addr
	State    HostState
	Hostname string
	MAC      string
	Vendor   string
	Ports    []Port
	Services []Service
	Gateway  bool
	ViaMDNS  bool // found through service discovery, not a probe
	LastSeen time.Time
}

// LastOctet returns the last octet of the IPv4 address, for the
// "主机号" table column.
func (h Host) LastOctet() int {
	if !h.IP.Is4() {
		return 0
	}
	b := h.IP.As4()
	return int(b[3])
}

// Progress is the running tally of a sweep.
type Progress struct {
	Scanned int
	Total   int
	Online  int
}

// PingSample is one reply (or timeout) of a continuous ping.
type PingSample struct {
	Seq int
	OK  bool
	RTT time.Duration
	TTL int
}
