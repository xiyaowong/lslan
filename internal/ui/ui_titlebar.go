package ui

import "github.com/egoist/mygo/ui"

// titleBar draws the app's name into the native title bar strip, after the
// window icon Windows draws there itself; the rest of the strip drags the
// window (double click maximizes). The minimize/maximize/close buttons are
// the window's own, kept by TitleBarHidden; the view only keeps clear of
// their room.
func (a *app) titleBar(c *ui.Context, t *ui.Theme) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 44)).Shrink(0).AlignItems(ui.Center).
		Padding(0, bar.Right+12, 0, 16).DragWindow().Children(func() {
		ui.Text(c, "局域网 IP 扫描").FontSize(12.5).FontWeight(600).TextColor(t.TextMuted)
	})
}
