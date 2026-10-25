//go:build windows

package scan

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"lslan/internal/model"
)

const (
	pingInterval = time.Second
	pingTimeout  = time.Second
)

// PingSession is one continuous ping to a single address.
type PingSession struct {
	cancel  context.CancelFunc
	samples chan model.PingSample
}

// StartPing begins pinging ip roughly once per second until Cancel or
// ctx cancellation, streaming each reply or timeout on Samples.
func StartPing(ctx context.Context, ip netip.Addr) (*PingSession, error) {
	if !ip.IsValid() || !ip.Is4() {
		return nil, errInvalidAddress
	}
	if !icmpAvailable() {
		return nil, errors.New("scan: ICMP unavailable on this system")
	}
	cctx, cancel := context.WithCancel(ctx)
	samples := make(chan model.PingSample, 16)
	go func() {
		defer cancel()
		defer close(samples)
		for seq := 1; ; seq++ {
			rtt, ttl, ok := icmpPing(ip, pingTimeout)
			s := model.PingSample{Seq: seq, OK: ok}
			if ok {
				s.RTT = rtt
				s.TTL = ttl
			}
			select {
			case samples <- s:
			case <-cctx.Done():
				return
			}
			t := time.NewTimer(pingInterval)
			select {
			case <-t.C:
			case <-cctx.Done():
				t.Stop()
				return
			}
		}
	}()
	return &PingSession{cancel: cancel, samples: samples}, nil
}

// Samples returns the reply stream. It closes when the session ends.
func (p *PingSession) Samples() <-chan model.PingSample { return p.samples }

// Cancel stops the session. Samples closes shortly after.
func (p *PingSession) Cancel() { p.cancel() }
