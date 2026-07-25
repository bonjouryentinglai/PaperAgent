// SPDX-License-Identifier: MIT
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:     "Paper Agent Installer",
		Width:     1080,
		Height:    760,
		MinWidth:  900,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 242, B: 235, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	}); err != nil {
		println("Paper Agent Installer:", err.Error())
	}
}
