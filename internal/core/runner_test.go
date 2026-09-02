package core

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Verify at compile-time that Runner implements InstanceRunner
var _ InstanceRunner = (*Runner)(nil)

func TestRunner_BuildComposeArgs(t *testing.T) {
	t.Parallel()
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

	_, downV := runner.BuildComposeArgs(inst, "down", "-v")
	if !strings.Contains(strings.Join(downV, " "), "down -v") {
		t.Errorf("expected 'down -v' in args, got '%s'", strings.Join(downV, " "))
	}
}

func TestRunner_PodmanCompose(t *testing.T) {
	t.Parallel()
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

func TestRunner_CheckEngineHealth_NotInstalled(t *testing.T) {
	t.Parallel()
	runner := NewRunner("/test/app")

	got := runner.CheckEngineHealth(context.Background(), "nonexistent_runtime_binary_xyz")
	if got != EngineNotInstalled {
		t.Fatalf("CheckEngineHealth = %q, want %s", got, EngineNotInstalled)
	}
}

func TestRunner_Start_OfflineError(t *testing.T) {
	t.Parallel()
	runner := NewRunner("/test/app")
	inst := &DatabaseInstance{
		Name:        "offline_test",
		EngineType:  "postgres",
		Runtime:     "nonexistent_runtime_binary_xyz",
		EnvFilePath: "dummy.env",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := runner.Start(ctx, inst)
	if err == nil {
		t.Fatal("expected error for nonexistent runtime, got nil")
	}

	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Errorf("expected error to wrap ErrEngineNotInstalled, got %v", err)
	}
}

func TestRunner_StartEngine_NotInstalled(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	err := r.StartEngine(context.Background(), "nonexistent_runtime_binary_xyz")
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("got %v, want ErrEngineNotInstalled", err)
	}
}

func TestIsPodmanMachineAlreadyRunning(t *testing.T) {
	t.Parallel()
	if !isPodmanMachineAlreadyRunning("Error: unable to start \"podman-machine-default\": already running") {
		t.Fatal("expected already running detection")
	}
	if isPodmanMachineAlreadyRunning("some other error") {
		t.Fatal("should not match unrelated stderr")
	}
}

func TestPickPodmanConnectionToDefault(t *testing.T) {
	t.Parallel()
	conns := []podmanSystemConnection{
		{Name: "podman-machine-default-root", Default: true},
		{Name: "podman-machine-default", Default: false},
	}
	if got := pickPodmanConnectionToDefault(conns, map[string]bool{
		"podman-machine-default-root": false,
		"podman-machine-default":      true,
	}); got != "podman-machine-default" {
		t.Fatalf("got %q, want podman-machine-default", got)
	}
	if got := pickPodmanConnectionToDefault(conns, map[string]bool{
		"podman-machine-default-root": true,
		"podman-machine-default":      true,
	}); got != "" {
		t.Fatalf("got %q, want empty when default already works", got)
	}
	if got := pickPodmanConnectionToDefault(conns, map[string]bool{}); got != "" {
		t.Fatalf("got %q, want empty when nothing works", got)
	}
}

func TestWindowsNamedPipeToDOCKERHost(t *testing.T) {
	t.Parallel()
	got := windowsNamedPipeToDOCKERHost(`\\.\pipe\podman-machine-default`)
	want := "npipe:////./pipe/podman-machine-default"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if windowsNamedPipeToDOCKERHost("") != "" {
		t.Fatal("empty path should yield empty host")
	}
	if windowsNamedPipeToDOCKERHost(`C:\not\a\pipe`) != "" {
		t.Fatal("non-pipe path should yield empty host")
	}
}

func TestIsPodmanComposePipeError(t *testing.T) {
	t.Parallel()
	stderr := `unable to get image 'postgres:18': error during connect: Get "http://%2F%2F.%2Fpipe%2Fpodman-machine-default/v1.55/images/postgres:18/json": EOF`
	if !isPodmanComposePipeError(stderr) {
		t.Fatal("expected pipe EOF detection")
	}
	if isPodmanComposePipeError("image not found") {
		t.Fatal("should not match unrelated errors")
	}
}

func TestRunner_StartEngine_AlreadyOnlineIsNoop(t *testing.T) {
	t.Parallel()
	// Skip if neither docker nor podman is online on this machine.
	r := NewRunner("/tmp")
	ctx := context.Background()
	for _, rt := range []string{"docker", "podman"} {
		if r.CheckEngineHealth(ctx, rt) != EngineOnline {
			continue
		}
		if err := r.StartEngine(ctx, rt); err != nil {
			t.Fatalf("online %s StartEngine = %v, want nil", rt, err)
		}
		return
	}
	t.Skip("no online docker/podman to assert no-op")
}

func TestWaitUntilOnline_TimesOut(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := r.waitUntilOnline(ctx, "nonexistent_runtime_binary_xyz")
	if err == nil {
		t.Fatal("expected timeout/error")
	}
	if !errors.Is(err, ErrEngineStartFailed) && !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("got err=%v", err)
	}
}

func TestFindDockerDesktopExe_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	_, err := findDockerDesktopExe()
	_ = err // existence depends on machine; just ensure it doesn't panic
}

func TestRunner_StopEngine_NotInstalled(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	err := r.StopEngine(context.Background(), "nonexistent_runtime_binary_xyz")
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("got %v, want ErrEngineNotInstalled", err)
	}
}

func TestWaitUntilOffline_TimesOut(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx := context.Background()
	if r.CheckEngineHealth(ctx, "docker") != EngineOnline {
		t.Skip("docker not online; cannot test waitUntilOffline timeout")
	}
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err := r.waitUntilOffline(ctx, "docker")
	if err == nil {
		t.Fatal("expected timeout/error")
	}
	if !errors.Is(err, ErrEngineStartFailed) && !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("got err=%v", err)
	}
}

func TestRunner_StopEngine_AlreadyOfflineIsNoop(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx := context.Background()
	for _, rt := range []string{"docker", "podman"} {
		h := r.CheckEngineHealth(ctx, rt)
		if h != EngineOffline && h != EngineNotInstalled {
			continue
		}
		if h == EngineNotInstalled {
			continue
		}
		if err := r.StopEngine(ctx, rt); err != nil {
			t.Fatalf("offline %s StopEngine = %v, want nil", rt, err)
		}
		return
	}
	t.Skip("no offline docker/podman to assert no-op")
}
