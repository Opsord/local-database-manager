package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanInstances scans the given directory for all .env instance definition files.
func ScanInstances(instancesDir string) ([]*DatabaseInstance, error) {
	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*DatabaseInstance{}, nil
		}
		return nil, fmt.Errorf("failed to read instances directory: %w", err)
	}

	var instances []*DatabaseInstance
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Ignore templates and non-env files
		if strings.HasPrefix(name, ".template") || strings.HasPrefix(name, ".env.template") {
			continue
		}
		if !strings.HasSuffix(name, ".env") && !strings.HasPrefix(name, ".env.") {
			continue
		}

		fullPath := filepath.Join(instancesDir, name)
		inst, err := ParseEnvFile(fullPath)
		if err != nil {
			// Skip or log broken file, but keep scanning
			continue
		}

		instances = append(instances, inst)
	}

	// Sort alphabetically by Name
	sort.Slice(instances, func(i, j int) bool {
		return strings.ToLower(instances[i].Name) < strings.ToLower(instances[j].Name)
	})

	return instances, nil
}
