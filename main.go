package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"currency-wails/api"
	"currency-wails/backend"
	mcpserver "currency-wails/mcp-server"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := backend.NewService()
	app := backend.NewApp(service)
	apiServer := api.NewServer("127.0.0.1:8080", service)
	var mcpServer *mcpserver.Server

	startup := func(ctx context.Context) {
		service.Startup(ctx)

		status := service.GetStartupStatus()
		if !status.Ready {
			return
		}

		apiListener, err := apiServer.Listen()
		if err != nil {
			service.SetStartupError(fmt.Errorf("bind desktop API: %w", err))
			slog.Error("Desktop API listener failed", "error", err)
			return
		}

		apiURL := "http://" + apiListener.Addr().String()
		mcpServer = mcpserver.NewServer("127.0.0.1:8081", apiURL)
		mcpListener, err := mcpServer.Listen()
		if err != nil {
			service.SetStartupError(fmt.Errorf("bind MCP server: %w", err))
			slog.Error("MCP listener failed", "error", err)
			_ = apiServer.Shutdown()
			return
		}

		go func() {
			if err := apiServer.Serve(apiListener); err != nil {
				service.SetStartupError(fmt.Errorf("serve desktop API: %w", err))
				slog.Error("Desktop API server stopped unexpectedly", "error", err)
				_ = mcpServer.Shutdown()
			}
		}()

		go func() {
			if err := mcpServer.Serve(mcpListener); err != nil {
				service.SetStartupError(fmt.Errorf("serve MCP server: %w", err))
				slog.Error("MCP server stopped unexpectedly", "error", err)
				_ = apiServer.Shutdown()
			}
		}()
	}

	shutdown := func(ctx context.Context) {
		// Stop new MCP calls and wait for active CLI-backed calls first.
		if err := mcpServer.Shutdown(); err != nil {
			println("Error stopping MCP server:", err.Error())
		}

		// Stop accepting API requests after active MCP calls have finished.
		if err := apiServer.Shutdown(); err != nil {
			println("Error stopping API server:", err.Error())
		}

		// Close the shared database afterward.
		service.Shutdown(ctx)
	}

	err := wails.Run(&options.App{
		Title:            "Something",
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
