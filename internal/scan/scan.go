// Package scan implements the LAN discovery engine: adapter
// enumeration, host sweep, ARP lookup, vendor/hostname resolution,
// TCP port scanning and continuous ping.
//
// The public API is the contract the UI codes against; it must
// stay stable. The Windows API layer is built directly on
// iphlpapi.dll via the stdlib syscall package (no golang.org/x/sys in
// the module graph), so the package only builds on windows.
package scan

import "lslan/internal/model"

// EventKind labels one Event from a Scanner.
type EventKind int

const (
	EventProgress EventKind = iota // Progress is set
	EventHost                      // Host is set (state just resolved)
	EventDone                      // sweep finished, Progress is final
	EventError                     // Err is set, sweep aborted
)

// Event is one message on a Scanner's event stream.
type Event struct {
	Kind     EventKind
	Progress model.Progress
	Host     *model.Host
	Err      error
}
