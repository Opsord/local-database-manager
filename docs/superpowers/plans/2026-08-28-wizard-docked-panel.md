# Docked Wizard (Right Panel Bottom) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed the new-instance wizard in the bottom half of the right panel while keeping the main split layout visible; keyboard focus stays on the wizard until cancel or create.

**Architecture:** `ModeWizard` renders `viewMain()` instead of a full-screen overlay. `view_main.go` splits the right panel ~50/50 (details top, wizard dock bottom). `view_wizard.go` builds dock-sized content with a bubbles `viewport` for the scrollable body and a fixed hints row at the dock footer. Focus changes call `syncScrollToFocus()`.

**Tech Stack:** Go 1.22+, Bubble Tea, bubbles (`viewport`, `textinput`), Lip Gloss, existing `internal/app` helpers.

**Spec:** `docs/superpowers/specs/2026-08-28-wizard-docked-panel-design.md`

## Global Constraints

- All user-facing strings remain English.
- Do not change help, action menu, or logs overlay behavior.
- Wizard field order, unlock rules, lateral/vertical keys, engine-default recalc, and `.env` save behavior stay as in the vertical-nav wizard (see `docs/superpowers/specs/2026-08-28-wizard-nav-surface-design.md`).
- `ModeWizard` must not use full-screen `renderOverlay` for the wizard.
- While wizard is open: list and details visible but inactive; footer shortcuts show wizard hints.
- Bottom dock: fixed height (~half right panel); hints fixed at dock bottom; body scrolls above hints.
- Every inner wizard fragment uses `Background(BgSurface)` (Night Owl rule).
- TDD: failing test first; `go test ./internal/app/... -v` after each step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/app/helpers.go` | `splitPanelHalfHeight`, optional `rightPanelSplitHeights` |
| `internal/app/helpers_test.go` | Height split tests |
| `internal/app/tui.go` | `View()` routing; `WindowSizeMsg` wizard viewport sizing |
| `internal/app/view_main.go` | Right-panel split when `ModeWizard`; wizard footer shortcuts |
| `internal/app/view_wizard.go` | `viewport` on `wizardModel`, dock render, scroll sync, body/hints split |
| `internal/app/view_wizard_test.go` | Dock layout, scroll, footer tests |
| `internal/app/tui_test.go` | Optional `View()` integration test |

---

### Task 1: Height helpers + View routing

**Files:**
- Modify: `internal/app/helpers.go`
- Modify: `internal/app/helpers_test.go`
- Modify: `internal/app/tui.go`

**Interfaces:**
- Consumes: `mainContentHeight(termHeight int) int`
- Produces:
  - `func splitPanelHalfHeight(contentHeight int) (top, bottom int)` — `top = contentHeight/2`, `bottom = contentHeight - top`, each at least 3
  - `ModeWizard` in `View()` returns `m.wrapScreen(m.viewMain())` (not `renderOverlay(viewWizard)`)

- [ ] **Step 1: Write failing tests**

Append to `internal/app/helpers_test.go`:

```go
func TestSplitPanelHalfHeight(t *testing.T) {
	t.Parallel()
	top, bottom := splitPanelHalfHeight(20)
	if top != 10 || bottom != 10 {
		t.Fatalf("20 -> top=%d bottom=%d, want 10/10", top, bottom)
	}
	top, bottom = splitPanelHalfHeight(11)
	if top != 5 || bottom != 6 {
		t.Fatalf("11 -> top=%d bottom=%d, want 5/6", top, bottom)
	}
	top, bottom = splitPanelHalfHeight(4)
	if top < 3 || bottom < 3 {
		t.Fatalf("min heights: top=%d bottom=%d", top, bottom)
	}
}
```

Append to `internal/app/view_wizard_test.go`:

```go
func TestViewWizardUsesMainLayout(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.instances = []*core.DatabaseInstance{
		{Name: "demo", EngineType: "postgres", Runtime: "docker"},
	}
	m.selectedIndex = 0

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "LOCAL DATABASE MANAGER") {
		t.Fatal("expected main header visible in wizard mode")
	}
	if !strings.Contains(plain, "New Database Instance") {
		t.Fatal("expected wizard title in wizard mode")
	}
	if !strings.Contains(plain, "DB Instances") {
		t.Fatal("expected left panel in wizard mode")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/... -run 'TestSplitPanelHalfHeight|TestViewWizardUsesMainLayout' -v`

Expected: FAIL — `splitPanelHalfHeight` undefined; `View()` still overlay-only (may fail on header if overlay hides it).

- [ ] **Step 3: Implement**

In `internal/app/helpers.go`:

```go
func splitPanelHalfHeight(contentHeight int) (top, bottom int) {
	top = contentHeight / 2
	bottom = contentHeight - top
	if top < 3 {
		top = 3
	}
	if bottom < 3 {
		bottom = 3
	}
	if top+bottom > contentHeight {
		top = contentHeight - bottom
		if top < 3 {
			top = 3
			bottom = contentHeight - top
		}
	}
	return top, bottom
}
```

In `internal/app/tui.go`, change `View()`:

```go
case ModeWizard:
	return m.wrapScreen(m.viewMain())
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -run 'TestSplitPanelHalfHeight|TestViewWizardUsesMainLayout' -v`

Expected: PASS (wizard title may still be missing until Task 2 — if `TestViewWizardUsesMainLayout` still fails on wizard title, proceed to Task 2 before claiming Task 1 complete).

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add internal/app/helpers.go internal/app/helpers_test.go internal/app/tui.go internal/app/view_wizard_test.go
git commit -m "refactor(tui): route ModeWizard through viewMain"
```

---

### Task 2: Split right panel (details + dock shell)

**Files:**
- Modify: `internal/app/view_main.go`
- Modify: `internal/app/view_wizard.go`
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: `splitPanelHalfHeight`, `m.mode`, `viewWizardDock(rightWidth, dockHeight int) string`
- Produces:
  - `func (m *AppModel) buildRightDetailsContent(rightInner, rightWidth int) string`
  - When `m.mode == ModeWizard`: right column = vertical join of details box (top half) + wizard dock box (bottom half)
  - `func (m *AppModel) viewWizardDock(rightWidth, dockHeight int) string` — initial shell calling existing row builders (hints still inline for now)

- [ ] **Step 1: Write failing test**

Append to `internal/app/view_wizard_test.go`:

```go
func TestWizardDockedShowsDetailsAndWizard(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.instances = []*core.DatabaseInstance{
		{Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped},
	}
	m.selectedIndex = 0

	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "Details & Config") {
		t.Fatal("expected details panel title")
	}
	if !strings.Contains(plain, "demo") {
		t.Fatal("expected selected instance name in details area")
	}
	if !strings.Contains(plain, "New Database Instance") {
		t.Fatal("expected wizard in dock")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/... -run TestWizardDockedShowsDetailsAndWizard -v`

Expected: FAIL — wizard not in `viewMain`.

- [ ] **Step 3: Implement**

In `view_main.go`, extract details building into a helper:

```go
func (m *AppModel) buildRightDetailsContent(rightInner, rightWidth int) string {
	inst := m.selectedInstance()
	if inst == nil {
		return surfaceLine(rightInner, MutedStyle.Render("Select an instance from the left list to view details."))
	}
	codeBoxWidth := rightInner - 16
	if codeBoxWidth < 20 {
		codeBoxWidth = 20
	}
	details := renderDetailRows(inst, rightWidth)
	for i, row := range details {
		details[i] = surfaceLine(rightInner, row)
	}
	details = append(details, surfaceBlankLine(rightInner))
	details = append(details, surfaceLine(rightInner, detailField("URI:",
		lipgloss.JoinHorizontal(lipgloss.Top,
			URIBoxStyle.Render(truncateMiddle(inst.ConnectionURI(), codeBoxWidth)),
			surfaceGap(1),
			MutedStyle.Render("[c] copy"),
		),
	)))
	details = append(details, surfaceLine(rightInner, detailField("CLI:",
		CLIBoxStyle.Render(truncateMiddle(inst.CLICommand(), codeBoxWidth)),
	)))
	return lipgloss.JoinVertical(lipgloss.Left, details...)
}
```

Replace right-panel block in `viewMain()`:

```go
detailsTopHeight, wizardDockHeight := contentHeight, 0
if m.mode == ModeWizard {
	detailsTopHeight, wizardDockHeight = splitPanelHalfHeight(contentHeight)
}

detailsContent := m.buildRightDetailsContent(rightInner, rightWidth)
detailsBox := panelBoxStyle(false).
	Width(rightWidth).
	Height(detailsTopHeight).
	Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			panelTitle("Details & Config", rightInner),
			detailsContent,
		),
	)

var rightColumn string
if m.mode == ModeWizard {
	wizardDock := m.viewWizardDock(rightWidth, wizardDockHeight)
	rightColumn = lipgloss.JoinVertical(lipgloss.Left, detailsBox, wizardDock)
} else {
	rightBox := panelBoxStyle(false).
		Width(rightWidth).
		Height(contentHeight).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				panelTitle("Details & Config", rightInner),
				detailsContent,
			),
		)
	rightColumn = rightBox
}
```

Use `rightColumn` instead of `rightBox` in `mainSplit`.

In `view_wizard.go`, add `viewWizardDock` — refactor `viewWizard` body to use `rightWidth` and `dockHeight` instead of centered `boxWidth`:

```go
func (m *AppModel) viewWizardDock(rightWidth, dockHeight int) string {
	inner := panelInnerWidth(rightWidth)
	// Reuse existing row-building from viewWizard; width = rightWidth, height = dockHeight
	// Return ActivePanelStyle.Width(rightWidth).Height(dockHeight).Render(...)
}
```

Rename or delegate: `viewWizard()` can call `viewWizardDock` with defaults for tests that still call it directly.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -v`

Expected: all pass (fix any tests that assumed overlay-only wizard).

- [ ] **Step 5: Commit** (if user asked)

---

### Task 3: Wizard footer shortcuts in main status bar

**Files:**
- Modify: `internal/app/view_main.go`
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: existing shortcut bar builder in `viewMain`
- Produces: `func wizardShortcutBar(inner int) string` — wizard hint copy

- [ ] **Step 1: Write failing test**

```go
func TestWizardDockedFooterShowsWizardHints(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)

	plain := stripANSI(m.viewMain())
	if strings.Contains(plain, "[n]") && strings.Contains(plain, "New") {
		t.Fatal("main [n] New shortcut should not appear while wizard is open")
	}
	if !strings.Contains(plain, "[Esc]") {
		t.Fatal("expected wizard cancel hint in footer area")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

- [ ] **Step 3: Implement**

In `view_main.go` footer section:

```go
var shortcuts []string
if m.mode == ModeWizard {
	shortcuts = wizardShortcutEntries()
} else {
	shortcuts = mainShortcutEntries() // extract existing slice
}
shortcutsBar := formatShortcutBar(inner-2, shortcuts)
```

```go
func wizardShortcutEntries() []string {
	return []string{
		shortcut("[↑↓]", "Rows"),
		shortcut("[←→]", "Options"),
		shortcut("[Enter]", "Next"),
		shortcut("[b]", "Back"),
		shortcut("[Esc]", "Cancel"),
	}
}
```

- [ ] **Step 4: Run tests — expect PASS**

- [ ] **Step 5: Commit** (if user asked)

---

### Task 4: Viewport scroll (body + fixed hints)

**Files:**
- Modify: `internal/app/view_wizard.go`
- Modify: `internal/app/view_wizard_test.go`
- Modify: `internal/app/tui.go` (resize)

**Interfaces:**
- Consumes: `github.com/charmbracelet/bubbles/viewport`
- Produces:
  - `wizardModel.scrollViewport viewport.Model`
  - `func (w *wizardModel) buildWizardBody(inner, inputWidth int) string`
  - `func (w *wizardModel) wizardHintsLine(inner int) string`
  - `func (w *wizardModel) syncScrollToFocus()`
  - `func (w *wizardModel) bodyLineForStep(step wizardStep) int`
  - Calls to `syncScrollToFocus()` from `setFocus`, `moveFocus`, `confirmAdvance` (after step changes)

- [ ] **Step 1: Write failing tests**

```go
func TestWizardScrollFollowsFocus(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.scrollViewport = viewport.New(60, 4) // small viewport
	w.maxReached = StepReview
	w.setFocus(StepMemoryLimit)
	w.syncScrollToFocus()
	if w.scrollViewport.YOffset == 0 {
		t.Fatal("expected scroll offset > 0 when focusing late step in small viewport")
	}
}

func TestWizardDockFixedHintsOutsideViewport(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 80
	m.height = 20 // small terminal -> short dock
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.wizard.maxReached = StepReview
	m.wizard.step = StepReview

	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "[Esc] cancel") || !strings.Contains(plain, "All set!") {
		t.Fatal("review hints should appear in dock/footer")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

- [ ] **Step 3: Implement**

Add to `wizardModel`:

```go
scrollViewport viewport.Model
```

In `newWizardModel`, after inputs setup:

```go
scrollViewport: viewport.New(60, 8),
```

Split `viewWizardDock`:

```go
func (m *AppModel) viewWizardDock(rightWidth, dockHeight int) string {
	w := &m.wizard
	inner := panelInnerWidth(rightWidth)
	inputWidth := inner - 14 - 1
	if inputWidth < 8 {
		inputWidth = 8
	}
	for i := range w.inputs {
		w.inputs[i].Width = inputWidth
	}

	hints := w.wizardHintsLine(inner)
	hintsLines := strings.Count(hints, "\n") + 1
	// dock inner: subtract panel border (2) and hints
	scrollHeight := dockHeight - 2 - hintsLines
	if scrollHeight < 1 {
		scrollHeight = 1
	}

	body := w.buildWizardBody(m, inner, inputWidth)
	w.scrollViewport.Width = inner
	w.scrollViewport.Height = scrollHeight
	w.scrollViewport.SetContent(body)
	w.syncScrollToFocus()

	scrollBox := surfaceLine(inner, w.scrollViewport.View())
	dockInner := lipgloss.JoinVertical(lipgloss.Left, scrollBox, hints)

	return ActivePanelStyle.
		Width(rightWidth).
		Height(dockHeight).
		Render(dockInner)
}
```

Move row-building from `viewWizard` into `buildWizardBody` (title, separator, unlocked rows — **no** hints).

`wizardHintsLine` — same strings as today’s footer inside wizard (review vs nav hints), each line `surfaceLine(inner, ...)`.

`bodyLineForStep`:

```go
func (w *wizardModel) bodyLineForStep(step wizardStep) int {
	line := 3 // title, separator, blank
	for s := StepEngine; s <= step; s++ {
		if s <= w.maxReached {
			line++
		}
	}
	return line - 1
}
```

`syncScrollToFocus`:

```go
func (w *wizardModel) syncScrollToFocus() {
	target := w.bodyLineForStep(w.step)
	if target < w.scrollViewport.YOffset {
		w.scrollViewport.YOffset = target
	}
	if target >= w.scrollViewport.YOffset+w.scrollViewport.Height {
		w.scrollViewport.YOffset = target - w.scrollViewport.Height + 1
	}
	if w.scrollViewport.YOffset < 0 {
		w.scrollViewport.YOffset = 0
	}
}
```

Call `syncScrollToFocus()` at end of `setFocus`, `moveFocus`, and `confirmAdvance`.

In `tui.go` `WindowSizeMsg`, when `m.mode == ModeWizard`, no extra work needed if dock sizes from `viewMain` each frame — viewport is resized in `viewWizardDock` each `View()` call.

- [ ] **Step 4: Run full app tests**

Run: `go test ./internal/app/... -v`

Expected: PASS

- [ ] **Step 5: Commit** (if user asked)

```bash
git add internal/app/view_wizard.go internal/app/view_wizard_test.go internal/app/view_main.go internal/app/tui.go
git commit -m "feat(tui): dock wizard with scrollable body and fixed hints"
```

---

### Task 5: Regression sweep + manual check

**Files:** none new

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`

Expected: PASS

- [ ] **Step 2: Manual smoke test**

Run: `go run ./cmd/db-manager`

1. Select an instance — details show top-right.
2. Press `n` — wizard appears bottom-right; header and list remain.
3. Navigate through steps — focused row stays visible when dock is short (resize terminal small).
4. `Esc` — full-height details return; `n` again — scroll resets at Engine.

- [ ] **Step 3: Commit plan doc** (if user asked)

```bash
git add docs/superpowers/plans/2026-08-28-wizard-docked-panel.md
git commit -m "docs: plan docked wizard in right panel bottom"
```

---

## Spec self-review (plan vs spec)

| Spec requirement | Task |
|------------------|------|
| Main chrome stays (header, list, footer) | Task 1–2 |
| Right panel 50/50 split | Task 2 (`splitPanelHalfHeight`) |
| Details top / wizard bottom | Task 2 |
| No full-screen overlay for wizard | Task 1 |
| Keys locked to wizard | Already in `updateWizard`; unchanged |
| Footer wizard shortcuts | Task 3 |
| Fixed hints + scrollable body | Task 4 |
| Scroll follows focus | Task 4 |
| Existing wizard logic unchanged | Tasks 2–4 only touch layout/render |
| Tests: header + details + wizard together | Task 1–2 |
| Tests: scroll on focus | Task 4 |

No placeholders remain in task steps.
