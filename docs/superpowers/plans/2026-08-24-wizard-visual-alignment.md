# Wizard Visual Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the new-instance wizard use the same Night Owl surface contract as the main menu so the modal no longer shows black holes, short separators, raw engine ids, wrapping fields, or plaintext passwords.

**Architecture:** Reuse existing helpers (`surfaceLine`, `panelInnerWidth`, `styleTextInput`, `renderOverlay`). Do not add a new layout framework. Wizard rows become full-width surface lines; text inputs get `Background(BgSurface)`, a real `Width`, single-field focus, and password echo.

**Tech Stack:** Go 1.22+, Bubble Tea, Lip Gloss, bubbles/textinput, `github.com/muesli/termenv` (test-only TrueColor).

**Spec:** `docs/superpowers/specs/2026-08-24-wizard-visual-alignment-design.md`

## Global Constraints

- User-facing copy stays English.
- No change to `.env` persistence, step order, or validation rules.
- No new dependencies.
- Do not redesign Help/Action menus beyond the shared `renderOverlay` whitespace fill.
- Do not add back-navigation or a step counter.

## Files

- Modify: `internal/app/helpers.go` (`styleTextInput`, `renderOverlay`; add `panelSeparator` if missing)
- Modify: `internal/app/view_wizard.go` (row rendering, focus, display labels, input width, password echo)
- Create: `internal/app/view_wizard_test.go`
- Modify: `internal/app/helpers_test.go` (overlay / input style checks)

---

### Task 1: Failing tests for wizard surface and copy

**Files:**
- Create: `internal/app/view_wizard_test.go`
- Modify: `internal/app/helpers_test.go`

**Interfaces:**
- Consumes: `NewApp`, `newWizardModel`, `AppModel.View`, `styleTextInput`, `renderOverlay`
- Produces: Tests that fail on current wizard render (raw ids, `>` on every field, wrap, plaintext password, unpainted cells)

- [ ] **Step 1: Write `internal/app/view_wizard_test.go`**

```go
package app

import (
	"strings"
	"testing"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func cellsWithoutBG(view string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		inEsc := false
		var esc strings.Builder
		hasBG := false
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == 0x1b {
				inEsc = true
				esc.Reset()
				continue
			}
			if inEsc {
				esc.WriteByte(c)
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					inEsc = false
					seq := esc.String()
					if strings.Contains(seq, "48;2;") || strings.Contains(seq, "48;5;") {
						hasBG = true
					}
					if seq == "[0m" || seq == "[m" || seq == "[;m" {
						hasBG = false
					}
				}
				continue
			}
			if !hasBG {
				n++
			}
		}
	}
	return n
}

func reviewModel(width, height int) *AppModel {
	m := NewApp("/tmp")
	m.width = width
	m.height = height
	m.mode = ModeWizard
	m.instances = []*core.DatabaseInstance{{Name: "demo", Port: 5432}}
	m.wizard = newWizardModel(m.projectRoot, m.instancesDir, m.instances)
	m.wizard.step = StepReview
	m.wizard.inputs[0].SetValue("shop")
	m.wizard.inputs[1].SetValue("pg-this-is-a-very-long-container-name-that-will-overflow")
	m.wizard.inputs[2].SetValue("5433")
	m.wizard.inputs[3].SetValue("shop_db")
	m.wizard.inputs[4].SetValue("pgdata_shop")
	m.wizard.inputs[5].SetValue("postgres")
	m.wizard.inputs[6].SetValue("512M")
	return m
}

func TestWizardReviewFitsTerminal(t *testing.T) {
	t.Parallel()
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, sz := range []struct{ w, h int }{{120, 32}, {80, 24}} {
		m := reviewModel(sz.w, sz.h)
		view := m.View()
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > sz.w {
				t.Fatalf("%dx%d line %d width=%d: %q", sz.w, sz.h, i, got, stripANSI(line))
			}
		}
		plain := stripANSI(view)
		if strings.Contains(plain, "\noverflow") || strings.Contains(plain, "│ overflow") {
			t.Fatalf("long container name wrapped out of its row:\n%s", plain)
		}
	}
}

func TestWizardReviewUsesDisplayLabels(t *testing.T) {
	t.Parallel()
	plain := stripANSI(reviewModel(120, 32).View())
	if strings.Contains(plain, "postgres") || strings.Contains(plain, "docker") {
		t.Fatalf("review still shows raw ids:\n%s", plain)
	}
	if !strings.Contains(plain, "Postgres") || !strings.Contains(plain, "Docker") {
		t.Fatalf("review missing display labels:\n%s", plain)
	}
}

func TestWizardReviewHidesPasswordAndIdlePrompts(t *testing.T) {
	t.Parallel()
	plain := stripANSI(reviewModel(120, 32).View())
	if strings.Contains(plain, "postgres") {
		t.Fatalf("password leaked into view:\n%s", plain)
	}
	if strings.Count(plain, ">") > 0 {
		t.Fatalf("completed fields still show input prompt:\n%s", plain)
	}
}

func TestWizardReviewFillsSurface(t *testing.T) {
	t.Parallel()
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := reviewModel(120, 32)
	got := cellsWithoutBG(m.View())
	const maxUnpainted = 80
	if got > maxUnpainted {
		t.Fatalf("wizard review unpainted cells=%d want <= %d", got, maxUnpainted)
	}
}
```

- [ ] **Step 2: Add helper tests for overlay whitespace and input background**

Append to `internal/app/helpers_test.go` (imports: `textinput`, `termenv`):

```go
func TestRenderOverlayPaintsWhitespace(t *testing.T) {
	t.Parallel()
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := &AppModel{width: 40, height: 10}
	out := m.renderOverlay("X")
	if !strings.Contains(out, "48;2;1;22;39") {
		t.Fatalf("overlay whitespace missing BgDark: %q", out)
	}
}

func TestStyleTextInputSetsSurfaceBackground(t *testing.T) {
	t.Parallel()
	lipgloss.SetColorProfile(termenv.TrueColor)
	ti := styleTextInput(textinput.New())
	ti.SetValue("abc")
	ti.Focus()
	view := wrapInputField(ti.View())
	// BgSurface #0b253a → 11;37;58 (Lip Gloss may round to 11;36;58)
	if !strings.Contains(view, "48;2;11;37;58") && !strings.Contains(view, "48;2;11;36;58") {
		t.Fatalf("input view missing BgSurface: %q", view)
	}
}
```

- [ ] **Step 3: Run tests and confirm they fail**

Run: `go test ./internal/app -run 'TestWizardReview|TestRenderOverlay|TestStyleTextInput' -v`

Expected: FAIL on raw ids (`postgres`/`docker`), wrap (`overflow`), unpainted cells (~957), and input view without `48;2;11;…`.

- [ ] **Step 4: Commit**

```bash
git add internal/app/view_wizard_test.go internal/app/helpers_test.go
git commit -m "test(tui): capture wizard surface, label, and wrap regressions"
```

---

### Task 2: Shared surface helpers (overlay + inputs)

**Files:**
- Modify: `internal/app/helpers.go:21-23` (`renderOverlay`)
- Modify: `internal/app/helpers.go:82-92` (`styleTextInput`)
- Modify: `internal/app/helpers.go` (add `panelSeparator`)

**Interfaces:**
- Consumes: `BgDark`, `BgSurface`, Lip Gloss `Place` whitespace options
- Produces: overlay filler painted with `BgDark`; textinput styles with `Background(BgSurface)`; `panelSeparator(width int) string`

- [ ] **Step 1: Fill overlay whitespace**

Replace `renderOverlay` with:

```go
func (m *AppModel) renderOverlay(modal string) string {
	return lipgloss.Place(
		m.width-2,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceBackground(BgDark),
	)
}
```

- [ ] **Step 2: Paint textinput styles onto the surface**

```go
func styleTextInput(ti textinput.Model) textinput.Model {
	surface := lipgloss.NewStyle().Foreground(FgText).Background(BgSurface)
	ti.Prompt = ""
	ti.PromptStyle = lipgloss.NewStyle().Foreground(AccentColor).Background(BgSurface)
	ti.TextStyle = surface
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(MutedColor).Background(BgSurface)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(PrimaryColor).Background(BgSurface)
	ti.Cursor.TextStyle = surface
	return ti
}
```

- [ ] **Step 3: Add `panelSeparator`**

```go
func panelSeparator(width int) string {
	if width < 1 {
		width = 1
	}
	return SeparatorStyle.Width(width).Render(strings.Repeat("─", width))
}
```

- [ ] **Step 4: Re-run helper tests**

Run: `go test ./internal/app -run 'TestRenderOverlay|TestStyleTextInput' -v`

Expected: PASS. Wizard review tests still FAIL (layout not updated yet).

- [ ] **Step 5: Commit**

```bash
git add internal/app/helpers.go internal/app/helpers_test.go
git commit -m "fix(tui): paint overlay whitespace and text input surfaces"
```

---

### Task 3: Wizard rows, focus, labels, truncation

**Files:**
- Modify: `internal/app/view_wizard.go`

**Interfaces:**
- Consumes: `surfaceLine`, `panelInnerWidth`, `panelSeparator`, `styleTextInput`, `LabelStyle`, `ValueHighlightStyle`
- Produces: `engineDisplay(id string) string`, `runtimeDisplay(id string) string`, `(*wizardModel).blurAll()`, `(*wizardModel).focusInput(i int)`, `wizardFieldRow(...)`

- [ ] **Step 1: Display names + focus helper**

Add next to `wizardModel`:

```go
func engineDisplay(id string) string {
	if id == "sqlserver" {
		return "SQL Server"
	}
	return "Postgres"
}

func runtimeDisplay(id string) string {
	if id == "podman" {
		return "Podman"
	}
	return "Docker"
}

func (w *wizardModel) blurAll() {
	for i := range w.inputs {
		w.inputs[i].Blur()
	}
}

func (w *wizardModel) focusInput(i int) {
	w.blurAll()
	if i >= 0 && i < len(w.inputs) {
		w.inputs[i].Focus()
	}
}
```

In `newWizardModel`, after creating inputs:

```go
inputs[5].EchoMode = textinput.EchoPassword
inputs[5].EchoCharacter = '•'
for i := range inputs {
	inputs[i].Prompt = ""
	inputs[i].Width = 32
}
```

Replace every `w.inputs[n].Focus()` in `updateWizard` with `w.focusInput(n)`. On `StepEngine`, `StepRuntime`, and `StepReview`, call `w.blurAll()`.

When rendering completed engine/runtime, use `engineDisplay` / `runtimeDisplay` instead of raw ids. Reuse the same helpers in the active-step chip loop (delete the duplicated if/else labels).

- [ ] **Step 2: Render every wizard row as a surface line**

Replace the body of `viewWizard` after computing `boxWidth` with:

```go
inner := panelInnerWidth(boxWidth)
inputWidth := inner - 14 - 1
if inputWidth < 8 {
	inputWidth = 8
}
for i := range w.inputs {
	w.inputs[i].Width = inputWidth
}

row := func(parts ...string) string {
	return surfaceLine(inner, joinWithSurfaceGaps(parts, 1))
}

content := []string{
	surfaceLine(inner, TitleStyle.Render("New Database Instance")),
	panelSeparator(inner),
}

// engine
if w.step == StepEngine {
	parts := []string{LabelStyle.Render("1. Engine:")}
	for i, eng := range w.engines {
		label := engineDisplay(eng)
		if i == w.selectedEngineIdx {
			parts = append(parts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", label)))
		} else {
			parts = append(parts, NormalItemStyle.Render(fmt.Sprintf(" %s ", label)))
		}
	}
	content = append(content, row(parts...))
} else {
	content = append(content, row(LabelStyle.Render("1. Engine:"), ValueHighlightStyle.Render(engineDisplay(w.engines[w.selectedEngineIdx]))))
}
```

Same pattern for runtime.

For text steps (`Name`…`Memory`):

```go
func (m *AppModel) wizardValueRow(inner int, label, value string, inputIdx int, extra string) string {
	w := &m.wizard
	parts := []string{LabelStyle.Render(label)}
	active := w.step == wizardStep(int(StepName)+inputIdx)
	if active && w.step != StepReview {
		parts = append(parts, wrapInputField(w.inputs[inputIdx].View()))
	} else {
		parts = append(parts, ValueStyle.Render(value))
	}
	if extra != "" {
		parts = append(parts, MutedStyle.Render(extra))
	}
	return surfaceLine(inner, joinWithSurfaceGaps(parts, 1))
}
```

Password `value` in completed/review rows must be the echo (`strings.Repeat("•", len(pass))` or `w.inputs[5].View()` after blur — prefer explicit bullets so review has no cursor). Truncate with existing `truncateEnd(value, inputWidth)` so long container names cannot wrap.

Hint rows: `surfaceLine(inner, MutedStyle.Render(...))` / `RunningStyle` for review.

Keep `ActivePanelStyle.Width(boxWidth).Render(lipgloss.JoinVertical(lipgloss.Left, content...))`.

- [ ] **Step 3: Run wizard tests**

Run: `go test ./internal/app -run 'TestWizardReview|TestRenderOverlay|TestStyleTextInput' -v`

Expected: PASS. If `cellsWithoutBG` is still high, wrap the JoinVertical result in an extra `lipgloss.NewStyle().Width(inner).Background(BgSurface)` before the panel, or ensure `surfaceLine` covers title/separator/hint. Do not lower the threshold; fix the paint.

- [ ] **Step 4: Run the full package**

Run: `go test ./internal/app -count=1`

Expected: PASS (existing panel-width tests included).

- [ ] **Step 5: Commit**

```bash
git add internal/app/view_wizard.go internal/app/view_wizard_test.go
git commit -m "fix(tui): align new-instance wizard with main surface layout"
```

---

### Task 4: Package verification

**Files:** none new

- [ ] **Step 1: Run all tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Manual smoke (optional if a TTY is available)**

Run: `go run ./cmd/db-manager`, press `n`, walk Engine → Review. Confirm: filled navy panel, full-width rule, one active field without `>` forest, `Postgres`/`Docker` labels, masked password, long names stay on one row.

- [ ] **Step 3: Commit only if verification required extra fixes**
