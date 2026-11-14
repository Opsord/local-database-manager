package app

import (
	"bufio"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLogsModel_VisibleLines(t *testing.T) {
	t.Parallel()

	m := logsModel{
		lines: []string{
			"PostgreSQL database server initialized",
			"LOG: database system was shut down at 2026-11-09",
			"LOG: database system is ready to accept connections",
			"ERROR: relation not found",
		},
	}

	// Without filter
	if len(m.visibleLines()) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(m.visibleLines()))
	}

	// With matching filter
	m.filterInput = "ready"
	vis := m.visibleLines()
	if len(vis) != 1 || !strings.Contains(vis[0], "ready to accept") {
		t.Fatalf("expected 1 matching line, got %v", vis)
	}

	// Case insensitive
	m.filterInput = "error"
	vis = m.visibleLines()
	if len(vis) != 1 || !strings.Contains(vis[0], "ERROR") {
		t.Fatalf("expected 1 matching line for case-insensitive error, got %v", vis)
	}

	// No matches
	m.filterInput = "nonexistent"
	vis = m.visibleLines()
	if len(vis) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(vis))
	}
}

func TestAppModel_UpdateLogsKeys(t *testing.T) {
	t.Parallel()

	app := &AppModel{
		mode: ModeLogs,
		logs: logsModel{
			viewport:   viewport.New(80, 20),
			instName:   "test-db",
			lines:      []string{"line 1", "line 2 error", "line 3"},
			autoScroll: true,
		},
	}

	// 1. Toggle pause/follow with 'f'
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	app = updated.(*AppModel)
	if app.logs.autoScroll != false {
		t.Fatalf("expected autoScroll to be false after 'f'")
	}

	// 2. Toggle pause/follow with ' '
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeySpace})
	app = updated.(*AppModel)
	if app.logs.autoScroll != true {
		t.Fatalf("expected autoScroll to be true after ' '")
	}

	// 3. Start filter with '/'
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(*AppModel)
	if !app.logs.isFiltering {
		t.Fatalf("expected isFiltering to be true after '/'")
	}

	// 4. Type 'error' in filter
	for _, r := range "error" {
		updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = updated.(*AppModel)
	}
	if app.logs.filterInput != "error" {
		t.Fatalf("expected filterInput to be 'error', got %q", app.logs.filterInput)
	}
	if len(app.logs.visibleLines()) != 1 {
		t.Fatalf("expected 1 filtered line, got %d", len(app.logs.visibleLines()))
	}

	// 5. Enter to apply filter
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(*AppModel)
	if app.logs.isFiltering {
		t.Fatalf("expected isFiltering to be false after enter")
	}
	if app.logs.filterInput != "error" {
		t.Fatalf("filterInput should remain 'error' after enter")
	}

	// 6. Esc to clear filter
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(*AppModel)
	if app.logs.filterInput != "" {
		t.Fatalf("expected filterInput to be cleared after Esc")
	}
	if app.mode != ModeLogs {
		t.Fatalf("first Esc should clear filter and stay in ModeLogs")
	}

	// 7. Esc again exits ModeLogs
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(*AppModel)
	if app.mode != ModeMain {
		t.Fatalf("second Esc should exit to ModeMain, got %v", app.mode)
	}
}

func TestAppModel_ViewLogsRendering(t *testing.T) {
	t.Parallel()

	app := &AppModel{
		width:  100,
		height: 30,
		mode:   ModeLogs,
		logs: logsModel{
			viewport:   viewport.New(90, 20),
			instName:   "postgres-dev",
			lines:      []string{"System ready"},
			autoScroll: true,
		},
	}

	view := app.viewLogs()
	if !strings.Contains(view, "Logs: postgres-dev") {
		t.Errorf("expected view to contain title, got: %s", view)
	}
	if !strings.Contains(view, "FOLLOWING") {
		t.Errorf("expected view to contain FOLLOWING badge")
	}

	app.logs.autoScroll = false
	view = app.viewLogs()
	if !strings.Contains(view, "PAUSED") {
		t.Errorf("expected view to contain PAUSED badge")
	}
}

func TestLogsStreamMultiLineScanner(t *testing.T) {
	t.Parallel()

	scanner := bufio.NewScanner(strings.NewReader("line1\nline2\nline3\n"))
	app := &AppModel{
		mode: ModeLogs,
		logs: logsModel{
			viewport:   viewport.New(80, 20),
			scanner:    scanner,
			streamID:   1,
			instName:   "postgres-dev",
			autoScroll: true,
		},
	}

	cmd := waitForLogLines(app.logs.streamID, app.logs.scanner, nil)
	for i := 0; i < 3; i++ {
		if cmd == nil {
			t.Fatalf("expected cmd for line %d, got nil", i+1)
		}
		msg := cmd()
		lineMsg, ok := msg.(logLineMsg)
		if !ok {
			t.Fatalf("expected logLineMsg at iteration %d, got %T", i+1, msg)
		}
		updated, nextCmd := app.Update(lineMsg)
		app = updated.(*AppModel)
		cmd = nextCmd
	}

	if len(app.logs.lines) != 3 {
		t.Fatalf("expected 3 lines, got %d (%v)", len(app.logs.lines), app.logs.lines)
	}
	if app.logs.lines[0] != "line1" || app.logs.lines[1] != "line2" || app.logs.lines[2] != "line3" {
		t.Fatalf("unexpected lines: %v", app.logs.lines)
	}

	// Next read should emit logClosedMsg once
	if cmd == nil {
		t.Fatal("expected cmd after third line")
	}
	endMsg := cmd()
	closed, ok := endMsg.(logClosedMsg)
	if !ok {
		t.Fatalf("expected logClosedMsg, got %T", endMsg)
	}

	// Stale streamID messages must be ignored
	updated, _ := app.Update(logLineMsg{streamID: 999, line: "stale"})
	app = updated.(*AppModel)
	if len(app.logs.lines) != 3 {
		t.Fatalf("stale streamID line should be ignored, got %v", app.logs.lines)
	}

	// Matching streamID close appends end marker
	updated, _ = app.Update(closed)
	app = updated.(*AppModel)
	if len(app.logs.lines) != 4 || app.logs.lines[3] != "--- End of logs stream ---" {
		t.Fatalf("expected end marker, got %v", app.logs.lines)
	}
}
