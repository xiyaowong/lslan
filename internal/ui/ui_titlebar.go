package ui

import "github.com/egoist/mygo/ui"

// The caption buttons' glyphs, drawn as strokes so no icon font is needed.
// ui.Icon paints an SVG's shapes in the text color, whatever colors the
// document has.
var (
	minimizeSVG = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M.5 5h9" stroke="#000" stroke-width="1"/></svg>`))
	closeSVG = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M.7.7l8.6 8.6M9.3.7l-8.6 8.6" stroke="#000" stroke-width="1" stroke-linecap="round"/></svg>`))
)

// The caption is 44 high (TitleBarHeight); Fluent caption buttons are 46
// wide on it, quiet until hovered, the close button alone in the danger
// red — the system's own caption would have shown a disabled maximize
// button where the app has none (#47).
const captionBtnW = 46

// titleBar draws the app's own caption: the icon and the name at the left,
// the drag strip between, and minimize and close buttons at the right, the
// window's own caption being hidden (HideWindowButtons).
func (a *app) titleBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(44).Shrink(0).DragWindow().Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 0, 0, 18).Shrink(0).Children(func() {
			if a.logo != nil {
				ui.Image(c, a.logo).Size(16, 16).Radius(4).Shrink(0)
			}
			ui.Text(c, "局域网 IP 扫描").FontSize(12.5).FontWeight(600).TextColor(t.Text)
		})
		ui.Box(c).Grow(1)
		a.captionButton(c, t, minimizeSVG, "最小化", t.SurfaceHover, t.Text, a.minimize)
		a.captionButton(c, t, closeSVG, "关闭", ui.Hex("#c42b1c"), ui.Hex("#ffffff"), a.close)
	})
}

// captionButton is one window button: transparent until hovered, its glyph
// in the hover's ink. fn runs on click; win is nil in tests, where the
// buttons then do nothing.
func (a *app) captionButton(c *ui.Context, t *ui.Theme, glyph *ui.SVG, label string, hover, ink ui.Color, fn func()) {
	b := ui.Button(c, "")
	b.Width(captionBtnW).FillHeight().Padding(0).Radius(0).Border(0, ui.Color{}).
		AlignItems(ui.Center).Justify(ui.Center)
	bg, fg := ui.Color{}, t.Text
	if b.Pressed() || b.Hovered() {
		bg, fg = hover, ink
	}
	b.Background(bg).TextColor(fg)
	b.Children(func() { ui.Icon(c, glyph).Size(10, 10).Label(label) })
	if b.Clicked() && a.win != nil {
		fn()
	}
}

func (a *app) minimize() { a.win.Minimize() }
func (a *app) close()    { a.win.Close() }
