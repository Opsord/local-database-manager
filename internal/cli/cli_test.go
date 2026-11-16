package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
)

type testRunnerStub struct {
	status      core.ContainerStatus
	startCalled bool
	stopCalled  bool
}

func (s *testRunnerStub) CheckEngineHealth(context.Context, string) core.EngineHealth {
	return core.EngineOnline
}
func (s *testRunnerStub) StartEngine(context.Context, string) error { return nil }
func (s *testRunnerStub) StopEngine(context.Context, string) error  { return nil }
func (s *testRunnerStub) Start(context.Context, *core.DatabaseInstance) error {
	s.startCalled = true
	return nil
}
func (s *testRunnerStub) Stop(context.Context, *core.DatabaseInstance) error {
	s.stopCalled = true
	return nil
}
func (s *testRunnerStub) DownVolumes(context.Context, *core.DatabaseInstance) error { return nil }
func (s *testRunnerStub) CheckStatus(context.Context, *core.DatabaseInstance) core.ContainerStatus {
	return s.status
}
func (s *testRunnerStub) GetStats(context.Context, *core.DatabaseInstance) core.ContainerStats {
	return core.ContainerStats{CPU: "0.10%", Memory: "50MiB / 512MiB"}
}
func (s *testRunnerStub) GetMemoryUsage(context.Context, *core.DatabaseInstance) string {
	return "50MiB / 512MiB"
}
func (s *testRunnerStub) LogsCommand(*core.DatabaseInstance) *exec.Cmd {
	return exec.Command("echo", "test log line")
}

func setupTestProject(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	instDir := filepath.Join(tmpDir, "instances")
	if err := os.MkdirAll(instDir, 0755); err != nil {
		t.Fatalf("failed to create instances dir: %v", err)
	}

	envContent := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=postgres-test
POSTGRES_USER=myuser
POSTGRES_PASSWORD=mypassword
POSTGRES_DB=testdb
PORT=5432
`
	if err := os.WriteFile(filepath.Join(instDir, "test.env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write test.env: %v", err)
	}

	return tmpDir
}

func TestCLI_Help(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := RunWithOutput(".", config.Config{}, []string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "update") || !strings.Contains(stdout.String(), "version") {
		t.Errorf("expected usage to document update and version commands, got: %s", stdout.String())
	}
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := RunWithOutput(".", config.Config{}, []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Local Database Manager") {
		t.Errorf("unexpected version output: %s", stdout.String())
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := RunWithOutput(".", config.Config{}, []string{"invalid_command_xyz"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command") {
		t.Errorf("expected unknown command error, got: %s", stderr.String())
	}
}

func TestCLI_URIAndEnv(t *testing.T) {
	t.Parallel()
	projectRoot := setupTestProject(t)
	cfg := config.Config{EngineHealthInterval: 5 * time.Second}

	// URI
	var uriOut, uriErr bytes.Buffer
	code := RunWithOutput(projectRoot, cfg, []string{"uri", "test"}, &uriOut, &uriErr)
	if code != 0 {
		t.Fatalf("uri failed with code %d: %s", code, uriErr.String())
	}
	if !strings.Contains(uriOut.String(), "postgresql://myuser:mypassword@localhost:5432/testdb") {
		t.Errorf("unexpected uri: %s", uriOut.String())
	}

	// Env
	var envOut, envErr bytes.Buffer
	code = RunWithOutput(projectRoot, cfg, []string{"env", "test"}, &envOut, &envErr)
	if code != 0 {
		t.Fatalf("env failed with code %d: %s", code, envErr.String())
	}
	if !strings.Contains(envOut.String(), "DATABASE_URL=") {
		t.Errorf("unexpected env: %s", envOut.String())
	}
}

func TestCLI_InstanceNotFound(t *testing.T) {
	t.Parallel()
	projectRoot := setupTestProject(t)
	cfg := config.Config{EngineHealthInterval: 5 * time.Second}

	var stdout, stderr bytes.Buffer
	code := RunWithOutput(projectRoot, cfg, []string{"up", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found' error, got: %s", stderr.String())
	}
}

func TestCLI_ListAndRunnerActions(t *testing.T) {
	t.Parallel()
	projectRoot := setupTestProject(t)
	instDir := filepath.Join(projectRoot, "instances")
	runner := &testRunnerStub{status: core.StatusReady}

	var listOut, listErr bytes.Buffer
	code := runList(instDir, runner, &listOut, &listErr)
	if code != 0 {
		t.Fatalf("runList failed with code %d: %s", code, listErr.String())
	}
	if !strings.Contains(listOut.String(), "test") || !strings.Contains(listOut.String(), "5432") {
		t.Errorf("expected test instance in list output, got: %s", listOut.String())
	}

	// Start already running
	var startOut, startErr bytes.Buffer
	code = runStart(instDir, runner, []string{"test"}, &startOut, &startErr)
	if code != 0 {
		t.Fatalf("expected 0 for already running, got %d", code)
	}
	if !strings.Contains(startOut.String(), "already running") {
		t.Errorf("expected 'already running', got: %s", startOut.String())
	}

	// Stop
	var stopOut, stopErr bytes.Buffer
	code = runStop(instDir, runner, []string{"test"}, &stopOut, &stopErr)
	if code != 0 {
		t.Fatalf("expected 0 for stop, got %d", code)
	}
	if !runner.stopCalled {
		t.Fatalf("expected runner.Stop to be called")
	}

	// Restart
	var restartOut, restartErr bytes.Buffer
	code = runRestart(instDir, runner, []string{"test"}, &restartOut, &restartErr)
	if code != 0 {
		t.Fatalf("expected 0 for restart, got %d", code)
	}
	if !runner.startCalled {
		t.Fatalf("expected runner.Start to be called during restart")
	}
}
