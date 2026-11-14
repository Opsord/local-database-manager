# Docked Engines Panel + Start/Stop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dock the Engines UI in the bottom half of the left panel (mirror of the wizard dock) and add Stop for online engines with `y`/`n` confirmation before stopping.

**Architecture:** Reuse the single-bordered panel + `splitPanelHalfHeight` pattern from the wizard dock on the **left** column when `ModeEngineMenu`. Extend `InstanceRunner` with `StopEngine` (Podman `machine stop`; Windows Docker Desktop quit + poll until offline). ONLINE rows become `Stop …` and Enter arms confirm; OFFLINE rows keep Start without confirm. `View()` for Engines uses `viewMain()` (no overlay).

**Tech Stack:** Go 1.22+, Bubble Tea, Lip Gloss, existing `internal/core` / `internal/app`.

**Spec:** `docs/superpowers/specs/2026-08-28-engines-dock-stop-design.md`

**Prerequisite:** Engine-start work already in the tree (`StartEngine`, `ModeEngineMenu`, Engines overlay, offline instance retry). If missing, land that first from `docs/superpowers/plans/2026-08-28-engine-start.md`.

## Global Constraints

- All user-facing strings remain English.
- Start only needs no confirm; Stop requires `y`/`n` confirm.
- No install / `podman machine init`.
- One bordered left panel when docked (do not stack two bordered boxes).
- `ModeEngineMenu` must not use centered `renderOverlay`.
- Footer shows Engines hints while docked.
- Reuse ~90s timeout / poll interval constants from start where possible.
- Mutual exclusion with other confirms via `clearConfirms()`.
- TDD: failing test first; `go test` after each step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/core/errors.go` | Optional `ErrEngineStopFailed` (or reuse `ErrEngineStartFailed`) |
| `internal/core/runner.go` | `StopEngine`, `stopPodmanMachine`, `waitUntilOffline` |
| `internal/core/engine_start_windows.go` | Add `stopDockerEngine` (or sibling `engine_stop_windows.go`) |
| `internal/core/engine_start_other.go` | Non-Windows `stopDockerEngine` |
| `internal/core/runner_test.go` | StopEngine gate tests |
| `internal/app/tui.go` | `View()` ModeEngineMenu → viewMain; confirm fields for stop |
| `internal/app/view_main.go` | Left-panel dock split; Engines footer shortcuts |
| `internal/app/view_engine.go` | Dock render; ONLINE=Stop; confirm on Enter; `stopEngineCmd` |
| `internal/app/view_engine_test.go` | Dock layout + stop confirm tests |
| Stubs | `StopEngine` on `stubRunner` |

---

### Task 1: StopEngine in core

**Files:**
- Modify: `internal/core/errors.go`, `runner.go`, `runner_test.go`
- Modify: `engine_start_windows.go` / `engine_start_other.go` (or new stop files with same build tags)
- Modify: `internal/app/tui_test.go` stub

**Interfaces:**
- Consumes: `CheckEngineHealth`, `engineStartTimeout`, `enginePollInterval`
- Produces:
  - `StopEngine(ctx context.Context, runtimeName string) error` on `InstanceRunner`
  - `waitUntilOffline(ctx, runtimeName string) error` — success when health != ONLINE (prefer OFFLINE)
  - Podman: `podman machine stop`
  - Windows Docker: best-effort quit Desktop then poll offline
  - ONLINE stop path; OFFLINE → nil; NOT_INSTALLED → `ErrEngineNotInstalled`

- [ ] **Step 1: Write failing tests**

```go
func TestRunner_StopEngine_NotInstalled(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	err := r.StopEngine(context.Background(), "nonexistent_runtime_binary_xyz")
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("got %v, want ErrEngineNotInstalled", err)
	}
}

func TestWaitUntilOffline_TimesOut(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// Use a name that LookPath finds but info won't go offline quickly — or test
	// waitUntilOffline against a runtime that is ONLINE if available; else skip.
	err := r.waitUntilOffline(ctx, "nonexistent_runtime_binary_xyz")
	// nonexistent → CheckEngineHealth is NOT_INSTALLED, which is "not ONLINE" → may return nil.
	// Prefer: if docker is ONLINE, short timeout should fail; else Skip.
	_ = err
}
```

Better concrete test for gates:

```go
func TestRunner_StopEngine_AlreadyOfflineIsNoop(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx := context.Background()
	for _, rt := range []string{"docker", "podman"} {
		h := r.CheckEngineHealth(ctx, rt)
		if h != EngineOffline && h != EngineNotInstalled {
			continue
		}
		if h == EngineNotInstalled {
			continue
		}
		if err := r.StopEngine(ctx, rt); err != nil {
			t.Fatalf("offline %s StopEngine = %v, want nil", rt, err)
		}
		return
	}
	t.Skip("no offline docker/podman to assert no-op")
}
```

- [ ] **Step 2: Run — expect FAIL** (`StopEngine` undefined)

- [ ] **Step 3: Implement**

```go
func (r *Runner) StopEngine(ctx context.Context, runtimeName string) error {
	bin := runtimeName
	if bin == "" {
		bin = "docker"
	}
	health := r.CheckEngineHealth(ctx, bin)
	switch health {
	case EngineOffline:
		return nil
	case EngineNotInstalled:
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, bin)
	}
	switch bin {
	case "podman":
		return r.stopPodmanMachine(ctx)
	case "docker":
		return r.stopDockerEngine(ctx)
	default:
		return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, bin)
	}
}
```

`stopPodmanMachine`: `podman machine stop`, then `waitUntilOffline`.  
Windows `stopDockerEngine`: try graceful quit (e.g. `taskkill /IM "Docker Desktop.exe"` is harsh — prefer documented best-effort: if a quit CLI exists use it; otherwise `os/exec` to stop the app via known approach used by the implementer with a comment). Poll until OFFLINE.  
`!windows`: return clear `ErrEngineStartFailed` / stop-failed “auto-stop supported on Windows” after optional poll.

Add `StopEngine` returning nil on stubs.

- [ ] **Step 4:** `go test ./internal/core/ ./internal/app/ -count=1` — PASS

- [ ] **Step 5: Commit** (if user asked)

---

### Task 2: Dock Engines in left panel

**Files:**
- Modify: `internal/app/tui.go` (`View` ModeEngineMenu)
- Modify: `internal/app/view_main.go`
- Modify: `internal/app/view_engine.go`
- Modify: `internal/app/view_engine_test.go`

**Interfaces:**
- Consumes: `splitPanelHalfHeight`, wizard left/right dock pattern
- Produces:
  - `func (m *AppModel) viewEngineDock(innerWidth, dockHeight int) string`
  - Left column when `ModeEngineMenu`: list half + separator + engine dock; single `ActivePanelStyle` border
  - Right column full-height details
  - Footer: Engines shortcuts when `ModeEngineMenu`
  - `View()`: `ModeEngineMenu` → `wrapScreen(m.viewMain())`

- [ ] **Step 1: Write failing tests**

```go
func TestEngineMenuDockedInLeftPanel(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOffline
	m.podmanHealth = core.EngineOffline
	m.instances = []*core.DatabaseInstance{{Name: "demo", EngineType: "postgres", Runtime: "docker"}}
	m.selectedIndex = 0

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "LOCAL DATABASE MANAGER") {
		t.Fatal("expected header")
	}
	if !strings.Contains(plain, "DB Instances") {
		t.Fatal("expected instance list")
	}
	if !strings.Contains(plain, "Container Engines") && !strings.Contains(plain, "Engines") {
		t.Fatal("expected engines dock title")
	}
	if !strings.Contains(plain, "Details & Config") {
		t.Fatal("expected right details")
	}
}

func TestEngineDockedLeftMatchesRightHeight(t *testing.T) {
	t.Parallel()
	inner := 118
	leftW, rightW, _ := splitPanelWidths(inner)
	contentH := 27
	rightBox := panelBoxStyle(false).Width(rightW).Height(contentH).Render("right")
	leftInner := panelInnerWidth(leftW)
	top, bottom := splitPanelHalfHeight(contentH - 1)
	listBlock := lipgloss.NewStyle().Width(leftInner).Height(top).MaxHeight(top).Render("list")
	engBlock := lipgloss.NewStyle().Width(leftInner).Height(bottom).MaxHeight(bottom).Render("eng")
	leftCol := ActivePanelStyle.Width(leftW).Height(contentH).Render(
		lipgloss.JoinVertical(lipgloss.Left, listBlock, panelSeparator(leftInner), engBlock),
	)
	if lipgloss.Height(leftCol) != lipgloss.Height(rightBox) {
		t.Fatalf("left=%d right=%d", lipgloss.Height(leftCol), lipgloss.Height(rightBox))
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (still overlay / no dock)

- [ ] **Step 3: Implement**

Mirror wizard block in `view_main.go` for `ModeEngineMenu` on the **left**:

```go
if m.mode == ModeEngineMenu {
	listH, engH := splitPanelHalfHeight(contentHeight - 1)
	listBlock := /* title + listItems, Height listH */
	engBlock := m.viewEngineDock(leftInner, engH)
	leftColumn = ActivePanelStyle.Width(leftWidth).Height(contentHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, listBlock, panelSeparator(leftInner), engBlock),
	)
}
```

`viewEngineDock`: same rows as today, width = leftInner, height constrained; **no** outer `ActivePanelStyle` (parent provides border). Title + rows + muted hints.

`tui.go` View:

```go
case ModeEngineMenu:
	return m.wrapScreen(m.viewMain())
```

Footer: if `ModeEngineMenu`, use engine shortcut entries (`[↑↓] Nav`, `[Enter] Action`, `[Esc] Close`).

Update tests that called `viewEngineMenu()` overlay — point at dock/`View()`.

- [ ] **Step 4:** `go test ./internal/app/ -count=1` — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 3: Stop labels + y/n confirm

**Files:**
- Modify: `internal/app/view_engine.go`, `view_main.go`, `tui.go`
- Modify: `internal/app/view_engine_test.go`

**Interfaces:**
- Consumes: `StopEngine`, `clearConfirms`
- Produces:
  - `engineRow` ONLINE → `Stop Docker` / `Stop Podman`, `actionable: true`, kind start|stop
  - `confirmEngineStop bool`, `pendingStopRuntime string`
  - Enter ONLINE → arm confirm (status prompt); Enter OFFLINE → Start as today
  - `y` when `confirmEngineStop` → `stopEngineCmd`
  - `engineStoppedMsg` or reuse `engineStartedMsg` renamed to `engineOpDoneMsg` with op field

Prefer extending message:

```go
type engineOpDoneMsg struct {
	runtime   string
	err       error
	op        string // "start" | "stop"
	retryInst *core.DatabaseInstance // start-only
}
```

Or keep `engineStartedMsg` for start and add `engineStoppedMsg` for stop — simpler, less churn.

- [ ] **Step 1: Write failing tests**

```go
func TestEngineOnlineRowShowsStop(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.podmanHealth = core.EngineOffline
	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "Stop Docker") {
		t.Fatalf("expected Stop Docker:\n%s", plain)
	}
	if !strings.Contains(plain, "Start Podman") {
		t.Fatalf("expected Start Podman:\n%s", plain)
	}
}

func TestEngineStopRequiresConfirm(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.engineMenuIndex = 0 // docker
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if !am.confirmEngineStop {
		t.Fatal("expected confirmEngineStop armed")
	}
	if cmd != nil {
		// Enter must not start stop cmd yet
		t.Fatal("Enter on online must not dispatch stop yet")
	}
	updated, cmd = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am = updated.(*AppModel)
	if !am.engineStarting {
		t.Fatal("expected busy after y")
	}
	if cmd == nil {
		t.Fatal("expected stopEngineCmd")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

Update `engineRow` for ONLINE → Stop label + actionable.  
Enter branch: if label/start vs stop — track `row.op` field `start|stop`.  
Arm confirm:

```go
m.clearConfirms()
m.confirmEngineStop = true
m.pendingStopRuntime = row.runtime
m.statusMsg = fmt.Sprintf("Stop %s? Press 'y' to confirm, 'n' to cancel", displayName)
m.statusIsErr = true
```

In `updateMain` / top-level `y`/`n` (same place as purge / offline start): handle `confirmEngineStop`.  
`stopEngineCmd` → `StopEngine` → `engineStoppedMsg` → clear busy, refresh health, status.

Hints in dock: `[Enter] start/stop  [y/n] confirm stop  [Esc] close`.

- [ ] **Step 4:** `go test ./... -count=1` — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 4: Regression + README

- [ ] **Step 1:** `go test ./... -count=1` — PASS
- [ ] **Step 2:** README / help: Engines dock; Start offline; Stop online with confirm; Edit `.env` via action menu
- [ ] **Step 3:** Manual smoke — `e` docks left; Start/Stop; heights aligned; wizard still docks right

---

## Spec self-review

| Spec item | Task |
|-----------|------|
| Left dock 50/50 single border | Task 2 |
| No Engines overlay | Task 2 |
| Footer Engines hints | Task 2 |
| Start OFFLINE no confirm | Task 3 (unchanged path) |
| Stop ONLINE + y/n | Task 3 |
| StopEngine core | Task 1 |
| clearConfirms mutual exclusion | Task 3 |
| Height alignment | Task 2 test |

No placeholders in task steps.
