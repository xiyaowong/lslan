package ui

import "github.com/egoist/mygo/ui"

// titleBar draws into the native title bar strip: logo and app name on
// the left, the strip drags the window (double click maximizes). The
// minimize/maximize/close buttons are the window's own, kept by
// TitleBarHidden; the view only keeps clear of their room.
func (a *app) titleBar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 44)).Shrink(0).AlignItems(ui.Center).
		Padding(0, bar.Right+12, 0, 14).DragWindow().Children(func() {
		if a.logo != nil {
			ui.Image(c, a.logo).Size(18, 18).Shrink(0)
		}
		ui.Text(c, "局域网 IP 扫描").FontSize(12.5).FontWeight(600)
	})
}
