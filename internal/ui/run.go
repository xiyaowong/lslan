package ui

import (
	"context"
	"net/netip"
)

// toolRun is the bookkeeping every background tool shares: which address
// it belongs to, whether it is still going, and a generation that drops
// the answers of a run the user has since replaced. Each tool keeps its
// own output beside it — a ping line and a tracert hop are not the same
// thing — so only the part that is genuinely identical lives here.
type toolRun struct {
	ip     netip.Addr
	cancel context.CancelFunc
	gen    int
}

// start takes a fresh generation for a run against ip and returns it;
// answers tagged with an older one are dropped by live.
func (r *toolRun) start(ip netip.Addr, cancel context.CancelFunc) int {
	r.gen++
	r.ip = ip
	r.cancel = cancel
	return r.gen
}

// live reports whether gen is still the current run.
func (r *toolRun) live(gen int) bool { return r.gen == gen }

// running reports whether a run is in flight.
func (r *toolRun) running() bool { return r.cancel != nil }

// stop ends the run: a later answer carries a stale generation, and the
// context cancels whatever the run was doing.
func (r *toolRun) stop() {
	r.gen++
	if r.cancel != nil {
		cancel := r.cancel
		r.cancel = nil
		cancel()
	}
}

// finish marks a run that ended on its own: nothing is left to cancel,
// but the generation stays, so the pane keeps showing its results.
func (r *toolRun) finish() { r.cancel = nil }

// clear forgets the run and its address, for a pane being put away.
func (r *toolRun) clear() {
	r.stop()
	r.ip = netip.Addr{}
}

// forHost reports whether the run belongs to ip.
func (r *toolRun) forHost(ip netip.Addr) bool { return r.ip == ip && r.ip.IsValid() }
