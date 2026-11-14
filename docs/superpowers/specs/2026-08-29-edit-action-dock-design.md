# Design: Docked Action Menu + In-TUI Edit Wizard

**Date:** 2026-08-29  
**Status:** Approved for planning  
**Related:** Wizard dock (`2026-08-28-wizard-docked-panel-design.md`), Engines dock (`2026-08-28-engines-dock-stop-design.md`)

## Goal

When managing an instance, the **Action Menu** and the **Edit** flow use the same bottom-right dock pattern as **New Instance** (`n`), instead of a centered overlay / external editor as the primary path.

## Decisions (locked)

| Topic | Choice |
|-------|--------|
| Scope | **C** — dock Action Menu **and** in-TUI edit wizard |
| Editable fields | **A** — all create fields (engine, runtime, name, container, port, DB, volume, password, memory) |
| Running instance on save | **B** — save `.env`, then `y`/`n` confirm to restart (down + up) |
| External editor | **C** — only from edit wizard (hint/shortcut), not a primary Action Menu item |
| Architecture | **A** — single `ModeWizard` with `create` \| `edit` kind; Action Menu reuses right-column dock |

## Layout

### Action Menu (`Enter` → `ModeActionMenu`)

- Stop using centered `renderOverlay` for the action list.
- Right column: **one** bordered panel (`ActivePanelStyle` when focused), split with `splitPanelHalfHeight(contentHeight - 1)` + `panelSeparator`:
  - Top: existing Details & Config
  - Bottom: action items (label + description + shortcut), scrollable if needed
- Left column: full-height instance list (same as wizard-open behavior).
- Footer: action-menu shortcuts while docked.
- `Esc` / cancel → `ModeMain`.

### Create and Edit (`ModeWizard`)

- Both use existing `viewWizardDock` / right-column split (unchanged layout recipe).
- `wizard.kind`: `create` | `edit`.
  - **create** (`n`): title “New Database Instance”; empty/defaults as today.
  - **edit** (from Action Menu): title “Edit Instance”; fields preloaded from the selected instance’s `.env`.
- Same vertical navigation, lateral option cycling, review step, and scroll behavior as create.

## Flows

### Open Edit

1. User opens Action Menu → chooses **Edit Instance** (replaces “Edit .env Configuration” that only called `OpenInEditor`).
2. App leaves `ModeActionMenu`, enters `ModeWizard` with `kind=edit` and a reference to the source instance (path / name).
3. Wizard loads current values into inputs and option indices.

### External editor (secondary)

- Inside **edit** wizard only: shortcut/hint (e.g. `[o] Open in external editor`) calls `OpenInEditor(envPath)`.
- Not listed as a primary Action Menu row.

### Save (Enter on Review)

1. Validate (same rules as create). For **name uniqueness**, the current instance’s name is allowed; any other existing name is rejected.
2. Write `.env`:
   - If name unchanged: overwrite file.
   - If name changed: write `instances/<new>.env`, remove `instances/<old>.env` (atomic-enough: write new first, then delete old; on failure leave status error and stay in wizard).
3. If instance was **not** `READY` / `STARTING` when edit started: success status, `ModeMain`, reload instances.
4. If instance **was** running/starting when edit started: after successful write, arm confirm via `clearConfirms()` then set a dedicated flag (e.g. `confirmRestartAfterEdit`):
   - Status: `Saved. Restart container with new config? Press 'y' to confirm, 'n' to cancel`
   - `y` → compose **down** (no `-v`) then **up -d** using the **post-save** instance identity (new name/project if renamed); clear confirm; reload. Not a purge.
   - `n` → clear confirm; stay on main with saved `.env`; container keeps old runtime config until manual restart.

### Cancel

- `Esc` in wizard or action dock discards unsaved wizard edits (no write).

## Errors

| Case | Behavior |
|------|----------|
| Write / rename failure | Status error; remain in edit wizard |
| Restart after `y` fails | `.env` already saved; status error; return to main + reload |
| Confirm clashes | `clearConfirms()` clears purge / engine-start / engine-stop / restart-after-edit together |

## Testing (minimum)

- Action Menu docked: main header + instance list + actions in lower-right; not a centered overlay.
- Edit opens wizard prefilled; create path unchanged.
- Rename: current name allowed; colliding other name rejected.
- Running + save → confirm armed; `y` schedules restart cmd; `n` does not.
- Right-column height still matches left (same dock recipe as wizard).

## Out of scope

- Visual diff of changes before save
- Multi-instance edit
- External editor outside the edit wizard
- Changing Engines dock behavior

## File touch map (indicative)

| Area | Likely files |
|------|----------------|
| Action dock | `view_modal.go`, `view_main.go`, `tui.go` |
| Wizard kind create/edit | `view_wizard.go`, `view_wizard_test.go` |
| Confirms / restart | `tui.go`, `view_main.go` |
| Docs | `README.md`, help strings in `view_modal.go` |
