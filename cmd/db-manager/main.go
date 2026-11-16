package main

import (
	"fmt"
	"log/slog"
	"os"

	dbmanager "github.com/Opsord/local-database-manager"
	"github.com/Opsord/local-database-manager/internal/app"
	"github.com/Opsord/local-database-manager/internal/cli"
	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/logger"
	"github.com/Opsord/local-database-manager/internal/updater"

	tea "github.com/charmbracelet/bubbletea"
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
		slog.Info("Local Database Manager started", "projectRoot", projectRoot)
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		if logErr == nil {
			slog.Error("Error loading config", "err", err)
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		os.Exit(cli.Run(projectRoot, cfg, os.Args[1:]))
	}

	appModel := app.NewApp(projectRoot, cfg)
	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		if logErr == nil {
			slog.Error("Error running app", "err", err)
		}
		fmt.Fprintf(os.Stderr, "Error running Local Database Manager: %v\n", err)
		os.Exit(1)
	}
}
