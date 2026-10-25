package ui

import "github.com/egoist/mygo/ui"

// titleBar replaces the system one: app name on the left, window buttons
// on the right, the whole strip drags the window (double click maximizes).
func (a *app) titleBar(c *ui.Context) {
	ui.Row(c).Height(40).Shrink(0).AlignItems(ui.Center).Padding(0, 0, 0, 14).DragWindow().Children(func() {
		ui.Text(c, "局域网 IP 扫描").FontSize(13).FontWeight(600).DragWindow()
		ui.Box(c).Grow(1)
		a.captionButton(c, "min")
		a.captionButton(c, "max")
		a.captionButton(c, "close")
	})
}

// captionButton draws one window control. With a.win nil (tests) it is
// left out.
func (a *app) captionButton(c *ui.Context, kind string) {
	if a.win == nil {
		return
	}
	t := c.Theme()
	b := ui.Box(c).Size(44, 40).Shrink(0)
	hoverBg, glyph := t.SurfacePressed, t.TextMuted
	if kind == "close" {
		hoverBg, glyph = t.Danger, t.Text
	}
	if b.Hovered() {
		b.Background(hoverBg)
		if kind == "close" {
			glyph = t.AccentText
		} else {
			glyph = t.Text
		}
	}
	maximized := kind == "max" && a.win.IsMaximized()
	b.Draw(func(p *ui.Painter, r ui.Rect) { drawCaptionGlyph(p, r, kind, maximized, glyph) })
	switch kind {
	case "min":
		if b.Clicked() {
			a.win.Minimize()
		}
	case "max":
		b.MaximizeButton()
		if b.Clicked() {
			a.win.ToggleMaximize()
		}
	case "close":
		if b.Clicked() {
			a.win.Close()
		}
	}
}

// drawCaptionGlyph paints the minimize, maximize/restore or close mark.
func drawCaptionGlyph(p *ui.Painter, r ui.Rect, kind string, maximized bool, col ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	var path ui.Path
	switch kind {
	case "min":
		p.Fill(ui.Rect{X: cx - 4.5, Y: cy - 0.5, W: 9, H: 1}, col, 0)
	case "max":
		if maximized {
			// Back pane, then the front one lower left.
			path.MoveTo(cx-2, cy-3.5).LineTo(cx+3.5, cy-3.5).LineTo(cx+3.5, cy+1.5)
			p.StrokePath(&path, 1, col)
			path.Reset()
			rectPath(&path, cx-3.5, cy-1.5, 7, 6)
			p.StrokePath(&path, 1, col)
			return
		}
		rectPath(&path, cx-4, cy-3.5, 8, 7)
		p.StrokePath(&path, 1, col)
	case "close":
		path.MoveTo(cx-3.5, cy-3.5).LineTo(cx+3.5, cy+3.5)
		p.StrokePath(&path, 1.25, col)
		path.Reset()
		path.MoveTo(cx+3.5, cy-3.5).LineTo(cx-3.5, cy+3.5)
		p.StrokePath(&path, 1.25, col)
	}
}

func rectPath(path *ui.Path, x, y, w, h float32) {
	path.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h).LineTo(x, y+h).LineTo(x, y)
}
