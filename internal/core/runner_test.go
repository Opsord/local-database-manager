package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunner_BuildComposeArgs(t *testing.T) {
	root := "/test/app"
	runner := NewRunner(root)

	inst := &DatabaseInstance{
		Name:          "super_calendar",
		EngineType:    "postgres",
		Runtime:       "docker",
		ProjectName:   "pg-super-calendar",
		EnvFilePath:   "/test/app/instances/super_calendar.env",
		ContainerName: "pg-super-calendar",
	}

	bin, args := runner.BuildComposeArgs(inst, "up", "-d")
	if bin != "docker" {
		t.Errorf("expected binary 'docker', got '%s'", bin)
	}

	joined := strings.Join(args, " ")
	expectedComposeFile := filepath.Join(root, "engines", "postgres", "docker-compose.yml")

	if !strings.Contains(joined, "-p pg-super-calendar") {
		t.Errorf("expected -p pg-super-calendar in args, got '%s'", joined)
	}
	if !strings.Contains(joined, expectedComposeFile) {
		t.Errorf("expected compose file '%s' in args, got '%s'", expectedComposeFile, joined)
	}
	if !strings.Contains(joined, "--env-file /test/app/instances/super_calendar.env") {
		t.Errorf("expected env file in args, got '%s'", joined)
	}
	if !strings.Contains(joined, "up -d") {
		t.Errorf("expected 'up -d' in args, got '%s'", joined)
	}
}

func TestRunner_PodmanCompose(t *testing.T) {
	root := "/test/app"
	runner := NewRunner(root)

	inst := &DatabaseInstance{
		Name:          "local_pod",
		EngineType:    "postgres",
		Runtime:       "podman",
		ProjectName:   "pod-calendar",
		EnvFilePath:   "/test/app/instances/local_pod.env",
		ContainerName: "pod-calendar",
	}

	bin, args := runner.BuildComposeArgs(inst, "down")
	if bin != "podman" {
		t.Errorf("expected binary 'podman', got '%s'", bin)
	}

	joined := strings.Join(args, " ")
	expectedComposeFile := filepath.Join(root, "engines", "postgres", "podman-compose.yml")
	if !strings.Contains(joined, expectedComposeFile) {
		t.Errorf("expected compose file '%s', got '%s'", expectedComposeFile, joined)
	}
}
