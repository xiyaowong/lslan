// Command lslan is a Windows LAN IP scanner desktop app built on mygo.
package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo"

	"lslan/internal/ui"
)

//go:embed resources/icon.png
var iconPNG []byte

// Window geometry: a scanner's window, not a workspace's. 900 is the host
// card and the detail sidebar side by side with a little room to spare,
// 620 shows a dozen hosts above the status bar; the screen it opens on
// bounds both, for a small laptop.
const (
	prefWidth, prefHeight   = 900, 620
	minWidth, minHeight     = 860, 520
	widthShare, heightShare = 0.82, 0.86
)

func main() {
	a := ui.NewApp(iconPNG)
	// The interface is always light; the window's own caption and buttons
	// follow it rather than a dark system, which drew them light on our
	// light ground.
	mygo.Theme.SetSource(mygo.ThemeLight)
	mygo.App.WhenReady(func() {
		w, h := windowSize(mygo.Screen.PrimaryDisplay().WorkArea.Width, mygo.Screen.PrimaryDisplay().WorkArea.Height)
		a.AttachWindow(mygo.WindowOptions{
			Title:     "局域网 IP 扫描",
			Width:     w,
			Height:    h,
			MinWidth:  minWidth,
			MinHeight: minHeight,
			// Native window buttons stay; the view draws logo and name
			// into the title bar strip (extended title bar).
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  44,
			BackgroundColor: "#f2f4f7",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// windowSize picks the window's content size from the room the screen has,
// workArea in DIPs: the preferred size where it fits, else most of the
// screen, never below the least the layout needs unless the screen itself
// is smaller.
func windowSize(availW, availH int) (int, int) {
	return fit(prefWidth, availW, widthShare, minWidth), fit(prefHeight, availH, heightShare, minHeight)
}

func fit(pref, avail int, share float64, least int) int {
	if avail <= 0 {
		return pref // no screen reported, as in a test
	}
	return min(max(min(pref, int(float64(avail)*share)), least), avail)
}
