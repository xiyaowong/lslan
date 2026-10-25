package ui

import (
	"io"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// pingTerminalIface lets tests fake the terminal plugin.
type pingTerminalIface interface {
	// feed shows one ping output line; !ok renders it in red.
	feed(line string, ok bool)
	view(c *ui.Context) ui.Element
	close()
}

// newTerminal builds a real terminal plugin view; tests replace it.
func newTerminal() (pingTerminalIface, error) { return newPingTerminal() }

// pingTerminal shows the ping session's output in a terminal plugin view,
// fed line by line as if a program had written it; no shell runs.
type pingTerminal struct {
	conn *pingConn
	term *terminal.Terminal
}

func newPingTerminal() (*pingTerminal, error) {
	p := &pingTerminal{conn: newPingConn()}
	t, err := terminal.New(terminal.Options{
		Conn:       p.conn,
		Scrollback: 1 << 20,
		Font:       terminal.Font{Size: 12},
	})
	if err != nil {
		return nil, err
	}
	p.term = t
	return p, nil
}

func (p *pingTerminal) feed(line string, ok bool) {
	if !ok {
		p.term.Feed([]byte("\x1b[31m" + line + "\x1b[0m\r\n"))
		return
	}
	p.term.Feed([]byte(line + "\r\n"))
}

func (p *pingTerminal) view(c *ui.Context) ui.Element {
	return terminal.View(c, p.term)
}

func (p *pingTerminal) close() { p.term.Close() }

// pingConn stands in for the ping session's program end of the terminal:
// the terminal reads output from Read; keys the user types arrive at
// Write and go nowhere.
type pingConn struct {
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

func newPingConn() *pingConn {
	p := &pingConn{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pingConn) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for !p.closed {
		p.cond.Wait()
	}
	return 0, io.EOF
}

func (p *pingConn) Write(b []byte) (int, error) { return len(b), nil }

func (p *pingConn) Close() error {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
	return nil
}
