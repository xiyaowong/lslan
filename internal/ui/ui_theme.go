package ui

import "github.com/egoist/mygo/ui"

// appTheme is the fixed light Fluent look: a cool paper ground, white
// cards and controls, the Windows 11 accent blue, 4px corners. Set on
// every frame so a dark system never bleeds through. The system Mica
// material is not enabled: it follows the system's dark mode and would
// turn dark under the fixed light theme.
func appTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f2f4f7")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f4f6f9")
	t.SurfacePressed = ui.Hex("#e9edf2")
	t.Border = ui.Hex("#e3e7ec")
	t.Text = ui.Hex("#17191c")
	t.TextMuted = ui.Hex("#5c6470")
	t.Accent = ui.Hex("#005fb8")
	t.AccentHover = ui.Hex("#1a6fc4")
	t.AccentPressed = ui.Hex("#004c94")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#c42b1c")
	t.Warning = ui.Hex("#9d5d00")
	t.Success = ui.Hex("#0f7b0f")
	t.Selection = ui.RGBA(0, 95, 184, 0.22)
	t.Focus = ui.RGBA(0, 95, 184, 0.6)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.22)
	t.Inverse = ui.Hex("#2b2b2b")
	t.InverseText = ui.Hex("#ffffff")
	t.Radius = 4
	// 4.5 puts table rows at 36px while keeping controls compact.
	t.Spacing = 4.5
	t.FontSize = 13
	return t
}

// cardBorder and cardShadow lift a card off the ground: a hairline edge
// and a shadow soft enough to read as a layer, not a drop.
var (
	cardBorder = ui.Hex("#e6eaef")
	cardShadow = ui.RGBA(16, 24, 40, 0.06)
)

// chosenRow is the fill of a chosen table row. Fluent tints the row with
// the accent and leaves the text alone, where the list's own choice floods
// it, so the table is built with this as its accent.
var chosenRow = ui.Hex("#dbe9fb")

// The ink of a drawn icon on a quiet button, and the accent, for parts
// painted inside a table built with another accent (chosenRow).
var (
	closeInk                 = ui.Hex("#5c5c5c")
	accentBlue               = ui.Hex("#005fb8")
	softPillBG, softPillText = ui.Hex("#eef4fb"), ui.Hex("#0f548c")
)
