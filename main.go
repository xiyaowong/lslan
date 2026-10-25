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

func main() {
	a := ui.NewApp(iconPNG)
	mygo.App.WhenReady(func() {
		a.AttachWindow(mygo.WindowOptions{
			Title:     "局域网 IP 扫描",
			Width:     1000,
			Height:    680,
			MinWidth:  800,
			MinHeight: 520,
			// Native window buttons stay; the view draws logo and name
			// into the title bar strip (extended title bar).
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  44,
			BackgroundColor: "#f3f3f3",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
