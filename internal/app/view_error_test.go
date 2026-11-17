package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestErrMsgPersistsLastErrorAndSurvivesClearTick(t *testing.T) {
	t.Parallel()
	m := NewApp(t.TempDir(), config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32

	updated, cmd := m.Update(errMsg{err: errors.New("port 5432 is already allocated")})
	am := updated.(*AppModel)
	if cmd != nil {
		t.Fatal("errMsg should not auto-schedule clearStatusMsg")
	}
	if !am.statusIsErr || am.lastError != "Error: port 5432 is already allocated" {
		t.Fatalf("unexpected error state: statusIsErr=%v lastError=%q", am.statusIsErr, am.lastError)
	}

	// Stale clearStatusMsg tick must not wipe out active error.
	updated, _ = am.Update(clearStatusMsg{})
	am = updated.(*AppModel)
	if am.statusMsg != "Error: port 5432 is already allocated" || !am.statusIsErr {
		t.Fatalf("stale clearStatusMsg wiped error: statusMsg=%q statusIsErr=%v", am.statusMsg, am.statusIsErr)
	}

	// Esc dismisses the status error while keeping lastError inspectable.
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am = updated.(*AppModel)
	if am.statusMsg != "" || am.statusIsErr {
		t.Fatalf("Esc should dismiss status error, got statusMsg=%q statusIsErr=%v", am.statusMsg, am.statusIsErr)
	}
	if am.lastError == "" {
		t.Fatal("lastError should remain inspectable after dismissing status bar")
	}
}

func TestOpenAndCloseErrorModalTogglesMouse(t *testing.T) {
	t.Parallel()
	m := NewApp(t.TempDir(), config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeMain
	m.setError("Error: failed to bind port 5432")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	am := updated.(*AppModel)
	if am.mode != ModeError {
		t.Fatalf("mode=%v want ModeError", am.mode)
	}
	if cmd == nil {
		t.Fatal("expected tea.DisableMouse cmd on opening ModeError")
	}
	if got := cmd(); got != tea.DisableMouse() {
		t.Fatalf("got %T, want DisableMouse msg", got)
	}

	updated, cmd = am.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am = updated.(*AppModel)
	if am.mode != ModeMain {
		t.Fatalf("mode=%v want ModeMain", am.mode)
	}
	if cmd == nil {
		t.Fatal("expected tea.EnableMouseCellMotion cmd on closing ModeError")
	}
	if got := cmd(); got != tea.EnableMouseCellMotion() {
		t.Fatalf("got %T, want EnableMouseCellMotion msg", got)
	}
}

func TestErrorModalKeyboardLineSelectionAndCopy(t *testing.T) {
	origCopy := copyToClipboard
	var copied string
	copyToClipboard = func(s string) error {
		copied = s
		return nil
	}
	t.Cleanup(func() { copyToClipboard = origCopy })

	m := NewApp(t.TempDir(), config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 80, 28
	m.setError("first line error\nsecond line detail\nthird line tip")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if m.mode != ModeError {
		t.Fatalf("mode=%v want ModeError", m.mode)
	}

	// Copy full error with 'c' when no selection is marked.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if copied != "first line error\nsecond line detail\nthird line tip" {
		t.Fatalf("copied=%q", copied)
	}

	// Move to line 1, press Space to start selection, move to line 2, press 'c'.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if copied != "second line detail\nthird line tip" {
		t.Fatalf("range copied=%q", copied)
	}

	// First Esc clears active selection without closing modal.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeError {
		t.Fatal("first Esc with active selection should stay in ModeError")
	}

	// Move to line 0 and press Enter to copy single line.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if copied != "first line error" {
		t.Fatalf("single line copied=%q", copied)
	}

	// Select all with 'a' and copy.
	copied = ""
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if copied != "first line error\nsecond line detail\nthird line tip" {
		t.Fatalf("select all copied=%q", copied)
	}
}

func TestStatusErrorBarMouseClicksAndDoubleClick(t *testing.T) {
	origCopy := copyToClipboard
	var copied string
	copyToClipboard = func(s string) error {
		copied = s
		return nil
	}
	t.Cleanup(func() { copyToClipboard = origCopy })

	m := NewApp(t.TempDir(), config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeMain
	m.setError("Error: container failed to start due to port conflict")

	inner := screenInnerWidth(m.width)
	footerInner := inner - 2
	rowY := m.statusBarRowY()
	layout := buildStatusErrorLayout(footerInner, rowY, m.statusMsg)
	if !layout.showBtns {
		t.Fatal("expected error bar action buttons to be visible")
	}

	// 1. Single-click [C] Copy button copies full error.
	_, _, handled := m.handleStatusErrorMouseAt(tea.MouseMsg{
		X: layout.copyHit.X + 1, Y: rowY,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}, time.Now())
	if !handled {
		t.Fatal("expected [C] Copy click handled")
	}
	if copied != "Error: container failed to start due to port conflict" {
		t.Fatalf("copied=%q", copied)
	}

	// 2. Restore error and double-click error text to copy.
	copied = ""
	m.setError("Error: double click me")
	layout = buildStatusErrorLayout(footerInner, rowY, m.statusMsg)
	t0 := time.Now()
	_, _, _ = m.handleStatusErrorMouseAt(tea.MouseMsg{
		X: layout.textHit.X + 2, Y: rowY,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}, t0)
	_, _, handled = m.handleStatusErrorMouseAt(tea.MouseMsg{
		X: layout.textHit.X + 2, Y: rowY,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}, t0.Add(100*time.Millisecond))
	if !handled || copied != "Error: double click me" {
		t.Fatalf("double-click handled=%v copied=%q", handled, copied)
	}

	// 3. Restore error and single-click [v] Select button to open ModeError.
	m.setError("Error: inspect me")
	layout = buildStatusErrorLayout(footerInner, rowY, m.statusMsg)
	_, cmd, handled := m.handleStatusErrorMouseAt(tea.MouseMsg{
		X: layout.selectHit.X + 1, Y: rowY,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}, time.Now())
	if !handled || m.mode != ModeError {
		t.Fatalf("expected ModeError on [v] Select click, handled=%v mode=%v", handled, m.mode)
	}
	if cmd == nil {
		t.Fatal("expected DisableMouse cmd")
	}
}

func TestMainErrorStatusBarAndErrorModalPaintSurface(t *testing.T) {
	// SetColorProfile is process-global; do not run in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)

	m := NewApp(t.TempDir(), config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeMain
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped,
	}}
	m.selectedIndex = 0
	m.setError("Error: Ports are not available: exposing port TCP 0.0.0.0:5432 -> bind: address already in use — Tip: Port conflict")

	if full := cellsWithoutBG(m.View()); full > 10 {
		t.Fatalf("main View with error status unpainted cells=%d want <= 10", full)
	}
	if plain := stripANSI(m.View()); !strings.Contains(plain, "[v] Select") || !strings.Contains(plain, "[C] Copy") {
		t.Fatalf("expected [v] Select and [C] Copy in status bar:\n%s", plain)
	}

	_ = m.openErrorModal()
	if modalUnpainted := cellsWithoutBG(m.View()); modalUnpainted > 10 {
		t.Fatalf("ModeError View unpainted cells=%d want <= 10", modalUnpainted)
	}
}
