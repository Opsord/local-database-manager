package main

import (
	"fmt"
	"os"
	"path/filepath"

	"local-database-manager/internal/app"
	"local-database-manager/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func findProjectRoot() string {
	// 1. Current working directory
	cwd, err := os.Getwd()
	if err == nil {
		if hasEnginesAndInstances(cwd) {
			return cwd
		}
	}

	// 2. Executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if hasEnginesAndInstances(exeDir) {
			return exeDir
		}
		// Check parent of exe dir (e.g. if binary is in bin/ or cmd/db-manager/)
		parentDir := filepath.Dir(exeDir)
		if hasEnginesAndInstances(parentDir) {
			return parentDir
		}
	}

	return cwd
}

func hasEnginesAndInstances(dir string) bool {
	engines := filepath.Join(dir, "engines")
	instances := filepath.Join(dir, "instances")
	_, err1 := os.Stat(engines)
	_, err2 := os.Stat(instances)
	return err1 == nil && err2 == nil
}

func main() {
	projectRoot := findProjectRoot()

	cfg, err := config.Load(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	appModel := app.NewApp(projectRoot, cfg)
	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error ejecutando Local Database Manager: %v\n", err)
		os.Exit(1)
	}
}
