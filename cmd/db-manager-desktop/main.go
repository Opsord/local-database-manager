package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	dbmanager "github.com/Opsord/local-database-manager"
	"github.com/Opsord/local-database-manager/frontend"
	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
	"github.com/Opsord/local-database-manager/internal/desktop"
	"github.com/Opsord/local-database-manager/internal/logger"
	"github.com/Opsord/local-database-manager/internal/updater"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func main() {
	updater.CleanupOldBinary()

	projectRoot, rootErr := dbmanager.ResolveWorkspace()
	if rootErr != nil {
		fmt.Fprintf(os.Stderr, "Error initializing workspace: %v\n", rootErr)
		os.Exit(1)
	}

	closeLogger, logErr := logger.Init(projectRoot)
	if logErr == nil {
		defer closeLogger()
		slog.Info("Local Database Manager Desktop started", "projectRoot", projectRoot)
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		if logErr == nil {
			slog.Error("Error loading config", "err", err)
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	runner := core.NewRunner(projectRoot)
	svc := desktop.NewService(projectRoot, cfg, runner)

	err = wails.Run(&options.App{
		Title:            "Local Database Manager",
		Width:            1180,
		Height:           760,
		MinWidth:         960,
		MinHeight:        620,
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 31, A: 255},
		AssetServer: &assetserver.Options{
			Assets: frontend.DistFS,
		},
		OnStartup: func(ctx context.Context) {
			svc.SetEventEmitter(func(event string, data ...any) {
				runtime.EventsEmit(ctx, event, data...)
			})
			svc.SetQuitFunc(func() {
				runtime.Quit(ctx)
			})
		},
		OnShutdown: func(_ context.Context) {
			svc.Shutdown()
		},
		Bind: []interface{}{
			svc,
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		Mac: &mac.Options{
			Appearance: mac.NSAppearanceNameDarkAqua,
		},
	})
	if err != nil {
		if logErr == nil {
			slog.Error("Error running desktop app", "err", err)
		}
		fmt.Fprintf(os.Stderr, "Error running Local Database Manager Desktop: %v\n", err)
		os.Exit(1)
	}
}
