# Design: Docked Engines Panel + Start/Stop

## Goal

Show the Engines UI in the **bottom half of the left panel** (mirror of the docked new-instance wizard on the right), without a full-screen overlay. Support **Start** when offline and **Stop** when online (Podman machine / Docker Desktop on Windows), with **`y`/`n` confirmation before Stop**.

## Non-goals

- Installing Docker/Podman or `podman machine init`.
- Changing offline instance-start → start-engine-and-retry (already shipped).
- Stopping/starting individual DB containers from this panel (that remains Space / action menu).
- Guaranteeing Docker Desktop stop on every Windows setup (best-effort).
- Mouse support.

## Layout

On `e` (`ModeEngineMenu`), main chrome stays. **One bordered left panel** split internally ~50/50:

```
┌──────────────┬─────────────────────┐
│ DB Instances │ Details & Config    │  ← list top; details full height
├──────────────┤                     │
│ Engines      │                     │  ← dock bottom-left
│ Start/Stop…  │                     │
└──────────────┴─────────────────────┘
```

- Reuse `splitPanelHalfHeight` and the single-panel-with-separator pattern from the wizard dock (avoid stacking two bordered boxes — that misaligns heights).
- Right panel stays full-height details.
- `ModeEngineMenu` renders via `viewMain()` — **no** centered `renderOverlay` for Engines.

## Interaction

| Behavior | Rule |
|----------|------|
| Open | `e` → `ModeEngineMenu`; keyboard focus locked to Engines dock |
| Nav | `↑`/`↓` (or `k`/`j`) among Docker / Podman rows |
| Enter on OFFLINE | Start that engine (no extra confirm) |
| Enter on ONLINE | Arm stop confirm (status prompt); do not stop yet |
| Enter on NOT_INSTALLED | No-op (row non-actionable) |
| `y` | If stop confirm armed → `StopEngine`; clear confirm |
| `n` / Esc / leaving context | Cancel stop confirm; Esc also closes Engines → `ModeMain` |
| Footer | While docked, Engines shortcuts (nav / Enter / Esc); not main shortcuts |
| Busy | One engine start/stop in flight at a time (`engineStarting` / shared busy flag) |

### Row labels

| Health | Label | Actionable |
|--------|-------|------------|
| OFFLINE | `Start Docker` / `Start Podman` | Yes → Start |
| ONLINE | `Stop Docker` / `Stop Podman` | Yes → confirm then Stop |
| NOT_INSTALLED | `Docker: not installed` / `Podman: not installed` | No |

Stop confirm copy (English), e.g.:  
`Stop Podman machine? Press 'y' to confirm, 'n' to cancel`  
`Stop Docker Desktop? Press 'y' to confirm, 'n' to cancel` (Windows wording OK for Docker).

Mutual exclusion with purge / offline-start confirms: clear other confirms when arming this one (same `clearConfirms` pattern).

## Operations (`internal/core`)

Existing: `StartEngine` (Podman `machine start`; Docker Desktop launch + poll until ONLINE).

Add: `StopEngine(ctx, runtimeName string) error`

| Case | Behavior |
|------|----------|
| NOT_INSTALLED | Error wrapping `ErrEngineNotInstalled` |
| Already OFFLINE | No-op success |
| ONLINE + podman | `podman machine stop`, then poll until not ONLINE (or timeout) |
| ONLINE + docker (Windows) | Best-effort quit/stop Docker Desktop, poll until OFFLINE or timeout |
| ONLINE + docker (!windows) | Best-effort message / poll-only if applicable; document limits |
| Timeout / failure | Wrap `ErrEngineStartFailed` or a dedicated `ErrEngineStopFailed` |

Timeout: same ballpark as start (~90s). After start/stop attempt, force one-shot health refresh for badges.

## Architecture

| Area | Change |
|------|--------|
| `tui.go` `View()` | `ModeEngineMenu` → `wrapScreen(viewMain())` |
| `view_main.go` | When `ModeEngineMenu`, split left panel (list + engine dock); Engines footer shortcuts |
| `view_engine.go` | Dock content (no outer modal border); ONLINE → Stop + confirm; wire `StopEngine` |
| `internal/core` | `StopEngine` + platform helpers parallel to start |
| Tests | Docked layout height/content; Stop confirm; Start still works |

## Testing

- In `ModeEngineMenu`, view includes list title, “Container Engines” (or “Engines”), and main header; left height matches right.
- OFFLINE row Enter starts; ONLINE row Enter only arms confirm; `y` calls stop path; `n` cancels.
- Existing offline instance-start retry still works.
- No full-screen Engines overlay.

## Copy / product rules

- All user-facing strings English.
- Do not commit secrets.
