// Command lslan is a Windows LAN IP scanner desktop app built on mygo.
package main

import (
	"log"

	"github.com/egoist/mygo"

	"lslan/internal/ui"
)

func main() {
	a := ui.NewApp()
	mygo.App.WhenReady(func() {
		a.AttachWindow(mygo.WindowOptions{
			Title:     "局域网 IP 扫描",
			Width:     1000,
			Height:    680,
			MinWidth:  800,
			MinHeight: 520,
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
