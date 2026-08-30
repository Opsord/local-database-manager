# Design: Delete Instance (purge + remove definition)

**Date:** 2026-08-30  
**Status:** Approved for implementation planning  
**Scope:** Add Delete Instance that purges container/volume and removes the instance `.env` from the list, while keeping existing Purge behavior.

## Goal

Users can permanently remove a defined DB instance from the list. Deletion always destroys related container and volume data first, then deletes the instance definition file.

## Non-goals

- Migrating or archiving instance configs
- Deleting orphan volumes from prior majors / other projects
- Soft-delete / recycle bin
- Changing Purge (`d`) semantics beyond coexistence with Delete

## Decisions

| Topic | Choice |
|-------|--------|
| Relationship to Purge | **Keep both** — Purge = data only; Delete = purge + remove `.env` |
| Offline engine | **Require online** — refuse delete if runtime offline / not installed |
| Failure on purge | **All-or-nothing** — if `down -v` fails, do not delete `.env` |
| Entry points | Action Menu item + hotkey `D` (uppercase; distinct from `d` Purge) |
| Confirm | Single `y`/`n` confirm with strong copy |

## Behavior

### Purge (`d`) — unchanged

1. Confirm: purge container and volume for `'name'`
2. Run compose `down -v` for the instance project
3. Keep `instances/<name>.env`
4. Reload / refresh status as today

### Delete Instance (`D` / Action Menu)

1. User selects instance and presses `D`, or chooses **Delete Instance** in the Action Menu
2. Arm `confirmDelete` (clears other confirms via `clearConfirms()`)
3. Status (English):  
   `Delete instance '<name>'? This purges container+volume and removes the .env. Press 'y' to confirm, 'n' to cancel`
4. On `n` / Esc pattern matching other confirms: cancel, clear flag
5. On `y`:
   - Resolve instance runtime health
   - If `NOT_INSTALLED` or `OFFLINE`: error status, **no** file delete (e.g. tell user to start the engine from Engines panel)
   - If `ONLINE`: `DownVolumes` (same as purge)
   - If DownVolumes errors: surface error, **keep** `.env`
   - If success: `os.Remove` the instance env file; reload instance list; clear selection safely (clamp index); success status

### Action Menu copy

- Label: `Delete Instance`
- Description: `Purge container+volume and remove the instance .env from the list`

Place the row after Purge Volume & Reset.

## UX / docs

- Main footer shortcuts: show `[D]` Delete (alongside `[d]` Purge) when space allows
- Help modal: document both actions and the difference
- README: short note that Delete removes definition + data; Purge only data

## Testing (acceptance)

- Action Menu includes Delete Instance; `D` arms `confirmDelete`
- `y` with online stub runner: DownVolumes called, `.env` removed, list reload
- Offline / not installed: no file remove
- DownVolumes error: `.env` still present
- `d` still only purges without deleting `.env`
- Mutual exclusion with purge / engine / restart confirms

## Implementation sketch

1. `confirmDelete` on `AppModel` + `clearConfirms`
2. Action Menu row + `D` key in main/modal handlers
3. Delete cmd: health check → DownVolumes → remove env → reload
4. Tests + README/help
