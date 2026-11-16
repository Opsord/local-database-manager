package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
)

type mockRunner struct {
	mu              sync.Mutex
	dockerHealth    core.EngineHealth
	podmanHealth    core.EngineHealth
	statusByName    map[string]core.ContainerStatus
	statsByName     map[string]core.ContainerStats
	startErr        error
	stopErr         error
	downVolumesErr  error
	startCalls      []string
	stopCalls       []string
	downCalls       []string
	startEngineList  []string
	stopEngineList   []string
	checkStatusCalls int
	logsCmdFn        func(inst *core.DatabaseInstance) *exec.Cmd
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineOffline,
		statusByName: make(map[string]core.ContainerStatus),
		statsByName:  make(map[string]core.ContainerStats),
	}
}

func (m *mockRunner) CheckEngineHealth(_ context.Context, runtimeName string) core.EngineHealth {
	m.mu.Lock()
	defer m.mu.Unlock()
	if runtimeName == "podman" {
		return m.podmanHealth
	}
	return m.dockerHealth
}

func (m *mockRunner) StartEngine(_ context.Context, runtimeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startEngineList = append(m.startEngineList, runtimeName)
	if runtimeName == "podman" {
		m.podmanHealth = core.EngineOnline
	} else {
		m.dockerHealth = core.EngineOnline
	}
	return nil
}

func (m *mockRunner) StopEngine(_ context.Context, runtimeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopEngineList = append(m.stopEngineList, runtimeName)
	if runtimeName == "podman" {
		m.podmanHealth = core.EngineOffline
	} else {
		m.dockerHealth = core.EngineOffline
	}
	return nil
}

func (m *mockRunner) Start(_ context.Context, inst *core.DatabaseInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls = append(m.startCalls, inst.Name)
	if m.startErr != nil {
		return m.startErr
	}
	m.statusByName[inst.Name] = core.StatusReady
	m.statsByName[inst.Name] = core.ContainerStats{CPU: "0.42%", Memory: "64MiB / 512MiB"}
	return nil
}

func (m *mockRunner) Stop(_ context.Context, inst *core.DatabaseInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalls = append(m.stopCalls, inst.Name)
	if m.stopErr != nil {
		return m.stopErr
	}
	m.statusByName[inst.Name] = core.StatusStopped
	delete(m.statsByName, inst.Name)
	return nil
}

func (m *mockRunner) DownVolumes(_ context.Context, inst *core.DatabaseInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downCalls = append(m.downCalls, inst.Name)
	if m.downVolumesErr != nil {
		return m.downVolumesErr
	}
	m.statusByName[inst.Name] = core.StatusStopped
	delete(m.statsByName, inst.Name)
	return nil
}

func (m *mockRunner) CheckStatus(_ context.Context, inst *core.DatabaseInstance) core.ContainerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkStatusCalls++
	if s, ok := m.statusByName[inst.Name]; ok {
		return s
	}
	return core.StatusStopped
}

func (m *mockRunner) GetStats(_ context.Context, inst *core.DatabaseInstance) core.ContainerStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.statsByName[inst.Name]; ok {
		return st
	}
	return core.ContainerStats{CPU: "-", Memory: "-"}
}

func (m *mockRunner) GetMemoryUsage(ctx context.Context, inst *core.DatabaseInstance) string {
	return m.GetStats(ctx, inst).Memory
}

func (m *mockRunner) LogsCommand(inst *core.DatabaseInstance) *exec.Cmd {
	if m.logsCmdFn != nil {
		return m.logsCmdFn(inst)
	}
	return nil
}

func setupTestService(t *testing.T) (*Service, *mockRunner, string) {
	t.Helper()
	root := t.TempDir()
	instancesDir := filepath.Join(root, "instances")
	if err := os.MkdirAll(instancesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := newMockRunner()
	svc := NewService(root, config.Config{EngineHealthInterval: 5 * time.Second}, runner)
	return svc, runner, instancesDir
}

func TestListInstancesAndMetadata(t *testing.T) {
	svc, runner, instancesDir := setupTestService(t)
	envContent := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_PORT=5435
POSTGRES_USER=postgres
POSTGRES_PASSWORD=secret
POSTGRES_DB=shop_db
POSTGRES_SCHEMA=public
POSTGRES_VERSION=17
POSTGRES_VOLUME=pgdata_shop_17
`
	if err := os.WriteFile(filepath.Join(instancesDir, "shop.env"), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runner.statusByName["shop"] = core.StatusReady
	runner.statsByName["shop"] = core.ContainerStats{CPU: "1.15%", Memory: "110MiB / 512MiB"}

	meta := svc.GetAppMetadata()
	if meta.EngineHealthIntervalMs != 5000 {
		t.Fatalf("EngineHealthIntervalMs = %d, want 5000", meta.EngineHealthIntervalMs)
	}
	if len(meta.PostgresVersions) == 0 || len(meta.SQLServerVersions) == 0 {
		t.Fatalf("expected non-empty supported versions in metadata")
	}

	list, err := svc.ListInstances()
	if err != nil {
		t.Fatalf("ListInstances() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	got := list[0]
	if got.Name != "shop" || got.Status != string(core.StatusReady) {
		t.Fatalf("unexpected instance DTO: %+v", got)
	}
	if got.CPUUsage != "1.15%" || !strings.Contains(got.ConnectionURI, "postgresql://") {
		t.Fatalf("unexpected metrics/URI: %+v", got)
	}
	if !strings.Contains(got.BackendEnv, "DATABASE_URL=") || !strings.Contains(got.CLICommand, "psql") {
		t.Fatalf("expected BackendEnv and CLICommand populated, got: %+v", got)
	}
}

func TestGetEnginesHealth(t *testing.T) {
	svc, runner, _ := setupTestService(t)
	runner.dockerHealth = core.EngineOnline
	runner.podmanHealth = core.EngineNotInstalled

	health := svc.GetEnginesHealth()
	if health.Docker != string(core.EngineOnline) {
		t.Fatalf("docker health = %q, want ONLINE", health.Docker)
	}
	if health.Podman != string(core.EngineNotInstalled) {
		t.Fatalf("podman health = %q, want NOT_INSTALLED", health.Podman)
	}
}

func TestStartInstanceOfflineReturnsTypedPrefix(t *testing.T) {
	svc, runner, _ := setupTestService(t)
	_, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "18",
		Name:    "orders",
		Port:    "5441",
	})
	if err != nil {
		t.Fatal(err)
	}

	runner.startErr = fmt.Errorf("%w: docker daemon offline", core.ErrEngineOffline)
	_, err = svc.StartInstance("orders")
	if err == nil || !strings.HasPrefix(err.Error(), core.ErrCodeEngineOffline+":") {
		t.Fatalf("expected error prefixed with %s:, got %v", core.ErrCodeEngineOffline, err)
	}
}

func TestStartStopPurgeAndDeleteInstance(t *testing.T) {
	svc, runner, instancesDir := setupTestService(t)
	_, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "18",
		Name:    "billing",
		Port:    "5442",
	})
	if err != nil {
		t.Fatal(err)
	}

	started, err := svc.StartInstance("billing")
	if err != nil {
		t.Fatalf("StartInstance() error = %v", err)
	}
	if started.Status != string(core.StatusReady) {
		t.Fatalf("Status = %q, want READY", started.Status)
	}

	stopped, err := svc.StopInstance("billing")
	if err != nil {
		t.Fatalf("StopInstance() error = %v", err)
	}
	if stopped.Status != string(core.StatusStopped) {
		t.Fatalf("Status = %q, want STOPPED", stopped.Status)
	}

	purged, err := svc.PurgeInstance("billing")
	if err != nil {
		t.Fatalf("PurgeInstance() error = %v", err)
	}
	if purged.Status != string(core.StatusStopped) || len(runner.downCalls) != 1 {
		t.Fatalf("expected PurgeInstance to call DownVolumes once")
	}

	// Delete requires online engine
	runner.dockerHealth = core.EngineOffline
	if err := svc.DeleteInstance("billing"); err == nil || !strings.HasPrefix(err.Error(), core.ErrCodeEngineOffline+":") {
		t.Fatalf("expected ENGINE_OFFLINE when deleting while engine offline, got %v", err)
	}

	runner.dockerHealth = core.EngineOnline
	if err := svc.DeleteInstance("billing"); err != nil {
		t.Fatalf("DeleteInstance() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(instancesDir, "billing.env")); !os.IsNotExist(statErr) {
		t.Fatalf("expected billing.env to be removed, statErr=%v", statErr)
	}
}

func TestSaveInstanceCreateDuplicateAndEditWhileRunning(t *testing.T) {
	svc, runner, instancesDir := setupTestService(t)

	res, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "16",
		Name:    "My App",
		Port:    "5450",
	})
	if err != nil {
		t.Fatalf("SaveInstance(create) error = %v", err)
	}
	if res.Name != "my_app" || res.DerivedVolume != "pgdata_my_app_16" {
		t.Fatalf("unexpected create response: %+v", res)
	}

	// Reject duplicate name
	if _, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "18",
		Name:    "my-app",
		Port:    "5451",
	}); err == nil {
		t.Fatalf("expected duplicate name error")
	}

	// Mark running and edit + rename
	runner.statusByName["my_app"] = core.StatusReady
	editRes, err := svc.SaveInstance(SaveInstanceRequest{
		IsEdit:        true,
		OriginalName:  "my_app",
		Engine:        "postgres",
		Runtime:       "podman", // should remain locked to docker
		Version:       "18",
		Name:          "my_app_v2",
		ContainerName: "pg-my-app-v2",
		Port:          "5450",
		Database:      "my_app_db",
		Password:      "postgres",
		MemoryLimit:   "1G",
	})
	if err != nil {
		t.Fatalf("SaveInstance(edit) error = %v", err)
	}
	if !editRes.NeedsRestartPrompt || !editRes.VolumeChanged {
		t.Fatalf("expected NeedsRestartPrompt=true and VolumeChanged=true, got %+v", editRes)
	}
	if editRes.CleanupOldEnvPath == "" {
		t.Fatalf("expected CleanupOldEnvPath to be preserved until restart/discard")
	}

	// Verify runtime remained locked to docker
	parsed, err := core.ParseEnvFile(editRes.EnvPath)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Runtime != "docker" {
		t.Fatalf("expected runtime locked to docker, got %q", parsed.Runtime)
	}

	if err := svc.RestartAfterEdit("my_app", editRes.Name, editRes.CleanupOldEnvPath); err != nil {
		t.Fatalf("RestartAfterEdit() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(instancesDir, "my_app.env")); !os.IsNotExist(err) {
		t.Fatalf("expected old env file to be removed after RestartAfterEdit")
	}
	if len(runner.stopCalls) != 1 || runner.stopCalls[0] != "my_app" {
		t.Fatalf("expected stop call for old instance my_app, got %v", runner.stopCalls)
	}
	if len(runner.startCalls) != 1 || runner.startCalls[0] != "my_app_v2" {
		t.Fatalf("expected start call for new instance my_app_v2, got %v", runner.startCalls)
	}
}

func TestStartAndStopLogStream(t *testing.T) {
	svc, runner, _ := setupTestService(t)
	_, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "18",
		Name:    "logs_db",
		Port:    "5460",
	})
	if err != nil {
		t.Fatal(err)
	}

	runner.logsCmdFn = func(_ *core.DatabaseInstance) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", "echo line-one && echo line-two")
		}
		return exec.Command("sh", "-c", "echo line-one && echo line-two")
	}

	var mu sync.Mutex
	var lines []string
	closedCh := make(chan LogClosedEvent, 1)
	svc.SetEventEmitter(func(event string, data ...any) {
		switch event {
		case "logs:line":
			if ev, ok := data[0].(LogLineEvent); ok {
				mu.Lock()
				lines = append(lines, ev.Line)
				mu.Unlock()
			}
		case "logs:closed":
			if ev, ok := data[0].(LogClosedEvent); ok {
				select {
				case closedCh <- ev:
				default:
				}
			}
		}
	})

	if err := svc.StartLogStream("logs_db"); err != nil {
		t.Fatalf("StartLogStream() error = %v", err)
	}

	select {
	case ev := <-closedCh:
		if ev.Error != "" {
			t.Fatalf("unexpected log stream close error: %s", ev.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for logs:closed event")
	}

	mu.Lock()
	joined := strings.Join(lines, "\n")
	mu.Unlock()
	if !strings.Contains(joined, "line-one") || !strings.Contains(joined, "line-two") {
		t.Fatalf("expected streamed lines, got: %s", joined)
	}

	if err := svc.StopLogStream(); err != nil {
		t.Fatalf("StopLogStream() error = %v", err)
	}
}

func TestListInstancesSkipsInspectionWhenEngineOffline(t *testing.T) {
	svc, runner, _ := setupTestService(t)
	runner.dockerHealth = core.EngineOffline

	if _, err := svc.SaveInstance(SaveInstanceRequest{
		Engine:  "postgres",
		Runtime: "docker",
		Version: "18",
		Name:    "offline_db",
		Port:    "5465",
	}); err != nil {
		t.Fatal(err)
	}

	runner.mu.Lock()
	runner.checkStatusCalls = 0
	runner.mu.Unlock()

	list, err := svc.ListInstances()
	if err != nil {
		t.Fatalf("ListInstances() error = %v", err)
	}
	if len(list) != 1 || list[0].Status != string(core.StatusStopped) {
		t.Fatalf("expected 1 stopped instance, got %+v", list)
	}

	runner.mu.Lock()
	calls := runner.checkStatusCalls
	runner.mu.Unlock()
	if calls != 0 {
		t.Fatalf("expected CheckStatus not to be called when engine offline, got %d calls", calls)
	}
}
