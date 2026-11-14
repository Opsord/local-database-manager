package app

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

type stubRunner struct {
	docker                 core.EngineHealth
	podman                 core.EngineHealth
	startErr               error
	downVolumesErr         error
	downVolumesCalls       int
	lastStartEngineRuntime string
	lastStopInst           *core.DatabaseInstance
	lastStartInst          *core.DatabaseInstance
}

func (s *stubRunner) CheckEngineHealth(_ context.Context, runtimeName string) core.EngineHealth {
	if runtimeName == "podman" {
		return s.podman
	}
	return s.docker
}

func (s *stubRunner) StartEngine(_ context.Context, runtime string) error {
	s.lastStartEngineRuntime = runtime
	return nil
}
func (s *stubRunner) StopEngine(context.Context, string) error { return nil }
func (s *stubRunner) Start(_ context.Context, inst *core.DatabaseInstance) error {
	s.lastStartInst = inst
	if s.startErr != nil {
		return s.startErr
	}
	return nil
}
func (s *stubRunner) Stop(_ context.Context, inst *core.DatabaseInstance) error {
	s.lastStopInst = inst
	return nil
}
func (s *stubRunner) DownVolumes(context.Context, *core.DatabaseInstance) error {
	s.downVolumesCalls++
	return s.downVolumesErr
}
func (s *stubRunner) CheckStatus(context.Context, *core.DatabaseInstance) core.ContainerStatus {
	return core.StatusStopped
}
func (s *stubRunner) GetMemoryUsage(context.Context, *core.DatabaseInstance) string {
	return "-"
}
func (s *stubRunner) LogsCommand(*core.DatabaseInstance) *exec.Cmd { return nil }

func TestEngineHealthMsgUpdatesBadges(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		dockerHealth: core.EngineOffline,
		podmanHealth: core.EngineOffline,
	}

	updated, cmd := m.Update(engineHealthMsg{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineNotInstalled,
		scheduleTick: true,
	})
	am := updated.(*AppModel)

	if am.dockerHealth != core.EngineOnline {
		t.Fatalf("dockerHealth = %q, want ONLINE", am.dockerHealth)
	}
	if am.podmanHealth != core.EngineNotInstalled {
		t.Fatalf("podmanHealth = %q, want NOT_INSTALLED", am.podmanHealth)
	}
	if cmd == nil {
		t.Fatal("expected a follow-up tick so daemon health keeps refreshing")
	}
}

func TestEngineHealthTickRunsProbe(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		runner: &stubRunner{
			docker: core.EngineOnline,
			podman: core.EngineOffline,
		},
	}

	_, cmd := m.Update(engineHealthTickMsg{})
	if cmd == nil {
		t.Fatal("expected health probe command")
	}

	msg := cmd()
	got, ok := msg.(engineHealthMsg)
	if !ok {
		t.Fatalf("got %T, want engineHealthMsg", msg)
	}
	if got.dockerHealth != core.EngineOnline {
		t.Fatalf("dockerHealth = %q, want ONLINE", got.dockerHealth)
	}
	if got.podmanHealth != core.EngineOffline {
		t.Fatalf("podmanHealth = %q, want OFFLINE", got.podmanHealth)
	}
}

func TestCheckEngineHealthCmdDoesNotDependOnInstanceScan(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		runner: &stubRunner{
			docker: core.EngineNotInstalled,
			podman: core.EngineOnline,
		},
	}

	msg := m.checkEngineHealthCmd(true)()
	got, ok := msg.(engineHealthMsg)
	if !ok {
		t.Fatalf("got %T, want engineHealthMsg", msg)
	}
	if got.dockerHealth != core.EngineNotInstalled || got.podmanHealth != core.EngineOnline {
		t.Fatalf("got docker=%q podman=%q", got.dockerHealth, got.podmanHealth)
	}
}

func TestEngineHealthTickUsesConfiguredInterval(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		cfg: config.Config{EngineHealthInterval: 42 * time.Second},
		dockerHealth: core.EngineOffline,
		podmanHealth: core.EngineOffline,
	}

	updated, cmd := m.Update(engineHealthMsg{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineOnline,
		scheduleTick: true,
	})
	if cmd == nil {
		t.Fatal("expected tick cmd")
	}
	_ = updated

	am := updated.(*AppModel)
	if am.cfg.EngineHealthInterval != 42*time.Second {
		t.Fatalf("interval = %v, want 42s", am.cfg.EngineHealthInterval)
	}
}

func TestEngineHealthMsgScheduleTick(t *testing.T) {
	t.Parallel()

	m := &AppModel{}

	_, cmd := m.Update(engineHealthMsg{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineOnline,
		scheduleTick: false,
	})
	if cmd != nil {
		t.Fatal("scheduleTick:false should not return follow-up tick cmd")
	}

	_, cmd = m.Update(engineHealthMsg{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineOnline,
		scheduleTick: true,
	})
	if cmd == nil {
		t.Fatal("scheduleTick:true should return follow-up tick cmd")
	}
}

func TestInitStartsEngineHealthProbe(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		runner:       &stubRunner{docker: core.EngineOnline, podman: core.EngineOffline},
		instancesDir: t.TempDir(),
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should start instance reload and engine health polling")
	}

	msg := cmd()
	if _, ok := msg.(tea.BatchMsg); !ok {
		t.Fatalf("Init cmd produced %T, want tea.BatchMsg (reload + health probe)", msg)
	}
}
