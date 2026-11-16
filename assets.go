package dbmanager

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed engines instances/.env.template config.yml
var embeddedAssets embed.FS

// EnsureWorkspace initializes targetDir with the embedded engines, config.yml,
// and instances/.env.template if they do not already exist. Existing files
// are never overwritten so user customizations are preserved.
func EnsureWorkspace(targetDir string) error {
	if err := os.MkdirAll(filepath.Join(targetDir, "instances"), 0o755); err != nil {
		return fmt.Errorf("create instances directory: %w", err)
	}

	return fs.WalkDir(embeddedAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}

		dest := filepath.Join(targetDir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		if _, statErr := os.Stat(dest); statErr == nil {
			return nil
		}

		data, readErr := embeddedAssets.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}

		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return fmt.Errorf("create parent dir for %s: %w", dest, mkErr)
		}

		perm := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			perm = 0o755
		}

		if writeErr := os.WriteFile(dest, data, perm); writeErr != nil {
			return fmt.Errorf("write %s: %w", dest, writeErr)
		}
		return nil
	})
}

// HasEnginesAndInstances reports whether dir contains both engines/ and instances/ subdirectories.
func HasEnginesAndInstances(dir string) bool {
	if dir == "" {
		return false
	}
	engines := filepath.Join(dir, "engines")
	instances := filepath.Join(dir, "instances")
	_, err1 := os.Stat(engines)
	_, err2 := os.Stat(instances)
	return err1 == nil && err2 == nil
}

// ResolveWorkspace finds the active workspace root directory containing engines/ and instances/,
// falling back to ~/.db-manager and initializing it from embedded assets if needed.
func ResolveWorkspace() (string, error) {
	// 1. Current working directory (cloned repo or local workspace)
	cwd, err := os.Getwd()
	if err == nil && HasEnginesAndInstances(cwd) {
		return cwd, nil
	}

	// 2. Executable directory (extracted release archive)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if HasEnginesAndInstances(exeDir) {
			return exeDir, nil
		}
		// Check parent of exe dir (e.g. if binary is in bin/ or cmd/db-manager/)
		parentDir := filepath.Dir(exeDir)
		if HasEnginesAndInstances(parentDir) {
			return parentDir, nil
		}
		// Check macOS .app bundle parent directory (App.app/Contents/MacOS/<bin>)
		if filepath.Base(exeDir) == "MacOS" {
			bundleParent := filepath.Dir(filepath.Dir(parentDir))
			if HasEnginesAndInstances(bundleParent) {
				return bundleParent, nil
			}
		}
	}

	// 3. Standalone / `go install` fallback: ~/.db-manager
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return cwd, fmt.Errorf("resolve user home directory: %w", homeErr)
	}
	workspaceDir := filepath.Join(homeDir, ".db-manager")
	if err := EnsureWorkspace(workspaceDir); err != nil {
		return workspaceDir, fmt.Errorf("initialize workspace %s: %w", workspaceDir, err)
	}
	return workspaceDir, nil
}
