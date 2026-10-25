package ui

import "github.com/egoist/mygo/ui"

// titleBar draws the app's identity into the native title bar strip: the
// icon and the name, level with the cards' left edge; the rest of the
// strip drags the window (double click maximizes). The
// minimize/maximize/close buttons are the window's own, kept by
// TitleBarHidden; the view only keeps clear of their room.
func (a *app) titleBar(c *ui.Context, t *ui.Theme) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 44)).Shrink(0).AlignItems(ui.Center).
		Padding(0, bar.Right+12, 0, 18).DragWindow().Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.logo != nil {
				ui.Image(c, a.logo).Size(16, 16).Radius(4).Shrink(0)
			}
			ui.Text(c, "局域网 IP 扫描").FontSize(12.5).FontWeight(600).TextColor(t.Text)
		})
	})
}
