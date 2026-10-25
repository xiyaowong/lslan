package ui

import "github.com/egoist/mygo/ui"

// appTheme is the fixed light Fluent-styled theme: light mica-like ground,
// white panels, the Windows 11 blue as the accent, hard 4px corners. Set
// on every frame so a dark system never bleeds through. The system Mica
// material is not enabled: it follows the system's dark mode and would
// turn dark under the fixed light theme.
func appTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f3f3f3")
	t.Surface = ui.Hex("#fbfbfb")
	t.SurfaceHover = ui.Hex("#f0f0f0")
	t.SurfacePressed = ui.Hex("#e4e4e4")
	t.Border = ui.Hex("#e0e0e0")
	t.Text = ui.Hex("#1a1a1a")
	t.TextMuted = ui.Hex("#616161")
	t.Accent = ui.Hex("#0067c0")
	t.AccentHover = ui.Hex("#1a76d2")
	t.AccentPressed = ui.Hex("#005ba1")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#c42b1c")
	t.Warning = ui.Hex("#9d5d00")
	t.Success = ui.Hex("#0f7b0f")
	t.Selection = ui.RGBA(0, 103, 192, 0.2)
	t.Focus = ui.RGBA(0, 103, 192, 0.55)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.24)
	t.Radius = 4
	// 4.5 puts table rows at 36px while keeping controls compact.
	t.Spacing = 4.5
	return t
}
