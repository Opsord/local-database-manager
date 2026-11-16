package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	EngineHealthInterval time.Duration
	CheckUpdates         bool
	UpdateChannel        string
}

func Load(projectRoot string) (Config, error) {
	path := filepath.Join(projectRoot, "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var raw struct {
		EngineHealthInterval string `yaml:"engine_health_interval"`
		CheckUpdates         *bool  `yaml:"check_updates"`
		UpdateChannel        string `yaml:"update_channel"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if raw.EngineHealthInterval == "" {
		return Config{}, fmt.Errorf("%s: missing engine_health_interval", path)
	}

	d, err := time.ParseDuration(raw.EngineHealthInterval)
	if err != nil {
		return Config{}, fmt.Errorf("%s: engine_health_interval: %w", path, err)
	}
	if d <= 0 {
		return Config{}, fmt.Errorf("%s: engine_health_interval must be > 0, got %s", path, d)
	}

	checkUpdates := true
	if raw.CheckUpdates != nil {
		checkUpdates = *raw.CheckUpdates
	}

	updateChannel := strings.ToLower(strings.TrimSpace(raw.UpdateChannel))
	if updateChannel == "" {
		updateChannel = "stable"
	}
	if updateChannel != "stable" && updateChannel != "prerelease" {
		return Config{}, fmt.Errorf("%s: update_channel must be 'stable' or 'prerelease', got %q", path, raw.UpdateChannel)
	}

	return Config{
		EngineHealthInterval: d,
		CheckUpdates:         checkUpdates,
		UpdateChannel:        updateChannel,
	}, nil
}
