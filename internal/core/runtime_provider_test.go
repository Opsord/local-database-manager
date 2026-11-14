package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// Ensure at compile time that providers implement RuntimeProvider
var _ RuntimeProvider = (*DockerProvider)(nil)
var _ RuntimeProvider = (*PodmanProvider)(nil)
var _ RuntimeProvider = (*unsupportedProvider)(nil)

func TestRuntimeProvider_Docker(t *testing.T) {
	t.Parallel()
	root := "/test/app"
	provider := NewDockerProvider(root)

	if provider.Name() != "docker" {
		t.Fatalf("expected Name() to be 'docker', got %q", provider.Name())
	}

	inst := &DatabaseInstance{
		Name:          "pg_test",
		EngineType:    "postgres",
		Runtime:       "docker",
		ProjectName:   "pg-proj",
		EnvFilePath:   "/test/app/instances/pg_test.env",
		ContainerName: "pg-proj-c",
	}

	composeFile := provider.GetComposeFile(inst)
	expectedCompose := filepath.Join(root, "engines", "postgres", "docker-compose.yml")
	if composeFile != expectedCompose {
		t.Fatalf("expected compose file %q, got %q", expectedCompose, composeFile)
	}

	bin, args := provider.BuildComposeArgs(inst, "up", "-d")
	if bin != "docker" {
		t.Fatalf("expected binary 'docker', got %q", bin)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-p pg-proj") || !strings.Contains(joined, "up -d") {
		t.Fatalf("unexpected compose args: %s", joined)
	}
}

func TestRuntimeProvider_Podman(t *testing.T) {
	t.Parallel()
	root := "/test/app"
	provider := NewPodmanProvider(root)

	if provider.Name() != "podman" {
		t.Fatalf("expected Name() to be 'podman', got %q", provider.Name())
	}

	inst := &DatabaseInstance{
		Name:          "sql_test",
		EngineType:    "sqlserver",
		Runtime:       "podman",
		ProjectName:   "sql-proj",
		EnvFilePath:   "/test/app/instances/sql_test.env",
		ContainerName: "sql-proj-c",
		Port:          1433,
		Database:      "app_db",
		Schema:        "dbo",
	}

	args, err := provider.buildPodmanRunArgs(inst)
	if err != nil {
		t.Fatalf("buildPodmanRunArgs error: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--name sql-proj-c") || !strings.Contains(joined, "127.0.0.1:1433:1433") {
		t.Fatalf("unexpected podman run args: %s", joined)
	}

	logsCmd := provider.LogsCommand(inst)
	if !strings.Contains(strings.Join(logsCmd.Args, " "), "logs --tail=100 -f sql-proj-c") {
		t.Fatalf("unexpected logs args: %v", logsCmd.Args)
	}
}

func TestRuntimeProvider_Unsupported(t *testing.T) {
	t.Parallel()
	u := newUnsupportedProvider("unknown_runtime_xyz")
	if u.Name() != "unknown_runtime_xyz" {
		t.Fatalf("expected Name() to be 'unknown_runtime_xyz', got %q", u.Name())
	}

	ctx := context.Background()
	health := u.Health(ctx)
	if health != EngineNotInstalled {
		t.Fatalf("expected EngineNotInstalled, got %s", health)
	}

	err := u.StartDaemon(ctx)
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("expected ErrEngineNotInstalled, got %v", err)
	}

	err = u.StopDaemon(ctx)
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("expected ErrEngineNotInstalled, got %v", err)
	}

	inst := &DatabaseInstance{Runtime: "unknown_runtime_xyz"}
	err = u.Start(ctx, inst)
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("expected ErrEngineNotInstalled, got %v", err)
	}

	if u.CheckStatus(ctx, inst) != StatusStopped {
		t.Fatalf("expected StatusStopped")
	}

	if u.GetMemoryUsage(ctx, inst) != "-" {
		t.Fatalf("expected '-'")
	}

	stats := u.GetStats(ctx, inst)
	if stats.CPU != "-" || stats.Memory != "-" {
		t.Fatalf("expected '-' for cpu and memory, got %+v", stats)
	}
}

func TestRunner_ProviderDelegation(t *testing.T) {
	t.Parallel()
	r := NewRunner("/test/app")

	dockerP := r.providerFor("docker")
	if dockerP.Name() != "docker" {
		t.Fatalf("expected docker provider, got %q", dockerP.Name())
	}

	// Default empty should resolve to docker
	emptyP := r.providerFor("")
	if emptyP.Name() != "docker" {
		t.Fatalf("expected docker provider for empty runtime, got %q", emptyP.Name())
	}

	podmanP := r.providerFor("podman")
	if podmanP.Name() != "podman" {
		t.Fatalf("expected podman provider, got %q", podmanP.Name())
	}

	customP := r.providerFor("custom_xyz")
	if customP.Name() != "custom_xyz" {
		t.Fatalf("expected custom_xyz, got %q", customP.Name())
	}
}
