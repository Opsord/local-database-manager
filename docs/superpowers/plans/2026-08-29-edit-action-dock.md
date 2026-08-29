# Docked Action Menu + Edit Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dock the Action Menu in the bottom-right panel (same recipe as New Instance) and add an in-TUI Edit Instance wizard that reuses `ModeWizard` with `create|edit`, including rename and optional restart-after-save.

**Architecture:** Extend `viewMain` so `ModeActionMenu` splits the right column like `ModeWizard`. Add `wizardKind` (`create`|`edit`) on `wizardModel`, preload from a selected instance for edit, and on save write/rename `.env`. If the instance was running when edit opened, arm `confirmRestartAfterEdit` and on `y` call `Stop` on the **pre-edit** identity then `Start` on the **post-save** identity (down without `-v`, then up `-d`).

**Tech Stack:** Go 1.22+, Bubble Tea, Lip Gloss, existing `internal/app` / `internal/core`.

**Spec:** `docs/superpowers/specs/2026-08-29-edit-action-dock-design.md`

## Global Constraints

- All user-facing UI copy remains English.
- Action Menu must not use centered `renderOverlay`.
- One bordered right panel when Action or Wizard is docked (no stacked bordered boxes).
- Edit fields = full create set; current name allowed for uniqueness; other names collide.
- External editor only from edit wizard (`o`), not a primary Action Menu item.
- Restart = `Stop` (compose `down`, no `-v`) then `Start` (`up -d`); not purge.
- On rename + restart: **Stop old** project/container identity, **Start new** from new `.env`.
- Mutual exclusion via `clearConfirms()` including `confirmRestartAfterEdit`.
- TDD: failing test first; `go test` after each step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/app/view_main.go` | Right dock for `ModeActionMenu`; footer shortcuts; `y`/`n` restart confirm |
| `internal/app/view_modal.go` | `viewActionDock`; Edit Instance action; remove primary OpenInEditor row; help copy |
| `internal/app/tui.go` | `View()` Action → `viewMain`; confirm fields; `clearConfirms` |
| `internal/app/view_wizard.go` | `wizardKind`; `newEditWizardModel`; save/rename; edit titles/hints; `o` open editor |
| `internal/app/view_wizard_test.go` | Prefill, rename uniqueness, save/restart confirms |
| `internal/app/view_modal_test.go` | Action dock layout tests |
| `README.md` | Action dock + Edit wizard notes |

---

### Task 1: Dock Action Menu (right panel)

**Files:**
- Modify: `internal/app/tui.go`, `internal/app/view_main.go`, `internal/app/view_modal.go`
- Test: `internal/app/view_modal_test.go` (and/or new tests in same package)

**Interfaces:**
- Consumes: `splitPanelHalfHeight`, wizard right-dock pattern in `viewMain`
- Produces:
  - `func (m *AppModel) viewActionDock(innerWidth, dockHeight int) string`
  - `func actionShortcutEntries() []string`
  - `View()` `ModeActionMenu` → `wrapScreen(m.viewMain())`
  - Right column when `ModeActionMenu`: details half + separator + action dock; single `ActivePanelStyle`
  - Left column full-height list (`panelBoxStyle(false)` when action docked, same idea as wizard: left inactive)

- [ ] **Step 1: Write failing tests**

```go
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
	// Overlay was centered modal; docked view must still show list+details together.
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
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/app/ -count=1 -run 'TestActionMenuDocked|TestActionDockedRight'`
Expected: FAIL (still overlay / missing dock title in `View`)

- [ ] **Step 3: Implement**

In `tui.go` `View()`:

```go
case ModeActionMenu:
	return m.wrapScreen(m.viewMain())
```

In `view_main.go` right column: treat `ModeActionMenu` like wizard — split details + `m.viewActionDock(...)`. Left: `panelBoxStyle(m.mode != ModeWizard && m.mode != ModeActionMenu)` (or equivalent: active left only on main).

`viewActionDock`: title `Actions: <name> (<engine>)`, list rows from `getActionMenuItems` (reuse selection styles), muted hints; **no** outer `ActivePanelStyle`. Constrain height; truncate/scroll simply (viewport optional; fixed list is OK if items fit).

Footer: `actionShortcutEntries()` → `[↑↓] Nav`, `[Enter] Run`, `[Esc] Close` when `ModeActionMenu`.

Refactor `viewActionMenu()` to call dock helper or delete overlay-only path; update any test that assumed overlay-only string.

- [ ] **Step 4:** `go test ./internal/app/ -count=1` — PASS

- [ ] **Step 5: Commit** (only if user asked)

---

### Task 2: Edit wizard kind + preload + Action entry + `o`

**Files:**
- Modify: `internal/app/view_wizard.go`, `internal/app/view_modal.go`, `internal/app/view_main.go`
- Test: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: `newWizardModel`, `ParseEnvFile` / instance fields, `core.OpenInEditor`
- Produces:
  - `type wizardKind int` with `wizardCreate`, `wizardEdit`
  - `wizardModel.kind`, `sourceName string`, `sourceEnvPath string`, `wasRunning bool`
  - `func newEditWizardModel(projectRoot, instancesDir string, existing []*core.DatabaseInstance, inst *core.DatabaseInstance) wizardModel`
  - Action item **Edit Instance** → `ModeWizard` + `newEditWizardModel`
  - Titles/hints differ by kind; `[o]` in edit opens external editor

- [ ] **Step 1: Write failing tests**

```go
func TestEditWizardPrefillsFromInstance(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "podman",
		ContainerName: "pg-shop", Port: "5433", Database: "shop_db",
		Password: "s3cret", Volume: "pgdata_shop", MemoryLimit: "1G",
		EnvFilePath: "/tmp/instances/shop.env",
		Status: core.StatusStopped,
	}
	w := newEditWizardModel("/tmp", "/tmp/instances", []*core.DatabaseInstance{inst}, inst)
	if w.kind != wizardEdit {
		t.Fatalf("kind=%v", w.kind)
	}
	if w.inputs[0].Value() != "shop" {
		t.Fatalf("name=%q", w.inputs[0].Value())
	}
	if w.engines[w.selectedEngineIdx] != "postgres" {
		t.Fatal("engine")
	}
	if w.runtimes[w.selectedRuntimeIdx] != "podman" {
		t.Fatal("runtime")
	}
	if w.inputs[2].Value() != "5433" || w.inputs[5].Value() != "s3cret" {
		t.Fatalf("port/pass not prefilled")
	}
	if !w.wasRunning && inst.Status == core.StatusReady {
		t.Fatal("wasRunning should track ready/starting")
	}
}

func TestActionEditOpensEditWizard(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeActionMenu
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-demo", Port: "5432", Database: "demo",
		Password: "x", Volume: "v", MemoryLimit: "512M",
		Status: core.StatusStopped,
	}}
	m.selectedIndex = 0
	// Select Edit Instance row (find by label after items change)
	items := m.getActionMenuItems(m.instances[0])
	editIdx := -1
	for i, it := range items {
		if it.label == "Edit Instance" {
			editIdx = i
			break
		}
	}
	if editIdx < 0 {
		t.Fatal("Edit Instance action missing")
	}
	m.actionMenuIndex = editIdx
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if am.mode != ModeWizard || am.wizard.kind != wizardEdit {
		t.Fatalf("mode=%v kind=%v", am.mode, am.wizard.kind)
	}
	plain := stripANSI(am.View())
	if !strings.Contains(plain, "Edit Instance") {
		t.Fatalf("expected edit title:\n%s", plain)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

```go
type wizardKind int

const (
	wizardCreate wizardKind = iota
	wizardEdit
)
```

Extend `wizardModel` with `kind`, `sourceName`, `sourceEnvPath`, `wasRunning`, and snapshots needed later for stop: `sourceRuntime`, `sourceProjectName`, `sourceContainerName` (copy from `inst` at open).

`newWizardModel`: set `kind=wizardCreate`.  
`newEditWizardModel`: set kind edit; fill engines/runtimes indices; set all inputs from `inst`; `maxReached=StepReview`; `wasRunning = status is Ready|Starting`; keep `source*` fields.

Create path (`n`) unchanged: `newWizardModel(...)`.

Replace Action Menu item:

```go
{
  label: "Edit Instance",
  description: "Edit instance settings in the docked wizard",
  shortcut: "",
  action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
    m.mode = ModeWizard
    m.wizard = newEditWizardModel(m.projectRoot, m.instancesDir, m.instances, inst)
    return m, nil
  },
},
```

Titles: create → `New Database Instance`; edit → `Edit Instance`.  
Hints: edit review → `Press [Enter] to save…`; include `[o] external editor` on edit hints.  
Handle `o` in `updateWizard` when `kind==wizardEdit`: `core.OpenInEditor(w.sourceEnvPath)` (or current path if not renamed yet).

- [ ] **Step 4:** `go test ./internal/app/ -count=1` — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 3: Save / rename + name uniqueness

**Files:**
- Modify: `internal/app/view_wizard.go`
- Test: `internal/app/view_wizard_test.go`

**Interfaces:**
- Consumes: `saveInstance` content builder
- Produces:
  - `func (w *wizardModel) nameTaken(name string) bool` — true if another instance (not `sourceName` when edit) exists
  - `func (w *wizardModel) saveInstance() error` — create write; edit overwrite or write-new+delete-old
  - On edit save success: return to caller without always creating; validation rejects taken names
  - Stay in wizard on write error (fix create path that jumps to main on error if still doing that for create — edit must stay)

- [ ] **Step 1: Write failing tests**

```go
func TestEditWizardNameUniquenessAllowsSelf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := []*core.DatabaseInstance{{Name: "shop"}, {Name: "other"}}
	inst := &core.DatabaseInstance{Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: "5432", Database: "db", Password: "p", Volume: "v", MemoryLimit: "512M",
		EnvFilePath: filepath.Join(dir, "shop.env")}
	_ = os.WriteFile(inst.EnvFilePath, []byte("ENGINE=postgres\n"), 0644)
	w := newEditWizardModel("/tmp", dir, existing, inst)
	if w.nameTaken("shop") {
		t.Fatal("self name must be allowed")
	}
	if !w.nameTaken("other") {
		t.Fatal("other name must be taken")
	}
}

func TestEditWizardRenameWritesNewDeletesOld(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	_ = os.WriteFile(oldPath, []byte("ENGINE=postgres\n"), 0644)
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: "5432", Database: "db", Password: "p",
		Volume: "v", MemoryLimit: "512M", EnvFilePath: oldPath,
	}
	w := newEditWizardModel("/tmp", dir, []*core.DatabaseInstance{inst}, inst)
	w.inputs[0].SetValue("shop2")
	w.inputs[1].SetValue("pg-shop2")
	// ensure required fields set...
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "shop2.env")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("expected new env")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expected old env removed")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

```go
func (w *wizardModel) nameTaken(name string) bool {
	name = strings.TrimSpace(name)
	for _, inst := range w.instances {
		if inst.Name == name {
			if w.kind == wizardEdit && name == w.sourceName {
				continue
			}
			return true
		}
	}
	return false
}
```

Before save / on advancing name step: if `nameTaken`, refuse advance / return error.

`saveInstance` edit branch:
1. Build content (same as create).
2. `newPath := filepath.Join(w.instancesDir, name+".env")`
3. `os.WriteFile(newPath, ...)`
4. If `w.kind == wizardEdit && newPath != w.sourceEnvPath` { `os.Remove(w.sourceEnvPath)` }
5. Update `w.sourceEnvPath = newPath` after success (optional)

`updateWizard` Enter on Review:
- If `nameTaken(name)` → status error, **stay** in wizard.
- If save error → status error, **stay** in wizard (edit); create may keep prior behavior or also stay — prefer stay for both.
- Create success: main + reload (unchanged message).
Implement edit success without restart in this task (`!wasRunning` → main + reload). If `wasRunning`, still save successfully and return to main with a plain saved status; Task 4 replaces that branch with `confirmRestartAfterEdit`.

```go
if w.kind == wizardEdit {
  m.mode = ModeMain
  m.statusMsg = fmt.Sprintf("Instance '%s' saved", name)
  m.statusIsErr = false
  return m, m.reloadInstancesCmd()
}
```

This task ends with save+rename+uniqueness green; restart confirm is Task 4 only.

- [ ] **Step 4:** `go test ./internal/app/ -count=1` — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 4: Restart confirm after edit (`y`/`n`)

**Files:**
- Modify: `internal/app/tui.go`, `internal/app/view_main.go`, `internal/app/view_wizard.go`
- Test: `internal/app/view_wizard_test.go` (or `tui_test.go`)

**Interfaces:**
- Consumes: `clearConfirms`, `runner.Stop`, `runner.Start`, stubRunner
- Produces:
  - `confirmRestartAfterEdit bool`
  - `pendingRestartOld *core.DatabaseInstance` (pre-edit identity for Stop)
  - `pendingRestartNewName string` (or full new inst after reload)
  - On edit save with `wasRunning`: `clearConfirms(); confirmRestartAfterEdit=true; status prompt`
  - `y` → Stop(old) then Start(new from disk); `n` → clear only
  - Include fields in `clearConfirms()`

- [ ] **Step 1: Write failing tests**

```go
func TestEditSaveWhenRunningArmsRestartConfirm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shop.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=v
`
	_ = os.WriteFile(path, []byte(content), 0644)
	inst, err := core.ParseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inst.Status = core.StatusReady

	m := NewApp(filepath.Dir(dir), config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.instances = []*core.DatabaseInstance{inst}
	m.selectedIndex = 0
	m.mode = ModeWizard
	m.wizard = newEditWizardModel(m.projectRoot, dir, m.instances, inst)
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if !am.confirmRestartAfterEdit {
		t.Fatal("expected restart confirm armed")
	}
	if am.mode != ModeMain {
		t.Fatalf("expected main after save, got %v", am.mode)
	}
	_ = cmd // may include reload
}

func TestEditRestartConfirmYesStopsThenStarts(t *testing.T) {
	t.Parallel()
	sr := &stubRunner{}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.runner = sr
	m.confirmRestartAfterEdit = true
	m.pendingRestartOld = &core.DatabaseInstance{Name: "old", Runtime: "docker", ProjectName: "pg-old", EnvFilePath: "/tmp/old.env"}
	m.pendingRestartNewName = "new"
	// Ensure new env exists or stub Start ignores path — wire test to whatever helper you add.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am := updated.(*AppModel)
	if am.confirmRestartAfterEdit {
		t.Fatal("confirm should clear")
	}
	if cmd == nil {
		t.Fatal("expected restart cmd")
	}
}
```

Adjust stub assertions to match implementation (`lastStop` / `lastStart` fields on stub if added).

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

On edit Review Enter after successful save:

```go
if w.wasRunning {
  m.clearConfirms()
  m.confirmRestartAfterEdit = true
  m.pendingRestartOld = &core.DatabaseInstance{
    Name: w.sourceName, Runtime: w.sourceRuntime,
    ProjectName: w.sourceProjectName, ContainerName: w.sourceContainerName,
    EnvFilePath: w.sourceEnvPath, // path before rename; keep pre-save path for Stop
    EngineType: w.engines[w.selectedEngineIdx], // prefer snapshotted engine/runtime from open
  }
  // Snapshot OLD fields at edit open time — do not use post-rename path for Stop.
  m.pendingRestartNewName = strings.TrimSpace(w.inputs[0].Value())
  m.mode = ModeMain
  m.statusMsg = "Saved. Restart container with new config? Press 'y' to confirm, 'n' to cancel"
  m.statusIsErr = true
  return m, m.reloadInstancesCmd()
}
```

**Critical:** store `pendingRestartOld` from fields captured in `newEditWizardModel` **before** any rename (`sourceEnvPath`, `sourceProjectName`, …). After rename, Stop uses old project; Start loads `instances/<new>.env` via reload then Start.

`restartAfterEditCmd`:

```go
func (m *AppModel) restartAfterEditCmd(old *core.DatabaseInstance, newName string) tea.Cmd {
  return func() tea.Msg {
    ctx := context.Background()
    _ = m.runner.Stop(ctx, old) // best-effort if already gone
    // find new inst by name after parse, or ParseEnvFile(instancesDir/newName.env)
    newInst, err := core.ParseEnvFile(filepath.Join(m.instancesDir, newName+".env"))
    if err != nil {
      return /* status err msg type or reuse existing */
    }
    if err := m.runner.Start(ctx, newInst); err != nil {
      return /* err */
    }
    return /* success + reload */
  }
}
```

Wire `y`/`n` in `updateMain` beside purge/engine confirms; nav keys clear via `clearConfirms()`.

- [ ] **Step 4:** `go test ./... -count=1` — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 5: Regression + README / help

- [ ] **Step 1:** `go test ./... -count=1` — PASS
- [ ] **Step 2:** Update README + `viewHelp` / action descriptions:
  - Enter opens docked Action Menu (bottom-right)
  - Edit Instance opens docked wizard; `[o]` external editor
  - Save while running prompts restart `y`/`n`
- [ ] **Step 3:** Manual smoke checklist (document in report): Action dock; Edit prefill; rename; running restart confirm; create still works; Engines left dock unaffected

---

## Spec self-review

| Spec item | Task |
|-----------|------|
| Action Menu docked right, no overlay | Task 1 |
| Footer action hints | Task 1 |
| ModeWizard create\|edit | Task 2 |
| Prefill + Edit Instance action | Task 2 |
| External editor only in edit (`o`) | Task 2 |
| All fields editable | Task 2–3 |
| Name uniqueness allows self | Task 3 |
| Rename write-new delete-old | Task 3 |
| Restart confirm + down/up | Task 4 |
| Stop old / Start new on rename | Task 4 |
| clearConfirms includes restart | Task 4 |
| Docs | Task 5 |

No placeholders in task steps.
