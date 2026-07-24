package main

import (
	"context"
	"embed"

	"currency-wails/api"
	"currency-wails/backend"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := backend.NewApp()
	apiServer := api.NewServer("127.0.0.1:8080", app)

	startup := func(ctx context.Context) {
		app.Startup(ctx)

		status := app.GetStartupStatus()
		if !status.Ready {
			return
		}

		go func() {
			if err := apiServer.Start(); err != nil {
				println("API server error:", err.Error())
			}
		}()
	}

	shutdown := func(ctx context.Context) {
		// Stop accepting API requests and wait for active requests first.
		if err := apiServer.Shutdown(); err != nil {
			println("Error stopping API server:", err.Error())
		}

		// Close the shared database afterward.
		app.Shutdown(ctx)
	}

	err := wails.Run(&options.App{
		Title:            "Currency Exchange Rates",
		Width:            1200,
		Height:           760,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 20, A: 1},
		OnStartup:        startup,
		OnShutdown:       shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
