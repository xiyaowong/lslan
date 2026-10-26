package ui

import (
	"github.com/egoist/mygo/ui"
)

// smallButton and smallPrimary are the buttons of a dense row, one step
// smaller than the default.
func smallButton(c *ui.Context, label string) ui.Element {
	b := ui.Button(c, label).Padding(3, 10)
	return b
}

func smallPrimary(c *ui.Context, label string) ui.Element {
	return ui.PrimaryButton(c, label).Padding(3, 12)
}

// flatButton is a text button that shows no face until the pointer is over
// it, for a lesser action beside the others.
func flatButton(c *ui.Context, label string) ui.Element {
	b := ui.Button(c, label).Padding(3, 6)
	return flatFace(c, b)
}

// iconButton is a small square button showing only an icon, named for
// assistive technology and its tooltip. draw gets the ink of the current
// theme, so its glyph follows the appearance.
func iconButton(c *ui.Context, name string, draw func(p *ui.Painter, r ui.Rect, ink ui.Color)) ui.Element {
	b := ui.ButtonBase(c).Size(24, 24).Radius(4).Shrink(0).Label(name).Tooltip(name)
	ink := iconInk(c.Theme())
	b.Draw(func(p *ui.Painter, r ui.Rect) { draw(p, r, ink) })
	return flatFace(c, b)
}

// flatFace paints a button only under the pointer, the Fluent text and
// icon buttons.
func flatFace(c *ui.Context, b ui.Element) ui.Element {
	t := c.Theme()
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	default:
		b.Background(ui.Transparent)
	}
	return b
}

// drawClose draws an × of an icon button.
func drawClose(p *ui.Painter, r ui.Rect, ink ui.Color) {
	d := min(r.W, r.H) * 0.28
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	var path ui.Path
	path.MoveTo(cx-d, cy-d).LineTo(cx+d, cy+d).MoveTo(cx+d, cy-d).LineTo(cx-d, cy+d)
	p.StrokePath(&path, 1.3, ink)
}

// drawCopy draws two overlapping sheets, the copy of an icon button.
func drawCopy(p *ui.Painter, r ui.Rect, ink ui.Color) {
	s := min(r.W, r.H) * 0.26
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	var back, front ui.Path
	back.MoveTo(cx-s*0.6, cy-s).LineTo(cx+s, cy-s).LineTo(cx+s, cy+s*0.6)
	front.MoveTo(cx-s, cy-s*0.6).LineTo(cx+s*0.6, cy-s*0.6).
		LineTo(cx+s*0.6, cy+s).LineTo(cx-s, cy+s).Close()
	p.StrokePath(&back, 1.2, ink)
	p.StrokePath(&front, 1.2, ink)
}
