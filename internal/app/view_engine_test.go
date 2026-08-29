package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestEngineMenuOpensOnE(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeMain
	m.dockerHealth = core.EngineOffline
	m.podmanHealth = core.EngineNotInstalled

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	am := updated.(*AppModel)
	if am.mode != ModeEngineMenu {
		t.Fatalf("mode=%v, want ModeEngineMenu", am.mode)
	}
}

func TestEngineMenuDisabledRows(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.podmanHealth = core.EngineNotInstalled
	plain := stripANSI(m.viewEngineDock(40, 10))
	if !strings.Contains(plain, "Stop Docker") {
		t.Fatalf("expected Stop Docker label:\n%s", plain)
	}
	if !strings.Contains(plain, "not installed") {
		t.Fatalf("expected not installed label:\n%s", plain)
	}
}

func TestEngineMenuDockedInLeftPanel(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOffline
	m.podmanHealth = core.EngineOffline
	m.instances = []*core.DatabaseInstance{{Name: "demo", EngineType: "postgres", Runtime: "docker"}}
	m.selectedIndex = 0

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "LOCAL DATABASE MANAGER") {
		t.Fatal("expected header")
	}
	if !strings.Contains(plain, "DB Instances") {
		t.Fatal("expected instance list")
	}
	if !strings.Contains(plain, "Container Engines") && !strings.Contains(plain, "Engines") {
		t.Fatal("expected engines dock title")
	}
	if !strings.Contains(plain, "Details & Config") {
		t.Fatal("expected right details")
	}
}

func TestEngineDockedLeftMatchesRightHeight(t *testing.T) {
	t.Parallel()
	inner := 118
	leftW, rightW, _ := splitPanelWidths(inner)
	contentH := 27
	rightBox := panelBoxStyle(false).Width(rightW).Height(contentH).Render("right")
	leftInner := panelInnerWidth(leftW)
	top, bottom := splitPanelHalfHeight(contentH - 1)
	listBlock := lipgloss.NewStyle().Width(leftInner).Height(top).MaxHeight(top).Render("list")
	engBlock := lipgloss.NewStyle().Width(leftInner).Height(bottom).MaxHeight(bottom).Render("eng")
	leftCol := ActivePanelStyle.Width(leftW).Height(contentH).Render(
		lipgloss.JoinVertical(lipgloss.Left, listBlock, panelSeparator(leftInner), engBlock),
	)
	if lipgloss.Height(leftCol) != lipgloss.Height(rightBox) {
		t.Fatalf("left=%d right=%d", lipgloss.Height(leftCol), lipgloss.Height(rightBox))
	}
}

func TestOfflineStartOffersEngineRetry(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "demo", Runtime: "podman", EngineType: "postgres", Status: core.StatusStopped,
	}
	sr := &stubRunner{startErr: fmt.Errorf("%w: podman", core.ErrEngineOffline)}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeMain
	m.instances = []*core.DatabaseInstance{inst}
	m.selectedIndex = 0
	m.runner = sr

	msg := m.toggleInstanceCmd(inst)()
	if _, ok := msg.(offlineStartMsg); !ok {
		t.Fatalf("got %T, want offlineStartMsg", msg)
	}

	updated, _ := m.Update(msg)
	am := updated.(*AppModel)
	if !am.confirmEngineStart {
		t.Fatal("confirmEngineStart not set")
	}
	if am.pendingStartInst != inst {
		t.Fatal("pendingStartInst not set")
	}
	if am.pendingEngineRuntime != "podman" {
		t.Fatalf("pendingEngineRuntime=%q, want podman", am.pendingEngineRuntime)
	}
	if !strings.Contains(am.statusMsg, "Podman is offline") {
		t.Fatalf("status=%q", am.statusMsg)
	}
}

func TestOfflineStartConfirmStartsEngine(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "demo", Runtime: "podman", EngineType: "postgres", Status: core.StatusStopped,
	}
	sr := &stubRunner{}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.runner = sr
	m.confirmEngineStart = true
	m.pendingStartInst = inst
	m.pendingEngineRuntime = "podman"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am := updated.(*AppModel)
	if am.confirmEngineStart {
		t.Fatal("confirmEngineStart should be cleared")
	}
	if !am.engineStarting {
		t.Fatal("engineStarting should be true")
	}
	if cmd == nil {
		t.Fatal("expected startEngineCmd")
	}
	started, ok := cmd().(engineStartedMsg)
	if !ok {
		t.Fatalf("got %T, want engineStartedMsg", cmd())
	}
	if started.runtime != "podman" || started.retryInst != inst {
		t.Fatalf("engineStartedMsg={runtime:%q retryInst:%v}", started.runtime, started.retryInst)
	}
}

func TestOfflineStartCancel(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker"}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.confirmEngineStart = true
	m.pendingStartInst = inst
	m.pendingEngineRuntime = "docker"
	m.statusMsg = "Docker is offline. Start engine and retry? Press 'y' to confirm, 'n' to cancel"
	m.statusIsErr = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	am := updated.(*AppModel)
	if am.confirmEngineStart {
		t.Fatal("confirmEngineStart should be cleared")
	}
	if am.pendingStartInst != nil {
		t.Fatal("pendingStartInst should be cleared")
	}
	if am.mode == ModeWizard {
		t.Fatal("cancel should not open wizard")
	}
	if cmd == nil {
		t.Fatal("expected status clear tick")
	}
}

func TestEngineOnlineRowShowsStop(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.podmanHealth = core.EngineOffline
	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "Stop Docker") {
		t.Fatalf("expected Stop Docker:\n%s", plain)
	}
	if !strings.Contains(plain, "Start Podman") {
		t.Fatalf("expected Start Podman:\n%s", plain)
	}
}

func TestEngineStopRequiresConfirm(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.engineMenuIndex = 0 // docker
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if !am.confirmEngineStop {
		t.Fatal("expected confirmEngineStop armed")
	}
	if cmd != nil {
		// Enter must not start stop cmd yet
		t.Fatal("Enter on online must not dispatch stop yet")
	}
	updated, cmd = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am = updated.(*AppModel)
	if !am.engineStarting {
		t.Fatal("expected busy after y")
	}
	if cmd == nil {
		t.Fatal("expected stopEngineCmd")
	}
}

func TestMainFooterShowsEnginesShortcut(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "[e]") || !strings.Contains(plain, "Engines") {
		t.Fatalf("footer missing Engines shortcut:\n%s", plain)
	}
}
