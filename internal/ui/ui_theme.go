package ui

import "github.com/egoist/mygo/ui"

// appTheme is the Fluent 2 look in the two appearances: light a cool
// paper ground with white cards, dark the Windows 11 charcoal ground with
// raised cards; both carry the Windows 11 accent blue, light's deep and
// dark's pale with ink on it. Set on every frame so nothing bleeds
// through from the other appearance.
func appTheme(dark bool) *ui.Theme {
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
	}
	if dark {
		t.Background = ui.Hex("#202020")
		t.Surface = ui.Hex("#2b2b2b")
		t.SurfaceHover = ui.Hex("#3a3d43")
		t.SurfacePressed = ui.Hex("#474a51")
		t.Border = ui.Hex("#6b7178")
		t.Text = ui.Hex("#ffffff")
		t.TextMuted = ui.Hex("#a8b0ba")
		t.Accent = ui.Hex("#4cc2ff")
		t.AccentHover = ui.Hex("#62cdff")
		t.AccentPressed = ui.Hex("#2ab3ef")
		t.AccentText = ui.Hex("#17191c")
		t.Danger = ui.Hex("#ff99a4")
		t.Warning = ui.Hex("#fcbb4a")
		t.Success = ui.Hex("#6ccb5f")
		t.Selection = ui.RGBA(76, 194, 255, 0.28)
		t.Focus = ui.Hex("#4cc2ff")
		t.Scrollbar = ui.RGBA(255, 255, 255, 0.38)
	} else {
		t.Background = ui.Hex("#f2f4f7")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#dfe5ec")
		t.SurfacePressed = ui.Hex("#d5dce6")
		t.Border = ui.Hex("#8a95a5")
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
		t.Focus = ui.Hex("#005fb8")
		t.Scrollbar = ui.RGBA(0, 0, 0, 0.42)
	}
	t.Radius = 4
	// 4.5 puts table rows at 36px while keeping controls compact.
	t.Spacing = 4.5
	t.FontSize = 13
	// Toasts ride the theme's inverse pair; Fluent's toast layer is the
	// same dark acrylic in both appearances, so a dark one never sits as
	// a white flash on the charcoal ground. In dark the card is already
	// that charcoal, so the toast lifts a step above it instead.
	t.Inverse = ui.Hex("#2b2b2b")
	if t.Dark {
		t.Inverse = ui.Hex("#45484d")
	}
	t.InverseText = ui.Hex("#ffffff")
	return t
}

// The extras below are the colors the theme type has no field for, each
// in both appearances.

// cardBorder and cardShadow lift a card off the ground: a hairline edge
// and a shadow soft enough to read as a layer, not a drop. Dark cards
// sit lower, so their shadow is deeper.
func cardBorder(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#3a3d42")
	}
	return ui.Hex("#e6eaef")
}

func cardShadow(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0, 0, 0, 0.32)
	}
	return ui.RGBA(16, 24, 40, 0.06)
}

// chosenRow is the fill of a chosen table row. Fluent tints the row with
// the accent and leaves the text alone, where the list's own choice floods
// it, so the table is built with this as its accent. The tint is deep
// enough that the row's muted text and badges keep their contrast on it.
func chosenRow(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#2f4f63")
	}
	return ui.Hex("#cfe2fa")
}

// chosenMuted is TextMuted for text on a chosen row, which the plain one
// is too faint for.
func chosenMuted(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#b9c2cc")
	}
	return ui.Hex("#4d5561")
}

// iconInk is the ink of a drawn icon on a quiet button.
func iconInk(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#c8c8c8")
	}
	return ui.Hex("#5c5c5c")
}

// gatewayPill is the 网关 badge, in the accent as a fill.
func gatewayPill(t *ui.Theme) (bg, fg ui.Color) {
	if t.Dark {
		return ui.Hex("#4cc2ff"), ui.Hex("#17191c")
	}
	return ui.Hex("#005fb8"), ui.Hex("#ffffff")
}

// softPill is the quieter 服务发现 badge, a wash of the accent; on the
// chosen row it takes a deeper wash so it does not vanish into the tint.
func softPill(t *ui.Theme) (bg, fg ui.Color) {
	if t.Dark {
		return ui.Hex("#0c3b58"), ui.Hex("#99e0ff")
	}
	return ui.Hex("#eef4fb"), ui.Hex("#0f548c")
}
