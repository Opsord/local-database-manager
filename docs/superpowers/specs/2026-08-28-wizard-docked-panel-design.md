# Design: Docked New-Instance Wizard (Right Panel Bottom)

## Goal

When the user presses `n`, open the new-instance wizard in the **bottom half of the right panel** without replacing the full screen. Keep the instance list and the selected instance details visible; keyboard focus stays locked on the wizard until cancel or successful create.

## Non-goals

- Changing help, action menu, or logs overlays.
- Mouse / scroll-wheel support.
- Changing wizard field order, unlock rules, engine-default recalculation, or `.env` save behavior (those stay as in the vertical-nav wizard design).
- Redesigning the main split beyond the right-panel dock when `ModeWizard` is active.

## Layout

On `n` (`ModeWizard`), the main chrome stays:

- Header (title + Docker/Podman badges)
- Left panel: DB Instances list (visible, inactive for keys)
- Footer: status line + shortcut bar (wizard-oriented while docked; see Interaction)
- Right panel: **split vertically ~50/50** of `contentHeight`

```
┌──────────────┬─────────────────────┐
│ DB Instances │ Details & Config    │  ← top half (selected instance or empty hint)
│              ├─────────────────────┤
│              │ New Database…       │  ← bottom half (wizard + internal scroll)
└──────────────┴─────────────────────┘
```

- **Top half:** same details content as today for the current selection (or the “Select an instance…” placeholder). Height is fixed at roughly half the right panel; excess detail content may clip (same practical constraint as today in a shorter box — no new details scroll required for this feature).
- **Bottom half:** docked wizard titled “New Database Instance”, active-panel styling so focus is obvious, fixed height (~other half).
- **Split:** a clear separator between the two halves (panel border or `panelSeparator` style consistent with existing surface rules).

`ModeWizard` must **not** use full-screen `renderOverlay` for the wizard. Help / action menu keep their current overlay behavior.

## Interaction

| Behavior | Rule |
|----------|------|
| Open | `n` → `ModeWizard`; wizard captures all keys |
| Keys | Existing wizard bindings (↑↓ / ←→ / Enter / b / Backspace / Esc) unchanged |
| List / details | Visible only; no list navigation, filter, or main shortcuts while wizard is open |
| Cancel | `Esc` (or existing cancel path) → `ModeMain`; right panel returns to full-height details |
| Success | After create → `ModeMain` as today |
| Footer shortcuts | While docked, replace main shortcut bar with wizard hints so ↑↓ / Enter are not contradicted by main copy |

Form logic (progressive unlock, lateral engine/runtime, `b` back, engine default recalculation, `BgSurface` row rendering) is unchanged.

## Scroll

Bottom dock height is fixed. Wizard body may exceed it.

- **Hints row** fixed at the bottom of the dock panel.
- **Everything above hints** (title, separator, unlocked rows) lives in a scrollable region (viewport or line offset).
- On focus change (Enter / ↑↓ / `b`), adjust scroll so the **focused row stays visible**.
- Keyboard-only; no mouse scroll requirement.

## Architecture

Touches primarily `internal/app`:

| Area | Change |
|------|--------|
| `tui.go` `View()` | `ModeWizard` renders main layout with dock (not `renderOverlay(viewWizard)`) |
| `view_main.go` | When `mode == ModeWizard`, split right panel into details + wizard; swap footer shortcuts |
| `view_wizard.go` | Render dock-sized content (right panel inner width, constrained height, scroll); keep key/update logic |
| `wizardModel` | Add scroll state (offset or bubbles `viewport`); reset on open/close |

No new packages or dependencies beyond what the app already uses (bubbles viewport is acceptable if already a dependency).

## Testing

- In `ModeWizard`, rendered output includes both details (or empty placeholder) and “New Database Instance”, and still shows the main header (“LOCAL DATABASE MANAGER”).
- Wizard is not presented as a full-screen overlay that hides the split layout.
- Advancing/focusing a step that would sit below the dock height moves scroll so the focused row is in the visible region.
- Existing wizard navigation / engine-default tests continue to pass (behavior unchanged; container is visual).

## Copy / product rules

- All user-facing strings remain English.
- Do not commit secrets; wizard still writes `instances/<name>.env` as today.
