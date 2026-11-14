package app

import (
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMainSplitGapFillsBackground(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeMain
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped,
	}}
	m.selectedIndex = 0

	// Before the fix, the 1-cell gap between left and right panels was only
	// 1 line tall, leaving (contentHeight+1) ~ 28 unpainted cells down the seam.
	full := cellsWithoutBG(m.View())
	if full > 10 {
		t.Fatalf("main View unpainted cells=%d want <= 10", full)
	}
}
