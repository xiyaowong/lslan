package ui

import (
	"math"
	"testing"
)

// contrast is the WCAG 2.1 ratio of two opaque colors, 1:1 to 21:1.
func contrast(a, b [3]uint8) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relLum(c [3]uint8) float64 {
	ch := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c[0]) + 0.7152*ch(c[1]) + 0.0722*ch(c[2])
}

var (
	white      = [3]uint8{255, 255, 255}
	lightCard  = [3]uint8{0xff, 0xff, 0xff}
	lightHover = [3]uint8{0xdf, 0xe5, 0xec}
	lightEdge  = [3]uint8{0x8a, 0x95, 0xa5}
	darkCard   = [3]uint8{0x2b, 0x2b, 0x2b}
	darkHover  = [3]uint8{0x3a, 0x3d, 0x43}
	darkEdge   = [3]uint8{0x6b, 0x71, 0x78}
)

// TestThemeContrast: the borders, hovers and focus rings are visible. The
// old light border sat at 1.24:1 against the card and the hover at 1.08:1,
// so controls had no edge and flat buttons gave no sign of being hovered.
func TestThemeContrast(t *testing.T) {
	cases := []struct {
		name    string
		a, b    [3]uint8
		atLeast float64
	}{
		{"light border on card", lightEdge, lightCard, 3.0},
		{"light hover on card", lightHover, lightCard, 1.2},
		{"dark border on card", darkEdge, darkCard, 1.5},
		{"dark hover on card", darkHover, darkCard, 1.2},
		{"dark muted on chosen row", [3]uint8{0xb9, 0xc2, 0xcc}, [3]uint8{0x2f, 0x4f, 0x63}, 4.5},
		{"light muted on chosen row", [3]uint8{0x4d, 0x55, 0x61}, [3]uint8{0xcf, 0xe2, 0xfa}, 4.5},
	}
	for _, c := range cases {
		got := contrast(c.a, c.b)
		t.Logf("%-26s %.2f:1 (want >= %.1f)", c.name, got, c.atLeast)
		if got < c.atLeast {
			t.Errorf("%s = %.2f:1, want at least %.1f:1", c.name, got, c.atLeast)
		}
	}
}
