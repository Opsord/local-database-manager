package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	EngineHealthInterval time.Duration
}

func Load(projectRoot string) (Config, error) {
	path := filepath.Join(projectRoot, "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var raw struct {
		EngineHealthInterval string `yaml:"engine_health_interval"`
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

	return Config{EngineHealthInterval: d}, nil
}
