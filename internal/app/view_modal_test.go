package app

import (
	"strings"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestActionMenuDockedInRightPanel(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeActionMenu
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped,
	}}
	m.selectedIndex = 0

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "LOCAL DATABASE MANAGER") {
		t.Fatal("expected header")
	}
	if !strings.Contains(plain, "DB Instances") {
		t.Fatal("expected instance list")
	}
	if !strings.Contains(plain, "Details & Config") {
		t.Fatal("expected details")
	}
	if !strings.Contains(plain, "Actions:") && !strings.Contains(plain, "Actions") {
		t.Fatal("expected actions dock title")
	}
	if !strings.Contains(plain, "demo") {
		t.Fatal("expected instance name visible")
	}
}

func TestActionDockedRightMatchesLeftHeight(t *testing.T) {
	t.Parallel()
	inner := 118
	leftW, rightW, _ := splitPanelWidths(inner)
	contentH := 27
	leftBox := panelBoxStyle(false).Width(leftW).Height(contentH).Render("left")
	rightInner := panelInnerWidth(rightW)
	top, bottom := splitPanelHalfHeight(contentH - 1)
	detailsBlock := lipgloss.NewStyle().Width(rightInner).Height(top).MaxHeight(top).Render("details")
	actionBlock := lipgloss.NewStyle().Width(rightInner).Height(bottom).MaxHeight(bottom).Render("actions")
	rightCol := ActivePanelStyle.Width(rightW).Height(contentH).Render(
		lipgloss.JoinVertical(lipgloss.Left, detailsBlock, panelSeparator(rightInner), actionBlock),
	)
	if lipgloss.Height(leftBox) != lipgloss.Height(rightCol) {
		t.Fatalf("left=%d right=%d", lipgloss.Height(leftBox), lipgloss.Height(rightCol))
	}
}

func TestActionMenuIncludesDownVolumes(t *testing.T) {
	t.Parallel()

	m := &AppModel{}
	inst := &core.DatabaseInstance{Name: "demo", EngineType: "postgres", Status: core.StatusReady}
	items := m.getActionMenuItems(inst)

	var found *actionMenuItem
	for i := range items {
		if items[i].shortcut == "d" {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatal("action menu is missing down -v (shortcut d)")
	}
	blob := strings.ToLower(found.label + " " + found.description)
	if !strings.Contains(blob, "volume") {
		t.Fatalf("d action should wipe volumes, got %q / %q", found.label, found.description)
	}
}

func TestActionMenuKeyDConfirmsPurge(t *testing.T) {
	t.Parallel()

	inst := &core.DatabaseInstance{Name: "demo", EngineType: "postgres", Status: core.StatusReady}
	m := &AppModel{
		mode:          ModeActionMenu,
		instances:     []*core.DatabaseInstance{inst},
		selectedIndex: 0,
	}

	updated, _ := m.updateActionMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am, ok := updated.(*AppModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if !am.confirmPurge {
		t.Fatal("pressing d in the action menu should start the down -v confirm")
	}
	if am.mode != ModeMain {
		t.Fatalf("mode = %v, want ModeMain", am.mode)
	}
}
