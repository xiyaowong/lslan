package ui

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// telnetTermIface lets tests fake the terminal plugin.
type telnetTermIface interface {
	view(c *ui.Context) ui.Element
	close()
	setDark(dark bool)
}

// telnetTerm is a live session with a device's telnet port, shown in the
// terminal plugin: what the device sends streams in, what the user types
// goes out, as in a console there.
type telnetTerm struct {
	term *terminal.Terminal
}

// buildTelnetTerm wraps an open connection in the terminal view; the
// terminal reads the device's output from the link and writes the user's
// keys to it. Built on the main thread, as the ping terminal is.
func buildTelnetTerm(conn net.Conn) (telnetTermIface, error) {
	t, err := terminal.New(terminal.Options{
		Conn:       &telnetLink{Conn: conn},
		Scrollback: 1 << 20,
		Font:       terminal.Font{Size: 12},
	})
	if err != nil {
		return nil, err
	}
	return &telnetTerm{term: t}, nil
}

func (p *telnetTerm) view(c *ui.Context) ui.Element { return terminal.View(c, p.term) }
func (p *telnetTerm) close()                        { p.term.Close() }
func (p *telnetTerm) setDark(dark bool)             { p.term.UseTheme(terminalTheme(dark)) }

// telnetLink is the device's end of the session in the terminal's hands:
// the keys pass through as typed, while the output is cleaned of the
// telnet protocol's negotiation, which would otherwise paint as garbage.
// The usual pair a telnet server offers — it echoes (ECHO) and may send
// without waiting (SGA) — is taken up; everything else is declined, and
// what the server agrees not to do needs no answer. When the device
// hangs up, a farewell line stands in for the silence.
type telnetLink struct {
	net.Conn
	// where a negotiation cut in half by a read boundary waits, the
	// command it belongs to, and whether the farewell has been said.
	state   int
	reply   byte
	saidBye bool
}

// Negotiation parsing states.
const (
	telnetData   = iota // in plain output
	telnetIAC           // after IAC, the command byte next
	telnetOpt           // after WILL/WONT/DO/DONT, the option byte next
	telnetSub           // in a subnegotiation, until IAC SE
	telnetSubIAC        // in one, after its IAC
)

// Telnet protocol bytes.
const (
	iacByte = 255
	sbByte  = 250
	seByte  = 240
	will    = 251
	wont    = 252
	doByte  = 253
	dont    = 254
	optEcho = 1
	optSGA  = 3
)

var telnetBye = []byte("\r\n[连接已断开]\r\n")

func (l *telnetLink) Read(b []byte) (int, error) {
	for {
		n, err := l.Conn.Read(b)
		if n > 0 {
			out := l.filter(b[:n])
			if len(out) > 0 {
				copy(b, out)
				return len(out), nil
			}
			// A read that was all negotiation: keep reading rather than
			// return nothing, which callers may take for a hangup.
			if err == nil {
				continue
			}
		}
		if err != nil {
			if !l.saidBye {
				l.saidBye = true
				copy(b, telnetBye)
				return len(telnetBye), nil
			}
			return 0, err
		}
	}
}

// filter strips this round's negotiation from p, answering it, and
// returns what is left as output; the parsing state carries across
// reads, as a sequence may be split at any byte.
func (l *telnetLink) filter(p []byte) []byte {
	out := p[:0]
	cr := false // the last output byte was CR, a NUL may follow it
	for _, c := range p {
		switch l.state {
		case telnetData:
			if c == iacByte {
				l.state = telnetIAC
				continue
			}
			// Telnet sends CR NUL where a terminal wants CR; drop the NUL.
			if cr && c == 0 {
				cr = false
				continue
			}
			cr = c == '\r'
			out = append(out, c)
		case telnetIAC:
			switch {
			case c == iacByte: // IAC IAC is the data byte 255
				out = append(out, c)
				l.state = telnetData
			case c == sbByte:
				l.state = telnetSub
			case c >= will && c <= dont:
				l.state = telnetOpt
				l.reply = c
			default: // NOP, GA and the other one-byte commands
				l.state = telnetData
			}
		case telnetOpt:
			l.answer(c)
			l.state = telnetData
		case telnetSub:
			if c == iacByte {
				l.state = telnetSubIAC
			}
		case telnetSubIAC:
			// SE ends the subnegotiation; IAC IAC is an embedded 255,
			// and anything else restarts it.
			if c == seByte {
				l.state = telnetData
			} else {
				l.state = telnetSub
			}
		}
	}
	return out
}

// answer replies to WILL x / DO x etc., carrying the command in l.reply.
func (l *telnetLink) answer(opt byte) {
	switch l.reply {
	case will:
		if opt == optEcho || opt == optSGA {
			l.writeCmd(doByte, opt) // taken up
		} else {
			l.writeCmd(dont, opt)
		}
	case doByte:
		l.writeCmd(wont, opt) // we offer no options of our own
	} // WONT and DONT need no answer
}

func (l *telnetLink) writeCmd(cmd, opt byte) {
	_, _ = l.Conn.Write([]byte{iacByte, cmd, opt})
}

// startTelnet opens the one-click test of a telnet port (#54): the
// device is dialed off the interface so a slow or dead one never freezes
// it, and the session lands in the panel's terminal pane when it answers.
func (a *app) startTelnet(ip netip.Addr, port int) {
	a.startTool()
	a.telnetPort = port
	a.telnetPending, a.telnetErr = true, ""
	gen := a.telnet.start(ip, nil)
	go func() {
		conn, err := a.dialTelnet(ip, port)
		a.dispatch(func() {
			if !a.telnet.live(gen) {
				if conn != nil {
					conn.Close()
				}
				return // ended while dialing
			}
			a.telnetPending = false
			if err != nil {
				a.telnetErr = telnetDialError(err, ip, port)
				return
			}
			term, err := a.newTelnet(conn)
			if err != nil {
				conn.Close()
				a.telnetErr = "打不开实时输出窗口，无法测试"
				return
			}
			a.telnetTerm = term
			term.setDark(a.dark)
		})
	}()
}

// telnetDialError turns a failed dial into words a home user can act on:
// a timeout means nothing answered, a refusal means the port is closed,
// and anything else is a guess.
func telnetDialError(err error, ip netip.Addr, port int) string {
	where := fmt.Sprintf("%s:%d", ip, port)
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return where + " 没有响应，设备可能已关机"
	case errors.Is(err, syscall.ECONNREFUSED):
		return where + " 拒绝了连接，远程登录可能没有开启"
	default:
		return "连接 " + where + " 失败，设备可能没开远程登录"
	}
}

// stopTelnet ends the test and clears its pane; safe with none running.
func (a *app) stopTelnet() {
	a.telnet.clear()
	if a.telnetTerm != nil {
		a.telnetTerm.close()
		a.telnetTerm = nil
	}
	a.telnetPending = false
	a.telnetErr = ""
}
