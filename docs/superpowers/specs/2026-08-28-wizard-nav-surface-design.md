# Design: New Instance Wizard — Vertical Nav + Surface Fix

## Goal

Fix the new-instance wizard so option rows (Engine / Runtime) use **lateral** selection, the form is navigated **vertically** as steps unlock, users can **go back** to re-edit a row, and the modal no longer shows **black patches** (same Lip Gloss `BgSurface` rule as the main menu).

## Non-goals

- Redesigning other overlays (help, action menu, logs) beyond reusing existing helpers.
- Hot-reload of `config.yml` or new config knobs.
- Mouse support.
- Changing how `.env` files are written beyond existing `saveInstance` behavior.

## Interaction model

Rows in order:

1. Engine  
2. Runtime  
3. Name  
4. Container  
5. Port  
6. Database  
7. Volume  
8. Password  
9. Memory  
10. Review  

| Key | Behavior |
|-----|----------|
| `↑` / `↓` or `k` / `j` | Move focus among **already unlocked** rows |
| `←` / `→` or `h` / `l` | On Engine/Runtime only: change the highlighted option |
| `Enter` | Confirm active row and advance to the next unlocked row; on Review, create instance |
| `b` | Move focus to the previous unlocked row and reopen it for editing |
| `Backspace` | In an empty text input: same as `b`; otherwise delete character |
| `Esc` / `Ctrl+C` | Cancel and return to main (unchanged) |

Unlock rules:

- On open, only Engine is unlocked and focused.
- Confirming a row unlocks the next (same progressive reveal as today).
- `↑` cannot move above the first unlocked row or into rows not yet unlocked.
- After Name is confirmed, existing autofill for container/port/db/volume/password/memory still runs once (current behavior), then later rows are editable.

## Engine change recalculation

When Engine is reopened and the selection changes (Postgres ↔ SQL Server):

- Recalculate **only** fields that still hold the **previous engine’s default** values:
  - Port: `5432` ↔ `1433` (and free-port suggestion when still on the old default)
  - Password: `postgres` ↔ `SuperPassword123!`
  - Memory: `512M` ↔ `2G`
  - Container / volume prefixes (`pg-` / `pgdata_` ↔ `sql-` / `sqlserver_`) only if the current value still matches the old pattern derived from Name
- User-edited values are left alone.
- Changing Runtime alone does **not** trigger this recalculation.

## Visual / surface

Apply the main-menu Night Owl rule everywhere inside the wizard panel:

- Every label, value, chip, hint, and separator sets `Background(BgSurface)`.
- Rows rendered via `surfaceLine` / `joinWithSurfaceGaps` (or equivalents already in `helpers.go`).
- Text inputs keep `styleTextInput` (prompt, placeholder, cursor, text all on `BgSurface`).
- Selected Engine/Runtime chips use `SelectedItemStyle`; unselected use `NormalItemStyle` (both already surface-backed).
- Overlay whitespace around the modal stays `BgDark` (`renderOverlay`).

Row presentation:

- **Active option row:** horizontal chips; current choice highlighted.
- **Active text row:** focused input.
- **Confirmed row:** label + value (highlight styles), not an input.
- **Locked/pending:** not shown until unlocked.

Footer copy (English UI):

`[↑↓] rows  [←→] options  [Enter] next  [b] back  [Esc] cancel`  
On Review: keep create/cancel messaging; still allow `↑`/`b` to leave Review and re-edit.

## Architecture

Touches primarily:

- `internal/app/view_wizard.go` — key handling, unlock/focus model, engine recalc, view
- Possibly small helpers in `internal/app/helpers.go` if shared surface helpers are missing
- Tests in `internal/app/*_test.go` (wizard-focused)

Suggested state (implementation may name differently):

- Track `focusStep` (or keep `step` as focus) and `maxReached` (highest unlocked step).
- Option indices stay `selectedEngineIdx` / `selectedRuntimeIdx`.

No new packages or dependencies.

## Testing

- Lateral keys change Engine/Runtime selection; vertical keys do not (when on those rows).
- `↑`/`↓` move among unlocked rows only.
- `b` (and Backspace on empty input) moves to previous row.
- `Enter` on Review still creates; cancel still works.
- Engine switch recalculates only default-matching fields (table-driven).
- Rendered wizard output includes surface background SGR for inner content (same idea as `TestStyleTextInputSetsSurfaceBackground` / overlay tests).

## Copy / product rules

- All user-facing wizard strings remain English.
- Do not commit secrets; wizard still writes `instances/<name>.env` as today.
