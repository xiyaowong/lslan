package ui

import (
	"context"
	"fmt"
	"net/netip"

	"lslan/internal/model"
)

// stopPing ends the current session; its in-flight samples carry a stale
// generation and are dropped. Safe with no session running. The pane
// stays up with the results until closePing clears it.
func (a *app) stopPing() {
	a.ping.stop()
	a.stopTelnet() // the terminal pane holds one interactive session
	a.stopTrace()  // so does the route trace
	if a.pingTerm != nil {
		a.pingTerm.close()
		a.pingTerm = nil
	}
}

// startTool ends whatever tool is running and clears the panes the new
// one would share the foot with: one output at a time.
func (a *app) startTool() {
	a.stopPing() // ends the telnet test and the trace too
	a.closePing()
	a.closeTrace()
}

// closePing clears the finished session's pane entirely; the pane shows
// the ✕ only once no session runs, so this never ends a running one.
func (a *app) closePing() {
	a.ping.clear()
	a.pingLines = nil
	a.pingStats = pingStats{}
	if a.pingTerm != nil {
		a.pingTerm.close()
		a.pingTerm = nil
	}
}

// startPing pings ip, stopping any session still running. The other
// tools' panes close whole: the foot holds one output at a time, and
// two panes of lines would fight for the frame.
func (a *app) startPing(ip netip.Addr) {
	a.startTool()
	ctx, cancel := context.WithCancel(context.Background())
	gen := a.ping.start(ip, cancel)
	a.pingLines = nil
	a.pingStats = pingStats{}
	if term, err := a.newPingTerm(); err == nil {
		a.pingTerm = term
		term.setDark(a.dark)
		term.feed("正在 Ping "+ip.String()+":", true)
	} else {
		// No terminal: say so in the line list, which must then show every
		// line, this one included, before the first sample arrives.
		a.pingLines = append(a.pingLines, "打不开实时输出窗口，下面用文字显示结果")
	}
	sess, err := a.startPingFn(ctx, ip)
	if err != nil {
		a.pingLines = append(a.pingLines, "Ping 没能启动，请稍后再试")
		a.feedPing("Ping 没能启动，请稍后再试", false)
		a.ping.cancel = nil
		cancel()
		return
	}
	go func() {
		for s := range sess.Samples() {
			a.dispatch(func() {
				if a.ping.live(gen) {
					a.applyPingSample(s)
				}
			})
		}
		a.dispatch(func() {
			if a.ping.live(gen) {
				a.ping.cancel = nil
			}
		})
		cancel()
	}()
}

func (a *app) applyPingSample(s model.PingSample) {
	st := &a.pingStats
	st.sent++
	line, ok := "请求超时。", false
	if s.OK {
		st.recv++
		st.rttTotal += s.RTT
		line, ok = fmt.Sprintf("来自 %s: 字节=32 时间=%.1fms TTL=%d", a.ping.ip, s.RTT.Seconds()*1000, s.TTL), true
	}
	a.pingLines = append(a.pingLines, line)
	if len(a.pingLines) > 400 {
		a.pingLines = append([]string(nil), a.pingLines[len(a.pingLines)-300:]...)
	}
	a.feedPing(line, ok)
}

func (a *app) feedPing(line string, ok bool) {
	if a.pingTerm != nil {
		a.pingTerm.feed(line, ok)
	}
}
