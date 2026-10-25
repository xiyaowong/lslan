package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// rowMenu builds the context menu of a table cell: every item acts on the
// cell's row, and right-clicking also selects that row.
func (a *app) rowMenu(m *ui.Menu, row, col int) {
	if row < 0 || row >= len(a.menuHosts) {
		return
	}
	h := a.menuHosts[row]
	a.pickMenuRow(h, row)
	if m.Item("复制 IP").Chosen() {
		a.menuAction(func(c *ui.Context) { c.WriteClipboard(h.IP.String()) })
	}
	if m.Item("复制 MAC").Disabled(strings.TrimSpace(h.MAC) == "").Chosen() {
		a.menuAction(func(c *ui.Context) { c.WriteClipboard(h.MAC) })
	}
	m.Separator()
	if m.Item("Ping").Chosen() {
		a.startPing(h.IP)
	}
	if m.Item("端口扫描").Disabled(a.portScanning).Chosen() {
		a.startPortScan(h.IP)
	}
	if m.Item("浏览器打开").Chosen() {
		a.menuAction(func(c *ui.Context) { c.OpenURL("http://" + h.IP.String()) })
	}
}

// menuAction runs fn with the frame's context on the next frame, where
// WriteClipboard and OpenURL work.
func (a *app) menuAction(fn func(c *ui.Context)) {
	a.dispatch(func() { a.nextFrame = append(a.nextFrame, fn) })
}

// pickMenuRow selects the row the menu opened on; dispatch repaints with
// the new selection after this frame.
func (a *app) pickMenuRow(h model.Host, row int) {
	if a.selRow == row && a.selIP == h.IP {
		return
	}
	a.dispatch(func() {
		a.selRow, a.selIP = row, h.IP
	})
}

// runNextFrame runs the actions a menu choice queued, with the context of
// the frame now building.
func (a *app) runNextFrame(c *ui.Context) {
	fns := a.nextFrame
	a.nextFrame = nil
	for _, fn := range fns {
		fn(c)
	}
}
