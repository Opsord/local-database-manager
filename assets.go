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
