package ui

import "github.com/egoist/mygo/ui"

// appTheme is the fixed light WeChat-styled theme: light gray ground,
// white panels, WeChat green as the accent. Set on every frame so a dark
// system never bleeds through.
func appTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f7f7f7")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f0f0f0")
	t.SurfacePressed = ui.Hex("#e4e4e4")
	t.Border = ui.Hex("#e3e3e3")
	t.Text = ui.Hex("#1a1a1a")
	t.TextMuted = ui.Hex("#8a8a8a")
	t.Accent = ui.Hex("#07c160")
	t.AccentHover = ui.Hex("#06ad56")
	t.AccentPressed = ui.Hex("#059b4d")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#fa5151")
	t.Warning = ui.Hex("#ffc300")
	t.Success = ui.Hex("#07c160")
	t.Selection = ui.RGBA(7, 193, 96, 0.22)
	t.Focus = ui.RGBA(7, 193, 96, 0.5)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.22)
	t.Radius = 6
	// 4.5 puts table rows at 36px while keeping controls compact.
	t.Spacing = 4.5
	return t
}
