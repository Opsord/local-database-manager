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
	}

	composeFile := provider.GetComposeFile(inst)
	expectedCompose := filepath.Join(root, "engines", "sql-server", "podman-compose.yml")
	if composeFile != expectedCompose {
		t.Fatalf("expected compose file %q, got %q", expectedCompose, composeFile)
	}

	bin, args := provider.BuildComposeArgs(inst, "down")
	if bin != "podman" {
		t.Fatalf("expected binary 'podman', got %q", bin)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-p sql-proj") || !strings.Contains(joined, "down") {
		t.Fatalf("unexpected compose args: %s", joined)
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
