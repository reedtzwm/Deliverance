package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	startHidden := false
	for _, a := range os.Args[1:] {
		if a == "--minimized" {
			startHidden = true
		}
	}

	err := wails.Run(&options.App{
		Title:             "Deliverance",
		Width:             900,
		Height:            640,
		MinWidth:          760,
		MinHeight:         560,
		StartHidden:       startHidden,
		Frameless:         true, // своя панель окна в интерфейсе
		HideWindowOnClose: true, // крестик сворачивает в трей
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: windowBackground(loadConfig().Theme),
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "b7e2c4a1-5d3f-4e8a-9c61-deliverance",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.showWindow()
			},
		},
		Windows: &windows.Options{
			Theme: windows.Light,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
