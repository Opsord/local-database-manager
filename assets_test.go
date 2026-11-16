package dbmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWorkspace(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureWorkspace(dir); err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}

	requiredPaths := []string{
		filepath.Join(dir, "config.yml"),
		filepath.Join(dir, "instances", ".env.template"),
		filepath.Join(dir, "engines", "postgres", "docker-compose.yml"),
		filepath.Join(dir, "engines", "sql-server", "docker-compose.yml"),
	}
	for _, p := range requiredPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist, got err: %v", p, err)
		}
	}

	// Verify existing user files are never overwritten
	customCfg := "engine_health_interval: 15s\n"
	cfgPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(cfgPath, []byte(customCfg), 0o644); err != nil {
		t.Fatalf("write custom config: %v", err)
	}

	if err := EnsureWorkspace(dir); err != nil {
		t.Fatalf("second EnsureWorkspace() error = %v", err)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.yml: %v", err)
	}
	if string(got) != customCfg {
		t.Errorf("config.yml was overwritten: got %q, want %q", string(got), customCfg)
	}
}

func TestHasEnginesAndInstances(t *testing.T) {
	dir := t.TempDir()
	if HasEnginesAndInstances(dir) {
		t.Fatalf("expected false for empty directory %s", dir)
	}

	if err := os.Mkdir(filepath.Join(dir, "engines"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HasEnginesAndInstances(dir) {
		t.Fatalf("expected false when only engines/ exists")
	}

	if err := os.Mkdir(filepath.Join(dir, "instances"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !HasEnginesAndInstances(dir) {
		t.Fatalf("expected true when both engines/ and instances/ exist")
	}
}
