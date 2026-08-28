# Wizard Nav + Surface Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the new-instance wizard use lateral keys for Engine/Runtime, vertical focus among unlocked rows with `b`/empty-Backspace to go back, recalculate engine defaults only when still at prior defaults, and keep the wizard free of black patches via `BgSurface`.

**Architecture:** Extend `wizardModel` with `maxReached` (highest unlocked step). `step` remains the focused row. Pure helpers on `wizardModel` handle focus moves, option cycling, and engine-default recalculation. `updateWizard` wires keys; `viewWizard` shows chips when an option row is focused and updates the footer.

**Tech Stack:** Go 1.22, Bubble Tea, Lip Gloss, existing `internal/app` helpers (`surfaceLine`, `styleTextInput`).

**Spec:** `docs/superpowers/specs/2026-08-28-wizard-nav-surface-design.md`

## Global Constraints

- All user-facing wizard strings remain English.
- Do not redesign help, action menu, or logs beyond reusing helpers.
- Engine recalc only when fields still hold the previous engine’s defaults; Runtime change does not recalc.
- Unlock: on open only Engine; Enter unlocks next; `↑` cannot enter locked rows.
- Lateral: `←`/`→`/`h`/`l` on Engine/Runtime only. Vertical: `↑`/`↓`/`k`/`j` among unlocked rows. Back: `b`, or Backspace on empty text input.
- Every inner wizard fragment uses `Background(BgSurface)` (same Night Owl rule as main menu).
- TDD: failing test first; `go test` after each implementation step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/app/view_wizard.go` | State (`maxReached`), nav helpers, recalc, `updateWizard`, `viewWizard` |
| `internal/app/view_wizard_test.go` | Nav, recalc, footer, surface tests (extend existing file) |
| `internal/app/helpers.go` | Only if a tiny shared helper is needed (prefer not) |

---

### Task 1: Focus / unlock / lateral helpers

**Files:**
- Modify: `internal/app/view_wizard.go`
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: existing `wizardStep`, `wizardModel`, `newWizardModel`
- Produces:
  - `wizardModel.maxReached wizardStep`
  - `func (w *wizardModel) setFocus(step wizardStep)`
  - `func (w *wizardModel) moveFocus(delta int)`
  - `func (w *wizardModel) cycleOption(delta int)`
  - `func (w *wizardModel) confirmAdvance() bool` — returns false if cannot advance (e.g. empty Name)
  - `newWizardModel` initializes `maxReached = StepEngine`

- [ ] **Step 1: Write failing tests**

Append to `internal/app/view_wizard_test.go`:

```go
func TestWizardNewModelStartsAtEngine(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if w.step != StepEngine {
		t.Fatalf("step = %v, want StepEngine", w.step)
	}
	if w.maxReached != StepEngine {
		t.Fatalf("maxReached = %v, want StepEngine", w.maxReached)
	}
}

func TestWizardMoveFocusRespectsUnlock(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.maxReached = StepRuntime
	w.step = StepEngine

	w.moveFocus(1)
	if w.step != StepRuntime {
		t.Fatalf("after down: step = %v, want StepRuntime", w.step)
	}
	w.moveFocus(1) // locked Name
	if w.step != StepRuntime {
		t.Fatalf("should not enter locked Name, got %v", w.step)
	}
	w.moveFocus(-1)
	if w.step != StepEngine {
		t.Fatalf("after up: step = %v, want StepEngine", w.step)
	}
}

func TestWizardCycleOptionIsLateralOnly(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if w.selectedEngineIdx != 0 {
		t.Fatalf("engine idx = %d", w.selectedEngineIdx)
	}
	w.cycleOption(1)
	if w.selectedEngineIdx != 1 {
		t.Fatalf("after right: engine idx = %d, want 1", w.selectedEngineIdx)
	}
	w.cycleOption(1)
	if w.selectedEngineIdx != 1 {
		t.Fatalf("should clamp at last engine, got %d", w.selectedEngineIdx)
	}
	w.cycleOption(-1)
	if w.selectedEngineIdx != 0 {
		t.Fatalf("after left: engine idx = %d, want 0", w.selectedEngineIdx)
	}

	w.step = StepRuntime
	w.maxReached = StepRuntime
	w.cycleOption(1)
	if w.selectedRuntimeIdx != 1 {
		t.Fatalf("runtime idx = %d, want 1", w.selectedRuntimeIdx)
	}

	w.step = StepName
	w.maxReached = StepName
	before := w.selectedEngineIdx
	w.cycleOption(1)
	if w.selectedEngineIdx != before {
		t.Fatal("cycleOption on text row must not change engine")
	}
}

func TestWizardConfirmAdvanceUnlocksNext(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if !w.confirmAdvance() {
		t.Fatal("Engine confirm should succeed")
	}
	if w.step != StepRuntime || w.maxReached != StepRuntime {
		t.Fatalf("step=%v max=%v, want Runtime/Runtime", w.step, w.maxReached)
	}
	if !w.confirmAdvance() {
		t.Fatal("Runtime confirm should succeed")
	}
	if w.step != StepName || w.maxReached != StepName {
		t.Fatalf("step=%v max=%v, want Name/Name", w.step, w.maxReached)
	}
	if w.confirmAdvance() {
		t.Fatal("empty Name must not advance")
	}
	w.inputs[0].SetValue("shop")
	if !w.confirmAdvance() {
		t.Fatal("Name with value should advance")
	}
	if w.step != StepContainerName {
		t.Fatalf("step = %v, want ContainerName", w.step)
	}
}
```

Also fix `reviewModel` if it still calls `NewApp("/tmp")` with one arg — use `NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})` and set `maxReached = StepReview` so existing review tests keep showing all rows:

```go
import (
	"time"
	"local-database-manager/internal/config"
)

func reviewModel(width, height int) *AppModel {
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	// ... existing setup ...
	m.wizard.maxReached = StepReview
	return m
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/app/ -count=1 -run "TestWizardNewModel|TestWizardMoveFocus|TestWizardCycleOption|TestWizardConfirmAdvance"
```

Expected: compile fail (`maxReached` / methods undefined) or FAIL.

- [ ] **Step 3: Implement helpers**

In `wizardModel` add:

```go
maxReached wizardStep
```

In `newWizardModel`, after setting `step: StepEngine`:

```go
maxReached: StepEngine,
```

Add methods:

```go
func (w *wizardModel) setFocus(step wizardStep) {
	if step < StepEngine {
		step = StepEngine
	}
	if step > w.maxReached {
		step = w.maxReached
	}
	w.step = step
	w.blurAll()
	if step >= StepName && step <= StepMemoryLimit {
		w.focusInput(int(step) - int(StepName))
	}
}

func (w *wizardModel) moveFocus(delta int) {
	w.setFocus(w.step + wizardStep(delta))
}

func (w *wizardModel) cycleOption(delta int) {
	switch w.step {
	case StepEngine:
		n := w.selectedEngineIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(w.engines) {
			n = len(w.engines) - 1
		}
		w.selectedEngineIdx = n
	case StepRuntime:
		n := w.selectedRuntimeIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(w.runtimes) {
			n = len(w.runtimes) - 1
		}
		w.selectedRuntimeIdx = n
	}
}

func (w *wizardModel) confirmAdvance() bool {
	switch w.step {
	case StepEngine:
		w.maxReached = maxStep(w.maxReached, StepRuntime)
		w.setFocus(StepRuntime)
		return true
	case StepRuntime:
		w.maxReached = maxStep(w.maxReached, StepName)
		w.setFocus(StepName)
		return true
	case StepName:
		if strings.TrimSpace(w.inputs[0].Value()) == "" {
			return false
		}
		w.applyNameAutofill() // stub in Task 1: empty body; filled in Task 2/3
		w.maxReached = maxStep(w.maxReached, StepContainerName)
		w.setFocus(StepContainerName)
		return true
	case StepContainerName:
		w.maxReached = maxStep(w.maxReached, StepPort)
		w.setFocus(StepPort)
		return true
	case StepPort:
		w.maxReached = maxStep(w.maxReached, StepDatabase)
		w.setFocus(StepDatabase)
		return true
	case StepDatabase:
		w.maxReached = maxStep(w.maxReached, StepVolume)
		w.setFocus(StepVolume)
		return true
	case StepVolume:
		w.maxReached = maxStep(w.maxReached, StepPassword)
		w.setFocus(StepPassword)
		return true
	case StepPassword:
		w.maxReached = maxStep(w.maxReached, StepMemoryLimit)
		w.setFocus(StepMemoryLimit)
		return true
	case StepMemoryLimit:
		w.maxReached = maxStep(w.maxReached, StepReview)
		w.setFocus(StepReview)
		return true
	default:
		return false // Review handled by save path
	}
}

func maxStep(a, b wizardStep) wizardStep {
	if a > b {
		return a
	}
	return b
}

func (w *wizardModel) applyNameAutofill() {
	// Implemented in Task 3 (move existing Enter/Name autofill here).
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/app/ -count=1 -run "TestWizardNewModel|TestWizardMoveFocus|TestWizardCycleOption|TestWizardConfirmAdvance|TestWizardReview"
```

Expected: `ok` (review tests still pass with `maxReached` set).

---

### Task 2: Engine default recalculation

**Files:**
- Modify: `internal/app/view_wizard.go`
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: `wizardModel` inputs/engines
- Produces: `func (w *wizardModel) applyEngineDefaults(prevEngine, nextEngine string)`

- [ ] **Step 1: Write failing tests**

```go
func TestWizardApplyEngineDefaultsOnlyWhenStillDefault(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[3].SetValue("shop_db")
	w.inputs[4].SetValue("pgdata_shop")
	w.inputs[5].SetValue("postgres")
	w.inputs[6].SetValue("512M")
	w.selectedEngineIdx = 0

	w.applyEngineDefaults("postgres", "sqlserver")
	w.selectedEngineIdx = 1

	if w.inputs[1].Value() != "sql-shop" {
		t.Fatalf("container = %q, want sql-shop", w.inputs[1].Value())
	}
	if w.inputs[2].Value() != "1433" {
		t.Fatalf("port = %q, want 1433", w.inputs[2].Value())
	}
	if w.inputs[4].Value() != "sqlserver_shop" {
		t.Fatalf("volume = %q, want sqlserver_shop", w.inputs[4].Value())
	}
	if w.inputs[5].Value() != "SuperPassword123!" {
		t.Fatalf("password = %q", w.inputs[5].Value())
	}
	if w.inputs[6].Value() != "2G" {
		t.Fatalf("memory = %q, want 2G", w.inputs[6].Value())
	}
	if w.inputs[3].Value() != "shop_db" {
		t.Fatalf("database should stay %q", w.inputs[3].Value())
	}
}

func TestWizardApplyEngineDefaultsSkipsEditedFields(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("custom-box")
	w.inputs[2].SetValue("5555")
	w.inputs[4].SetValue("custom_vol")
	w.inputs[5].SetValue("my-secret")
	w.inputs[6].SetValue("1G")

	w.applyEngineDefaults("postgres", "sqlserver")

	if w.inputs[1].Value() != "custom-box" || w.inputs[2].Value() != "5555" ||
		w.inputs[4].Value() != "custom_vol" || w.inputs[5].Value() != "my-secret" ||
		w.inputs[6].Value() != "1G" {
		t.Fatalf("edited fields were overwritten: container=%q port=%q vol=%q pass=%q mem=%q",
			w.inputs[1].Value(), w.inputs[2].Value(), w.inputs[4].Value(),
			w.inputs[5].Value(), w.inputs[6].Value())
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/app/ -count=1 -run TestWizardApplyEngineDefaults
```

Expected: `undefined: applyEngineDefaults` or FAIL.

- [ ] **Step 3: Implement `applyEngineDefaults`**

```go
func engineDefaults(engine string) (prefix, volPrefix, port, pass, mem string) {
	if engine == "sqlserver" {
		return "sql", "sqlserver", "1433", "SuperPassword123!", "2G"
	}
	return "pg", "pgdata", "5432", "postgres", "512M"
}

func (w *wizardModel) applyEngineDefaults(prevEngine, nextEngine string) {
	if prevEngine == nextEngine {
		return
	}
	name := strings.TrimSpace(w.inputs[0].Value())
	prevP, prevV, prevPort, prevPass, prevMem := engineDefaults(prevEngine)
	nextP, nextV, nextPort, nextPass, nextMem := engineDefaults(nextEngine)

	if name != "" {
		oldCont := fmt.Sprintf("%s-%s", prevP, name)
		newCont := fmt.Sprintf("%s-%s", nextP, name)
		if w.inputs[1].Value() == oldCont {
			w.inputs[1].SetValue(newCont)
		}
		oldVol := fmt.Sprintf("%s_%s", prevV, name)
		newVol := fmt.Sprintf("%s_%s", nextV, name)
		if w.inputs[4].Value() == oldVol {
			w.inputs[4].SetValue(newVol)
		}
	}
	if w.inputs[2].Value() == prevPort {
		free := core.FindNextFreePort(mustAtoi(nextPort), w.instances)
		w.inputs[2].SetValue(strconv.Itoa(free))
	}
	if w.inputs[5].Value() == prevPass {
		w.inputs[5].SetValue(nextPass)
	}
	if w.inputs[6].Value() == prevMem {
		w.inputs[6].SetValue(nextMem)
	}
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
```

Note: for the unit test with port `5432` and no conflicting instances, `FindNextFreePort(1433, nil)` returns `1433` if free — acceptable. If the test environment has 1433 busy, assert with `strconv.Itoa(core.FindNextFreePort(1433, nil))` instead of hardcoding `"1433"`.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/app/ -count=1 -run TestWizardApplyEngineDefaults
```

Expected: `ok`

---

### Task 3: Wire `updateWizard` keys + Name autofill

**Files:**
- Modify: `internal/app/view_wizard.go` (`updateWizard`, `applyNameAutofill`)
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: helpers from Tasks 1–2
- Produces: key behavior matching the spec table

- [ ] **Step 1: Write failing key-wiring tests**

```go
func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestWizardKeysLateralAndVertical(t *testing.T) {
	t.Parallel()
	m := &AppModel{mode: ModeWizard, wizard: newWizardModel("/tmp", "/tmp/instances", nil)}

	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyRight})
	if m.wizard.selectedEngineIdx != 1 {
		t.Fatalf("right should select sqlserver, idx=%d", m.wizard.selectedEngineIdx)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyDown})
	if m.wizard.step != StepEngine {
		t.Fatalf("down must not leave Engine before unlock, got %v", m.wizard.step)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyEnter})
	if m.wizard.step != StepRuntime {
		t.Fatalf("enter -> Runtime, got %v", m.wizard.step)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyDown})
	if m.wizard.step != StepRuntime {
		t.Fatalf("down must not enter locked Name")
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyUp})
	if m.wizard.step != StepEngine {
		t.Fatalf("up -> Engine, got %v", m.wizard.step)
	}
}

func TestWizardBackAndEmptyBackspace(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.maxReached = StepName
	w.setFocus(StepName)
	m := &AppModel{mode: ModeWizard, wizard: w}

	_, _ = m.updateWizard(key("b"))
	if m.wizard.step != StepRuntime {
		t.Fatalf("b from Name -> Runtime, got %v", m.wizard.step)
	}

	m.wizard.setFocus(StepName)
	m.wizard.inputs[0].SetValue("")
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.wizard.step != StepRuntime {
		t.Fatalf("empty backspace -> Runtime, got %v", m.wizard.step)
	}
}

func TestWizardCycleEngineTriggersRecalc(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[5].SetValue("postgres")
	w.inputs[6].SetValue("512M")
	w.maxReached = StepEngine
	w.setFocus(StepEngine)
	m := &AppModel{mode: ModeWizard, wizard: w}

	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyRight})
	if m.wizard.inputs[1].Value() != "sql-shop" {
		t.Fatalf("container after engine change = %q", m.wizard.inputs[1].Value())
	}
}
```

For `KeyRight`/`KeyLeft`/`KeyUp`/`KeyDown`/`KeyEnter`/`KeyBackspace`, use `tea.KeyMsg{Type: ...}` (not runes). For `b`, use runes as above. Adjust if Bubble Tea’s `msg.String()` for arrows is `"right"` etc. — current code uses `msg.String()` switch cases `"up"`, `"down"`; add `"left"`, `"right"`, `"h"`, `"l"`, `"b"`.

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/app/ -count=1 -run "TestWizardKeys|TestWizardBack|TestWizardCycleEngine"
```

Expected: FAIL (old ↑/↓ still cycle options).

- [ ] **Step 3: Rewrite `updateWizard` key handling**

Replace the `enter` / `up` / `down` cases with:

```go
case "esc", "ctrl+c":
	m.mode = ModeMain
	m.statusMsg = "Instance creation cancelled"
	m.statusIsErr = false
	return m, nil

case "enter":
	if w.step == StepReview {
		// keep existing saveInstance success/error path verbatim
		if err := w.saveInstance(); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving instance: %v", err)
			m.statusIsErr = true
			m.mode = ModeMain
			return m, nil
		}
		m.mode = ModeMain
		m.statusMsg = fmt.Sprintf("Instance '%s' created successfully!", w.inputs[0].Value())
		m.statusIsErr = false
		return m, m.reloadInstancesCmd()
	}
	_ = w.confirmAdvance()
	return m, nil

case "up", "k":
	w.moveFocus(-1)
	return m, nil

case "down", "j":
	w.moveFocus(1)
	return m, nil

case "left", "h":
	prev := w.engines[w.selectedEngineIdx]
	w.cycleOption(-1)
	if w.step == StepEngine {
		w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
	}
	return m, nil

case "right", "l":
	prev := w.engines[w.selectedEngineIdx]
	w.cycleOption(1)
	if w.step == StepEngine {
		w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
	}
	return m, nil

case "b":
	if w.step > StepEngine {
		w.moveFocus(-1)
	}
	return m, nil

case "backspace":
	if w.step >= StepName && w.step <= StepMemoryLimit {
		idx := int(w.step) - int(StepName)
		if w.inputs[idx].Value() == "" {
			if w.step > StepEngine {
				w.moveFocus(-1)
			}
			return m, nil
		}
	}
	// fall through to textinput update below
```

Then keep the textinput update block for `StepName`–`StepMemoryLimit`, but **skip** consuming the message when it was already handled (for backspace-empty, return early as above). Structure:

```go
	// after key switch, if still on a text step and msg is tea.KeyMsg that wasn't fully handled:
	if w.step >= StepName && w.step <= StepMemoryLimit {
		idx := int(w.step) - int(StepName)
		var cmd tea.Cmd
		w.inputs[idx], cmd = w.inputs[idx].Update(msg)
		return m, cmd
	}
```

Move the existing Name autofill body into `applyNameAutofill()` (the code currently under `case StepName:` that sets container/port/db/volume/password/memory). Call it from `confirmAdvance` when leaving Name (Task 1 stub).

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/app/ -count=1 -run "TestWizard"
```

Expected: all wizard tests `ok`.

---

### Task 4: View, footer, surface

**Files:**
- Modify: `internal/app/view_wizard.go` (`viewWizard`, option-row rendering)
- Modify: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: `maxReached`, focused `step`
- Produces: chips when focused on Engine/Runtime; confirmed values otherwise; new footer string

- [ ] **Step 1: Write failing tests**

```go
func TestWizardFooterShowsNavHints(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if !strings.Contains(plain, "[↑↓] rows") || !strings.Contains(plain, "[←→] options") ||
		!strings.Contains(plain, "[b] back") {
		t.Fatalf("footer missing nav hints:\n%s", plain)
	}
}

func TestWizardShowsRuntimeAfterUnlock(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if strings.Contains(plain, "2. Runtime:") {
		t.Fatal("Runtime must stay hidden until unlocked")
	}
	m.wizard.maxReached = StepRuntime
	m.wizard.setFocus(StepRuntime)
	plain = stripANSI(m.viewWizard())
	if !strings.Contains(plain, "2. Runtime:") {
		t.Fatalf("Runtime missing after unlock:\n%s", plain)
	}
}

func TestWizardActiveEngineShowsChips(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if !strings.Contains(plain, "[Postgres]") {
		t.Fatalf("active engine should show chip brackets:\n%s", plain)
	}
}
```

Keep `TestWizardReviewFillsSurface` as the surface regression (already exists). If it starts failing after view changes, fix unpainted cells — do not raise `maxUnpainted`.

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/app/ -count=1 -run "TestWizardFooter|TestWizardShowsRuntime|TestWizardActiveEngine"
```

Expected: FAIL (old footer / reveal rules).

- [ ] **Step 3: Update `viewWizard`**

Reveal rule: show a numbered row when `w.maxReached >= that step` (not `w.step >=`).

For Engine:

```go
if w.maxReached >= StepEngine {
	if w.step == StepEngine {
		// chips row (existing StepEngine branch)
	} else {
		// confirmed ValueHighlightStyle row
	}
}
```

Same pattern for Runtime.

Text rows: show when `w.maxReached >= StepName` (etc.); active when `w.step == that step` (existing `wizardValueRow` already keys off `w.step`).

Footer:

```go
if w.step == StepReview {
	content = append(content, surfaceLine(inner, RunningStyle.Render(
		"All set! Press [Enter] to create the instance, [↑/b] to edit, or [Esc] to cancel.")))
} else {
	content = append(content, surfaceLine(inner, MutedStyle.Render(
		"[↑↓] rows  [←→] options  [Enter] next  [b] back  [Esc] cancel")))
}
```

Ensure every fragment still goes through `surfaceLine` / styles with `BgSurface`.

- [ ] **Step 4: Run full package tests — expect PASS**

```bash
go test ./internal/app/ -count=1
go test ./... -count=1
```

Expected: all `ok`.

- [ ] **Step 5: Manual check (optional)**

```bash
go run ./cmd/db-manager
```

Open `[n]`, verify ←/→ on Engine, Enter unlocks Runtime, ↑/↓ move rows, `b` goes back, no black holes in the modal.

---

## Self-review

| Spec item | Task |
|-----------|------|
| Lateral option keys | 1 + 3 |
| Vertical focus among unlocked | 1 + 3 |
| `b` / empty Backspace back | 3 |
| Progressive unlock + Name autofill | 1 + 3 |
| Engine recalc only defaults | 2 + 3 |
| Runtime does not recalc | 2 + 3 |
| Footer copy | 4 |
| `BgSurface` / no black patches | 4 (+ existing surface test) |
| English UI | all |

No TBD. `applyNameAutofill` stub in Task 1 is filled in Task 3 before wiring is considered done.
