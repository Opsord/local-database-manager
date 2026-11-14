package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	dbmanager "github.com/Opsord/local-database-manager"
	"github.com/Opsord/local-database-manager/internal/app"
	"github.com/Opsord/local-database-manager/internal/cli"
	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/logger"

	tea "github.com/charmbracelet/bubbletea"
)

func findProjectRoot() (string, error) {
	// 1. Current working directory (cloned repo or local workspace)
	cwd, err := os.Getwd()
	if err == nil && hasEnginesAndInstances(cwd) {
		return cwd, nil
	}

	// 2. Executable directory (extracted release archive)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if hasEnginesAndInstances(exeDir) {
			return exeDir, nil
		}
		// Check parent of exe dir (e.g. if binary is in bin/ or cmd/db-manager/)
		parentDir := filepath.Dir(exeDir)
		if hasEnginesAndInstances(parentDir) {
			return parentDir, nil
		}
	}

	// 3. Standalone / `go install` fallback: ~/.db-manager
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return cwd, fmt.Errorf("resolve user home directory: %w", homeErr)
	}
	workspaceDir := filepath.Join(homeDir, ".db-manager")
	if err := dbmanager.EnsureWorkspace(workspaceDir); err != nil {
		return workspaceDir, fmt.Errorf("initialize workspace %s: %w", workspaceDir, err)
	}
	return workspaceDir, nil
}

func hasEnginesAndInstances(dir string) bool {
	engines := filepath.Join(dir, "engines")
	instances := filepath.Join(dir, "instances")
	_, err1 := os.Stat(engines)
	_, err2 := os.Stat(instances)
	return err1 == nil && err2 == nil
}

func main() {
	projectRoot, rootErr := findProjectRoot()
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
