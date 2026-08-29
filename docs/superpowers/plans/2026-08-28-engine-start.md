# Engine Start (Docker / Podman) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start an installed-but-offline Docker Desktop or Podman default machine from the TUI (Engines menu + offer to start-and-retry when instance Start fails with offline).

**Architecture:** Extend `core.InstanceRunner` with `StartEngine`. Podman runs `podman machine start`; Docker on Windows launches Docker Desktop then polls `docker info` until online or ~90s timeout. TUI adds `ModeEngineMenu` overlay and a confirm-offline-retry flow that reuses existing `confirmPurge`-style `y`/`n` status prompts. Health badges refresh via one-shot `checkEngineHealthCmd(false)` after start attempts.

**Tech Stack:** Go 1.22+, Bubble Tea, existing `internal/core` runner + `internal/app` overlays.

**Spec:** `docs/superpowers/specs/2026-08-28-engine-start-design.md`

## Global Constraints

- All user-facing strings remain English.
- Start only (no install, no `podman machine init`, no stop/restart engines).
- Reuse `ONLINE` / `OFFLINE` / `NOT_INSTALLED` from `CheckEngineHealth`.
- Docker Desktop launch is best-effort (Windows primary); poll `docker info` until timeout.
- Fixed ~90s start timeout; no new `config.yml` knob in v1.
- No concurrent engine starts (`engineStarting` flag).
- Do not redesign wizard, logs, or help beyond wiring Start failure → confirm.
- Key conflict: main-view `e` today opens the instance `.env` editor. **Resolution:** main `e` opens Engines menu; Edit `.env` remains available from the instance action menu (shortcut `e` there). Update footer shortcuts accordingly.
- TDD: failing test first; `go test` after each implementation step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/core/errors.go` | `ErrEngineStartFailed` |
| `internal/core/runner.go` | `StartEngine`, Docker Desktop helpers, poll |
| `internal/core/engine_start_windows.go` | Windows-specific Docker Desktop path + launch (`//go:build windows`) |
| `internal/core/engine_start_other.go` | Non-Windows Docker start stub (`//go:build !windows`) |
| `internal/core/runner_test.go` | StartEngine unit tests (health gates + podman args via injectable exec where practical) |
| `internal/app/tui.go` | `ModeEngineMenu`, `engineStarting`, msgs, Update routing |
| `internal/app/view_engine.go` | Engine menu update/view + start cmd |
| `internal/app/view_engine_test.go` | Menu + offline-retry tests |
| `internal/app/view_main.go` | `e` → Engines; offline confirm; footer `[e] Engines` |
| `internal/app/view_modal.go` | Unchanged Edit shortcut inside action menu |
| Stubs in `*_test.go` | Implement `StartEngine` on stub runners |

---

### Task 1: Errors + StartEngine health gates

**Files:**
- Modify: `internal/core/errors.go`
- Modify: `internal/core/runner.go`
- Modify: `internal/core/runner_test.go`
- Modify: any `InstanceRunner` stubs (`internal/app/tui_test.go`, etc.)

**Interfaces:**
- Consumes: `CheckEngineHealth`, `ErrEngineNotInstalled`
- Produces:
  - `var ErrEngineStartFailed = errors.New("failed to start container engine")`
  - `StartEngine(ctx context.Context, runtimeName string) error` on `InstanceRunner`
  - Gates: ONLINE → nil; NOT_INSTALLED → wrap `ErrEngineNotInstalled`; unknown runtime → error

- [ ] **Step 1: Write failing tests**

Append to `internal/core/runner_test.go`:

```go
func TestRunner_StartEngine_NotInstalled(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	err := r.StartEngine(context.Background(), "nonexistent_runtime_binary_xyz")
	if !errors.Is(err, ErrEngineNotInstalled) {
		t.Fatalf("got %v, want ErrEngineNotInstalled", err)
	}
}

func TestRunner_StartEngine_AlreadyOnlineIsNoop(t *testing.T) {
	t.Parallel()
	// Skip if neither docker nor podman is online on this machine.
	r := NewRunner("/tmp")
	ctx := context.Background()
	for _, rt := range []string{"docker", "podman"} {
		if r.CheckEngineHealth(ctx, rt) != EngineOnline {
			continue
		}
		if err := r.StartEngine(ctx, rt); err != nil {
			t.Fatalf("online %s StartEngine = %v, want nil", rt, err)
		}
		return
	}
	t.Skip("no online docker/podman to assert no-op")
}
```

- [ ] **Step 2: Run tests — expect FAIL** (method missing / interface break)

Run: `go test ./internal/core/ -run 'TestRunner_StartEngine_' -count=1 -v`

- [ ] **Step 3: Minimal implementation**

In `errors.go`:

```go
ErrEngineStartFailed = errors.New("failed to start container engine")
```

On `InstanceRunner`:

```go
StartEngine(ctx context.Context, runtimeName string) error
```

On `Runner`:

```go
func (r *Runner) StartEngine(ctx context.Context, runtimeName string) error {
	bin := runtimeName
	if bin == "" {
		bin = "docker"
	}
	health := r.CheckEngineHealth(ctx, bin)
	switch health {
	case EngineOnline:
		return nil
	case EngineNotInstalled:
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, bin)
	}
	switch bin {
	case "podman":
		return r.startPodmanMachine(ctx)
	case "docker":
		return r.startDockerEngine(ctx)
	default:
		return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, bin)
	}
}
```

Stub `startPodmanMachine` / `startDockerEngine` to return `ErrEngineStartFailed` for now (Task 2 fills them). Add empty `StartEngine` on all test stubs returning nil.

- [ ] **Step 4: Run tests — expect PASS** for the two gate tests (online skip OK)

- [ ] **Step 5: Commit** (only if user asked)

---

### Task 2: Podman machine start + Docker Desktop launch/poll

**Files:**
- Modify: `internal/core/runner.go`
- Create: `internal/core/engine_start_windows.go`
- Create: `internal/core/engine_start_other.go`
- Modify: `internal/core/runner_test.go`

**Interfaces:**
- Consumes: `StartEngine` gates from Task 1
- Produces:
  - `const engineStartTimeout = 90 * time.Second`
  - `func (r *Runner) startPodmanMachine(ctx context.Context) error`
  - `func (r *Runner) startDockerEngine(ctx context.Context) error`
  - `func (r *Runner) waitUntilOnline(ctx context.Context, runtimeName string) error`
  - Windows: `func findDockerDesktopExe() (string, error)` and launch via `exec.Command`

- [ ] **Step 1: Write failing tests**

```go
func TestWaitUntilOnline_TimesOut(t *testing.T) {
	t.Parallel()
	r := NewRunner("/tmp")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := r.waitUntilOnline(ctx, "nonexistent_runtime_binary_xyz")
	if err == nil {
		t.Fatal("expected timeout/error")
	}
	if !errors.Is(err, ErrEngineStartFailed) && !errors.Is(err, context.DeadlineExceeded) {
		// Accept either wrapped start-failed or deadline; prefer wrapping DeadlineExceeded in ErrEngineStartFailed
		t.Logf("got err=%v", err)
	}
}

func TestFindDockerDesktopExe_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	// Soft assert: function returns either a path ending in Docker Desktop.exe or an error
	_, err := findDockerDesktopExe()
	_ = err // existence depends on machine; just ensure it doesn't panic
}
```

For podman command shape, prefer a small injectable seam if the codebase already has one; otherwise test via documentation + integration skip, and unit-test `waitUntilOnline` against a fake by extracting:

```go
// package-level for tests
var lookPath = exec.LookPath
var commandContext = exec.CommandContext
```

Only introduce `lookPath`/`commandContext` vars if needed to stub; keep the change minimal.

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

```go
const engineStartTimeout = 90 * time.Second
const enginePollInterval = 2 * time.Second

func (r *Runner) startPodmanMachine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "podman", "machine", "start")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// "already running" → treat as success if now online
		if r.CheckEngineHealth(context.Background(), "podman") == EngineOnline {
			return nil
		}
		return fmt.Errorf("%w: podman machine start: %v (%s)", ErrEngineStartFailed, err, strings.TrimSpace(stderr.String()))
	}
	return r.waitUntilOnline(cmdCtx, "podman")
}

func (r *Runner) waitUntilOnline(ctx context.Context, runtimeName string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if r.CheckEngineHealth(ctx, runtimeName) == EngineOnline {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, runtimeName)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, runtimeName)
		case <-time.After(enginePollInterval):
		}
	}
}
```

`engine_start_windows.go`:

```go
//go:build windows

package core

func (r *Runner) startDockerEngine(ctx context.Context) error {
	exe, err := findDockerDesktopExe()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEngineStartFailed, err)
	}
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: launch Docker Desktop: %v", ErrEngineStartFailed, err)
	}
	_ = cmd.Process.Release()
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	return r.waitUntilOnline(cmdCtx, "docker")
}

func findDockerDesktopExe() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "Docker Desktop.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Docker", "Docker", "Docker Desktop.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Docker", "Docker Desktop.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("Docker Desktop.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("Docker Desktop.exe not found")
}
```

`engine_start_other.go`:

```go
//go:build !windows

package core

func (r *Runner) startDockerEngine(ctx context.Context) error {
	// Best-effort: wait in case the user started the daemon elsewhere; no Desktop launch.
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	if err := r.waitUntilOnline(cmdCtx, "docker"); err != nil {
		return fmt.Errorf("%w: docker is offline (auto-start supported on Windows via Docker Desktop)", ErrEngineStartFailed)
	}
	return nil
}
```

- [ ] **Step 4: `go test ./internal/core/ -count=1`** — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 3: Engines menu mode + footer shortcut

**Files:**
- Modify: `internal/app/tui.go`
- Create: `internal/app/view_engine.go`
- Create: `internal/app/view_engine_test.go`
- Modify: `internal/app/view_main.go` (key `e`, footer, remove main-level OpenInEditor on `e`)
- Update stubs: `StartEngine` already added in Task 1

**Interfaces:**
- Consumes: `m.runner.StartEngine`, `checkEngineHealthCmd(false)`
- Produces:
  - `ModeEngineMenu AppMode`
  - `engineMenuIndex int`, `engineStarting bool` on `AppModel`
  - `type engineStartedMsg struct { runtime string; err error; retryInst *core.DatabaseInstance }`
  - `func (m *AppModel) updateEngineMenu(msg tea.Msg) (tea.Model, tea.Cmd)`
  - `func (m *AppModel) viewEngineMenu() string`
  - `func (m *AppModel) startEngineCmd(runtime string, retryInst *core.DatabaseInstance) tea.Cmd`

- [ ] **Step 1: Write failing tests**

```go
func TestEngineMenuOpensOnE(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeMain
	m.dockerHealth = core.EngineOffline
	m.podmanHealth = core.EngineNotInstalled

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	am := updated.(*AppModel)
	if am.mode != ModeEngineMenu {
		t.Fatalf("mode=%v, want ModeEngineMenu", am.mode)
	}
}

func TestEngineMenuDisabledRows(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeEngineMenu
	m.dockerHealth = core.EngineOnline
	m.podmanHealth = core.EngineNotInstalled
	plain := stripANSI(m.viewEngineMenu())
	if !strings.Contains(plain, "online") {
		t.Fatalf("expected online label:\n%s", plain)
	}
	if !strings.Contains(plain, "not installed") {
		t.Fatalf("expected not installed label:\n%s", plain)
	}
}

func TestMainFooterShowsEnginesShortcut(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "[e]") || !strings.Contains(plain, "Engines") {
		t.Fatalf("footer missing Engines shortcut:\n%s", plain)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

`tui.go`: add mode + fields; route `ModeEngineMenu` in Update/View like action menu (overlay).

`view_main.go`:
- Change `case "e":` to open Engines menu (not editor). Cancel `confirmPurge` / `confirmEngineStart` when switching contexts as needed.
- In `mainShortcutEntries`, replace or add: `shortcut("[e]", "Engines")` — remove misleading edit hint from main footer if present; Edit stays in action menu only.

`view_engine.go` sketch:

```go
type engineMenuRow struct {
	runtime   string // "docker" | "podman"
	label     string
	actionable bool
}

func (m *AppModel) engineMenuRows() []engineMenuRow {
	return []engineMenuRow{
		engineRow("docker", m.dockerHealth),
		engineRow("podman", m.podmanHealth),
	}
}

func engineRow(runtime string, h core.EngineHealth) engineMenuRow {
	name := "Docker"
	if runtime == "podman" {
		name = "Podman"
	}
	switch h {
	case core.EngineOffline:
		return engineMenuRow{runtime: runtime, label: "Start " + name, actionable: true}
	case core.EngineOnline:
		return engineMenuRow{runtime: runtime, label: name + ": online", actionable: false}
	default:
		return engineMenuRow{runtime: runtime, label: name + ": not installed", actionable: false}
	}
}

func (m *AppModel) startEngineCmd(runtime string, retryInst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		err := m.runner.StartEngine(context.Background(), runtime)
		return engineStartedMsg{runtime: runtime, err: err, retryInst: retryInst}
	}
}
```

On Enter for actionable row: set `engineStarting=true`, status `Starting Docker Desktop...` / `Starting Podman machine...`, `mode=ModeMain`, return `startEngineCmd`.

Handle `engineStartedMsg` in `Update`: clear `engineStarting`; refresh health; set status success/error; if `retryInst != nil` and err==nil, call `toggleInstanceCmd(retryInst)` (start path).

- [ ] **Step 4: `go test ./internal/app/ -count=1`** — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 4: Offline instance Start → confirm → start engine → retry

**Files:**
- Modify: `internal/app/tui.go` (`confirmEngineStart`, pending instance pointer)
- Modify: `internal/app/view_main.go` (`toggleInstanceCmd` / err handling)
- Modify: `internal/app/view_engine_test.go` or `view_main` tests

**Interfaces:**
- Consumes: `errors.Is(err, core.ErrEngineOffline)`, `startEngineCmd`
- Produces:
  - `confirmEngineStart bool`
  - `pendingEngineRuntime string`
  - `pendingStartInst *core.DatabaseInstance`
  - On offline start failure: status prompt + `y`/`n` like purge

- [ ] **Step 1: Write failing test**

```go
func TestOfflineStartOffersEngineRetry(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 100, 30
	m.mode = ModeMain
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", Runtime: "podman", EngineType: "postgres", Status: core.StatusStopped,
	}}
	m.selectedIndex = 0
	m.runner = &stubRunner{ /* Start returns ErrEngineOffline; StartEngine records call */ }

	// Drive toggle / inject errMsg with ErrEngineOffline for selected inst
	updated, _ := m.Update(errMsg{err: fmt.Errorf("%w: podman", core.ErrEngineOffline)})
	am := updated.(*AppModel)
	// Prefer dedicated offlineStartMsg — see Step 3
	_ = am
}
```

Prefer a dedicated message from `toggleInstanceCmd`:

```go
type offlineStartMsg struct {
	inst *core.DatabaseInstance
	err  error
}
```

When Start fails with `ErrEngineOffline`, return `offlineStartMsg` instead of generic `errMsg`. Handler sets confirm prompt.

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement**

In `toggleInstanceCmd`, when starting (not stopping):

```go
if err != nil {
	if errors.Is(err, core.ErrEngineOffline) {
		return offlineStartMsg{inst: inst, err: err}
	}
	return errMsg{err}
}
```

Handler:

```go
case offlineStartMsg:
	m.confirmEngineStart = true
	m.pendingStartInst = msg.inst
	m.pendingEngineRuntime = msg.inst.Runtime
	name := "Docker"
	if msg.inst.Runtime == "podman" {
		name = "Podman"
	}
	m.statusMsg = fmt.Sprintf("%s is offline. Start engine and retry? Press 'y' to confirm, 'n' to cancel", name)
	m.statusIsErr = true
	return m, nil
```

In `updateMain` `y`/`n`:
- If `confirmEngineStart`: clear flag; on `y` set status Starting…, `engineStarting=true`, return `startEngineCmd(runtime, pendingInst)`; on `n` clear pending and status.

Guard: if `engineStarting`, ignore new Engines menu starts and new confirms.

- [ ] **Step 4: Full `go test ./internal/app/ ./internal/core/ -count=1`** — PASS

- [ ] **Step 5: Commit** (if asked)

---

### Task 5: Regression + manual smoke

- [ ] **Step 1:** `go test ./... -count=1` — PASS

- [ ] **Step 2: Manual smoke** (`go run ./cmd/db-manager`)

1. With Podman installed but machine stopped: badges show OFFLINE; `e` → Start Podman → badge ONLINE.
2. Docker Desktop quit: `e` → Start Docker → waits / comes ONLINE or clear timeout error.
3. Not installed runtime: row disabled.
4. Start instance while runtime offline → `y` starts engine and retries; `n` cancels.
5. Action menu still edits `.env` via `e` inside the menu; main `e` opens Engines.

- [ ] **Step 3:** Commit plan/spec only if user asks.

---

## Spec self-review (plan vs spec)

| Spec requirement | Task |
|------------------|------|
| Detect ONLINE/OFFLINE/NOT_INSTALLED | Existing + Task 3 refresh |
| `StartEngine` API | Task 1–2 |
| Podman `machine start` | Task 2 |
| Docker Desktop launch + poll | Task 2 |
| ~90s timeout | Task 2 `engineStartTimeout` |
| Engines menu `e` | Task 3 (key conflict resolved) |
| Disabled online / not installed rows | Task 3 |
| Status while starting; no double start | Task 3–4 `engineStarting` |
| Offline Start → confirm → start → retry | Task 4 |
| English copy | All tasks |
| No init/install | Task 2 non-goals |

No placeholders remain in task steps.
