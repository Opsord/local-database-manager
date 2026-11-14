# Design: Double-Click Token Copy in Details Panel

## Goal

Allow the user to **double-click a word (token)** in the right-hand instance details panel to copy that token to the clipboard, with a short status-bar notification. Avoid any change to `Ctrl+C` (quit) or existing copy shortcuts (`c` URI, `E`/`x` `.env` block).

## Non-goals

- Drag-to-select / free-range selection.
- Copy via `Ctrl+C` or remapping quit.
- Mouse selection in the instance list, logs, wizard dock, action menu, or help.
- Persistent visual highlight of the copied token (may be added later).
- Copying truncated/hidden portions of a value; only the **visible** token text is copied. Full URI remains available via `c`.

## Behavior

| Action | Result |
|--------|--------|
| Double-click on a value token (e.g. `postgres`, `5432`, a URI segment) | Copy that token; status `Copied: <token>` (~3s clear, same pattern as `c`) |
| Double-click on a label (`User:`, `Port:`, …) or `[c] copy` hint | No-op |
| Click outside any token / left list / dock | No-op |
| No instance selected | No hits; no-op |
| Clipboard error | `Failed to copy: …` with error status styling |

Scope: right **details** region only — including the top half when wizard or action menu is docked. Hits must not cover the dock half.

Double-click detection: two left-button presses within ~400–500 ms on the same cell (allow ±0–1 column/row tolerance). A second click far from the first starts a new first-click.

## Architecture

### Hit map

When building details content, also build absolute screen rectangles:

```text
copyHit { X, Y, W, H, Text string }
```

- Built alongside `buildRightDetailsContent` / `renderDetailRows` using the same widths, gaps, truncates, and two-column layout rules.
- Store on `AppModel` as `detailHits []copyHit` (rebuild on view/layout/instance change; cheap).
- Labels and chrome are not added as hits.

### Tokenization

Split each **value** string into tokens for hit boxes. Token characters: alphanumeric plus URI/CLI-friendly set such as `_ . / : @ % + -`. Separators (spaces, `(`, `)`, etc.) are not copyable hits.

Examples:

- `postgres` → one token
- `POSTGRES (DOCKER)` → `POSTGRES`, `DOCKER`
- Truncated URI/CLI → tokens from the **visible** truncated string only

### Input handling

- App already uses `tea.WithMouseCellMotion()`; handle `tea.MouseMsg` in main update when the click falls in the details region.
- On confirmed double-click: hit-test `detailHits` → `core.CopyToClipboard(text)` → set `statusMsg` (+ clear tick).
- Do not change keyboard bindings for `c`, `E`/`x`, `q`, or `ctrl+c`.

### Layout coordinates

Hit `X`/`Y` must account for header, panel borders/padding, left column + gap, details title row, and (when docked) the top-half offset only. Prefer computing origin once per frame from the same helpers used by `viewMain` (`splitPanelWidths`, `panelInnerWidth`, `splitPanelHalfHeight`).

## Error handling

Reuse existing clipboard helper and status patterns. Failed copy sets error status and schedules clear; successful copy uses the same ~3s clear as URI copy.

## Testing

- Unit: tokenize values (plain word, parenthetical engine label, memory string, URI fragments).
- Unit: hit-test point inside/outside rect; prefer first matching hit if overlap (should not overlap).
- Unit: double-click detector (same cell within window; different cell; outside window).
- Optional: building details for a fixture instance yields hits for values and none for label-only regions.

## Copy / product rules

- All user-facing status strings in English (AGENTS.md).
- Existing one-key URI / `.env` export shortcuts unchanged and remain the path for full connection strings.
