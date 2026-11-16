package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_ValidInterval(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "engine_health_interval: 10s\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.EngineHealthInterval != 10*time.Second {
		t.Fatalf("interval = %v, want 10s", cfg.EngineHealthInterval)
	}
	if !cfg.CheckUpdates {
		t.Fatalf("CheckUpdates = false, want default true")
	}
	if cfg.UpdateChannel != "stable" {
		t.Fatalf("UpdateChannel = %q, want default 'stable'", cfg.UpdateChannel)
	}
}

func TestLoad_UpdateSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "engine_health_interval: 5s\ncheck_updates: false\nupdate_channel: PreRelease\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CheckUpdates {
		t.Fatalf("CheckUpdates = true, want false")
	}
	if cfg.UpdateChannel != "prerelease" {
		t.Fatalf("UpdateChannel = %q, want 'prerelease'", cfg.UpdateChannel)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  func(t *testing.T) string
		want string
	}{
		{
			name: "missing file",
			dir:  func(t *testing.T) string { return t.TempDir() },
			want: "config.yml",
		},
		{
			name: "invalid yaml",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: [\n")
				return dir
			},
			want: "parse",
		},
		{
			name: "missing key",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "# empty\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "zero",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: 0s\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "negative",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: -1s\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "garbage duration",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: nope\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "invalid update channel",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: 5s\nupdate_channel: nightly\n")
				return dir
			},
			want: "update_channel",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(tt.dir(t))
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q, want substring %q", err, tt.want)
			}
			if tt.name == "invalid yaml" && !strings.Contains(err.Error(), "config.yml") {
				t.Fatalf("error %q, want substring %q", err, "config.yml")
			}
		})
	}
}

func TestLoad_RepoConfigYml(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("repo config.yml: %v", err)
	}
	if cfg.EngineHealthInterval <= 0 {
		t.Fatalf("interval = %v, want > 0", cfg.EngineHealthInterval)
	}
}
