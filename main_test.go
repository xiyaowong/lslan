package main

import "testing"

// TestWindowSize: the window takes its preferred size on a screen with the
// room, shares a smaller one, and never asks for more than the screen has.
func TestWindowSize(t *testing.T) {
	cases := []struct {
		name           string
		availW, availH int
		wantW, wantH   int
	}{
		{"large screen", 2560, 1400, prefWidth, prefHeight},
		{"1080p", 1920, 1040, prefWidth, prefHeight},
		{"laptop 1366x768", 1366, 728, 1040, 626},
		{"small 1024x600", 1024, 560, 860, 520},
		{"tiny 800x480", 800, 480, 800, 480},
	}
	for _, c := range cases {
		w, h := windowSize(c.availW, c.availH)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: windowSize(%d, %d) = %dx%d, want %dx%d",
				c.name, c.availW, c.availH, w, h, c.wantW, c.wantH)
		}
		if w > c.availW || h > c.availH {
			t.Errorf("%s: window %dx%d is larger than the screen %dx%d", c.name, w, h, c.availW, c.availH)
		}
	}
}
