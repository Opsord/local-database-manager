package app

import (
	"strings"
	"testing"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

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
