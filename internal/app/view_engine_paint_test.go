package app

import (
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestEngineDockFillsSurface(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOffline
	m.podmanHealth = core.EngineOffline
	m.engineMenuIndex = 1 // Podman selected — Docker row must still fill width

	dock := cellsWithoutBG(m.viewEngineDock(50, 8))
	full := cellsWithoutBG(m.View())

	const maxDockUnpainted = 16
	const maxFullUnpainted = 48
	if dock > maxDockUnpainted {
		t.Fatalf("engine dock unpainted cells=%d want <= %d", dock, maxDockUnpainted)
	}
	if full > maxFullUnpainted {
		t.Fatalf("engine menu View unpainted cells=%d want <= %d", full, maxFullUnpainted)
	}
}
