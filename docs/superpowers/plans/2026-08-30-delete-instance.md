# Delete Instance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Delete Instance (`D` + Action Menu) that requires an online engine, runs `down -v`, then removes the instance `.env`, while leaving Purge (`d`) unchanged.

**Architecture:** Mirror the existing `confirmPurge` / `purgeInstanceCmd` flow with a separate `confirmDelete` flag and `deleteInstanceCmd`. The cmd checks `runner.CheckEngineHealth` for the instance runtime, calls `DownVolumes`, and only then deletes the env file and triggers list reload.

**Tech Stack:** Go 1.22+, Bubble Tea, existing `internal/app` / `internal/core`.

**Spec:** `docs/superpowers/specs/2026-08-30-delete-instance-design.md`

## Global Constraints

- All user-facing UI copy remains English.
- Purge (`d`) semantics unchanged: `down -v` only; keep `.env`.
- Delete (`D`): engine must be ONLINE; all-or-nothing (no `.env` delete if purge fails or engine offline).
- Confirm copy exact: `Delete instance '<name>'? This purges container+volume and removes the .env. Press 'y' to confirm, 'n' to cancel`
- Action Menu label/description: `Delete Instance` / `Purge container+volume and remove the instance .env from the list` (after Purge row).
- Mutual exclusion via `clearConfirms()` including `confirmDelete`.
- TDD: failing test first; `go test` after each step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/app/tui.go` | `confirmDelete`; `clearConfirms`; esc/cancel clears delete |
| `internal/app/view_main.go` | `D` hotkey; `y`/`n` handlers; `deleteInstanceCmd`; footer `[D]` |
| `internal/app/view_modal.go` | Action Menu Delete row; help text |
| `internal/app/view_modal_test.go` / `view_main` tests or new `view_delete_test.go` | Arm confirm, delete success/offline/purge-fail, purge still keeps `.env` |
| `internal/app/tui_test.go` | Extend `stubRunner` with configurable `DownVolumes` error + call tracking |
| `README.md` | Document Delete vs Purge |

---

### Task 1: confirmDelete + Action Menu + D + delete command

**Files:**
- Modify: `internal/app/tui.go`, `internal/app/view_main.go`, `internal/app/view_modal.go`, `internal/app/tui_test.go`
- Test: `internal/app/view_delete_test.go` (new) and/or extend `view_modal_test.go`

**Interfaces:**
- Consumes: `InstanceRunner.CheckEngineHealth`, `DownVolumes`, existing `reloadInstancesCmd`, `errMsg`, `actionDoneMsg`
- Produces:
  - `AppModel.confirmDelete bool`
  - `clearConfirms()` also clears `confirmDelete`
  - `func (m *AppModel) deleteInstanceCmd(inst *core.DatabaseInstance) tea.Cmd`
  - Hotkey `D` and Action Menu item with `shortcut: "D"`
  - `y`/`n` branches for `confirmDelete` (same pattern as purge)
  - Esc paths that clear purge also clear delete (`confirmDelete` in the same `if` guards)

`deleteInstanceCmd` behavior:

```go
func (m *AppModel) deleteInstanceCmd(inst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		runtime := inst.Runtime
		if runtime == "" {
			runtime = "docker"
		}
		health := m.runner.CheckEngineHealth(ctx, runtime)
		if health == core.EngineNotInstalled {
			return errMsg{fmt.Errorf("%w: %s — start is unavailable; cannot delete", core.ErrEngineNotInstalled, runtime)}
		}
		if health == core.EngineOffline {
			return errMsg{fmt.Errorf("%w: %s is offline — start it from Engines (e) before deleting", core.ErrEngineOffline, runtime)}
		}
		if err := m.runner.DownVolumes(ctx, inst); err != nil {
			return errMsg{err}
		}
		if err := os.Remove(inst.EnvFilePath); err != nil && !os.IsNotExist(err) {
			return errMsg{err}
		}
		return actionDoneMsg{
			msg: fmt.Sprintf("Instance '%s' deleted (container, volume, and .env removed)", inst.Name),
		}
	}
}
```

After `actionDoneMsg` from delete, existing handler should already reload or refresh — verify `actionDoneMsg` path calls `reloadInstancesCmd` (if purge does not reload, batch reload after delete success in the `y` handler or extend `actionDoneMsg` handling for delete). Prefer: on `y` for delete, return `tea.Batch(m.deleteInstanceCmd(inst), …)` and in `actionDoneMsg` handler call `reloadInstancesCmd` if not already doing so for all actions. Inspect current `actionDoneMsg` handling and match purge UX; **must** reload list after successful delete so the row disappears.

Clamp `selectedIndex` after reload (existing reload logic may already clamp — verify).

Extend stub:

```go
type stubRunner struct {
	// ...existing...
	downVolumesErr error
	downVolumesCalls int
}

func (s *stubRunner) DownVolumes(context.Context, *core.DatabaseInstance) error {
	s.downVolumesCalls++
	return s.downVolumesErr
}
```

- [ ] **Step 1: Write failing tests**

```go
// internal/app/view_delete_test.go
package app

func TestActionMenuIncludesDeleteInstance(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	inst := &core.DatabaseInstance{Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped}
	items := m.getActionMenuItems(inst)
	found := false
	for _, it := range items {
		if it.label == "Delete Instance" && it.shortcut == "D" {
			found = true
			if !strings.Contains(it.description, ".env") {
				t.Fatalf("description=%q", it.description)
			}
		}
	}
	if !found {
		t.Fatal("Delete Instance missing after Purge")
	}
}

func TestKeyDArmsConfirmDelete(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.instances = []*core.DatabaseInstance{{Name: "demo", EngineType: "postgres", Runtime: "docker"}}
	m.selectedIndex = 0
	m.mode = ModeMain
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if !m.confirmDelete {
		t.Fatal("expected confirmDelete")
	}
	if !strings.Contains(m.statusMsg, "removes the .env") {
		t.Fatalf("status=%q", m.statusMsg)
	}
	if m.confirmPurge {
		t.Fatal("purge must not be armed")
	}
}

func TestDeleteInstanceRemovesEnvWhenOnline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	if err := os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stub := &stubRunner{docker: core.EngineOnline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	if _, ok := msg.(errMsg); ok {
		t.Fatalf("unexpected err: %#v", msg)
	}
	if stub.downVolumesCalls != 1 {
		t.Fatalf("DownVolumes calls=%d", stub.downVolumesCalls)
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatal("expected .env removed")
	}
}

func TestDeleteInstanceRefusesWhenOffline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOffline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	em, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if !errors.Is(em.err, core.ErrEngineOffline) {
		t.Fatalf("want ErrEngineOffline, got %v", em.err)
	}
	if stub.downVolumesCalls != 0 {
		t.Fatal("must not purge when offline")
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("env must remain")
	}
}

func TestDeleteInstanceKeepsEnvWhenPurgeFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOnline, downVolumesErr: errors.New("boom")}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("got %T, want errMsg", msg)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("env must remain after purge failure")
	}
}

func TestPurgeStillKeepsEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOnline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.purgeInstanceCmd(inst)()
	if _, ok := msg.(errMsg); ok {
		t.Fatalf("purge err: %#v", msg)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("purge must keep .env")
	}
}
```

Wire `NewApp` / tests so `m.runner = stub` works (field is already settable if unexported use existing test patterns — if `runner` is lowercase, tests in package `app` can set it).

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/app/ -run "Delete|PurgeStillKeeps|KeyDArms|ActionMenuIncludesDelete" -count=1 -v`  
Expected: FAIL (missing symbols / behavior)

- [ ] **Step 3: Implement**

1. `confirmDelete` + `clearConfirms`
2. Esc / cancel guards include `confirmDelete` wherever `confirmPurge` is listed
3. Action Menu item after Purge
4. `case "D":` arm confirm (main mode); Action Menu shortcut `"D"` returns to main with confirm armed
5. `y`/`n` for delete
6. `deleteInstanceCmd` as above; on success ensure list reload (e.g. `actionDoneMsg` handler already batches reload — if not, `y` handler: `return m, tea.Batch(m.deleteInstanceCmd(inst))` and make delete return a dedicated msg that triggers reload, **or** extend `actionDoneMsg` handling). Simplest reliable approach:

```go
type deleteDoneMsg struct {
	name string
	err  error
}

// deleteInstanceCmd returns deleteDoneMsg
// Update() on deleteDoneMsg: if err -> status error; else status success + reloadInstancesCmd()
```

Prefer `deleteDoneMsg` over overloading `actionDoneMsg` so reload is guaranteed.

7. Help strings in `view_modal.go` for `D` vs `d`
8. Footer shortcut `[D] Delete` next to `[d] Purge` in `view_main.go` shortcut list

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ ./internal/core/ -count=1`  
Expected: PASS (ignore known flaky engine timeout if environment-dependent; re-run once if only that flakes)

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add internal/app/
git commit -m "feat(tui): delete instance with purge and .env removal"
```

---

### Task 2: README + regression sweep

**Files:**
- Modify: `README.md`
- Touch help/footer if anything missed in Task 1

**Interfaces:**
- Produces: README feature + key table rows distinguishing Purge vs Delete

- [ ] **Step 1: Update README**

Under Features / keys:

```markdown
- 🗑️ **Delete Instance (`D`):** Purge container+volume and remove the instance `.env` from the list (engine must be online).
```

And in the keymap table:

| `d` | Purge | Destroy container and volume only (`y/N`); keeps `.env` |
| `D` | Delete Instance | Purge + remove `.env` definition (`y/N`); requires online engine |

- [ ] **Step 2: Full suite**

Run: `go test ./... -count=1`  
Expected: PASS

- [ ] **Step 3: Commit** (only if user asked)

```bash
git add README.md
git commit -m "docs: document Delete Instance vs Purge"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| Keep Purge (`d`) | 1 (`TestPurgeStillKeepsEnv`) |
| Delete = online → down -v → remove `.env` | 1 |
| Offline / not installed refuse | 1 |
| Purge fail keeps `.env` | 1 |
| Action Menu + `D` | 1 |
| Confirm copy | 1 |
| `clearConfirms` | 1 |
| README / help | 1–2 |

## Self-review notes

- No TBD placeholders.
- Prefer `deleteDoneMsg` so list reload is explicit.
- Action Menu match is case-sensitive (`"D"` ≠ `"d"`) — matches current `actionMenuItemMatchesKey`.
- Do not delete orphan volumes from other majors (non-goal).
