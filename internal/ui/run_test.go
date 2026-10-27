package ui

import (
	"net/netip"
	"testing"
)

// TestToolRun: the shared run bookkeeping every tool uses — one current
// generation, a cancel that belongs to it, and a stop that retires both.
func TestToolRun(t *testing.T) {
	var r toolRun
	a, b := ip("192.168.1.5"), ip("192.168.1.6")

	cancelled := false
	genA := r.start(a, func() { cancelled = true })
	if !r.running() || !r.forHost(a) {
		t.Fatal("a started run is not running for its host")
	}
	if !r.live(genA) {
		t.Error("the newest generation is not live")
	}

	// A new run retires the old one's answers.
	genB := r.start(b, nil)
	if r.live(genA) {
		t.Error("the replaced generation is still live")
	}
	if !r.live(genB) || !r.forHost(b) {
		t.Error("the new run is not current")
	}

	// A run that ends on its own leaves its results showing.
	r.finish()
	if r.running() {
		t.Error("a finished run still reads as running")
	}
	if !r.forHost(b) {
		t.Error("finishing cleared the host the results belong to")
	}

	// Stopping cancels and clears.
	r.start(b, func() { cancelled = true })
	r.stop()
	if r.running() {
		t.Error("a stopped run still reads as running")
	}
	if !cancelled {
		t.Error("stop did not cancel its context")
	}
	if r.live(genB) {
		t.Error("a stopped run still answers its old generation")
	}

	// Clearing forgets the host; an empty run belongs to nobody.
	r.clear()
	if r.forHost(b) || r.ip.IsValid() {
		t.Error("clear left the run's host behind")
	}
	if r.forHost(netip.Addr{}) {
		t.Error("an empty run claims the zero address")
	}
}
