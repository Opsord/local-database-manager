package app

import (
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestActionDockFillsSurface(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeActionMenu
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped,
	}}
	m.selectedIndex = 0

	// Longest row (purge) used to punch the most holes with nested styles.
	m.actionMenuIndex = len(m.getActionMenuItems(m.instances[0])) - 1
	dock := cellsWithoutBG(m.viewActionDock(50, 12))
	full := cellsWithoutBG(m.View())

	const maxDockUnpainted = 24
	const maxFullUnpainted = 48
	if dock > maxDockUnpainted {
		t.Fatalf("action dock unpainted cells=%d want <= %d", dock, maxDockUnpainted)
	}
	if full > maxFullUnpainted {
		t.Fatalf("action menu View unpainted cells=%d want <= %d (wizard baseline ~28)", full, maxFullUnpainted)
	}
}
