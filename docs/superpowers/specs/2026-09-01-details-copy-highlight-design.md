# Design: Highlight Copied Token in Details Panel

## Goal

After a successful double-click token copy in the details panel, **visually highlight the copied token** for ~3 seconds (same window as the `Copied: …` status), so the user can see exactly what went to the clipboard.

## Non-goals

- Drag selection or persistent selection chrome.
- Highlighting on failed clipboard copy.
- Changing `c` / `E`/`x` / `Ctrl+C` behavior.
- Highlighting labels, `[c] copy`, or tokens outside the successful hit.

## Behavior

| Event | Result |
|--------|--------|
| Double-click copies successfully | Status `Copied: <token>`; that token is highlighted in Details |
| Highlight / status lifetime | ~3 seconds (same clear tick as copy status today) |
| Another successful copy before clear | Highlight moves to the new token; timer restarts |
| Clipboard failure | `Failed to copy: …` only; **no** highlight |
| Instance change / no selection | Clear highlight immediately |
| Clear tick fires | Clear status **and** highlight together |

Style: invert/accent the token cells (e.g. `SelectedBg` background + readable foreground), consistent with existing selection chrome. English-only UI copy (AGENTS.md).

## Architecture

### State

On `AppModel`:

- `copiedHit *copyHit` (or `copyHit` + `hasCopiedHit bool`) holding the last successfully copied hit (absolute screen rect + `Text`).

Set only on successful `CopyToClipboard` in the existing double-click handler (`handleDetailsMouseAt`). Clear on:

- Shared clear message with status (~3s tick), **or** a dedicated clear that always clears both status and highlight when the copy timer fires.
- Selection / instance change / empty details.

### Render

When building details content, apply highlight styling to the value segment that matches `copiedHit` (same token text and position as the hit used for copy — prefer matching the hit identity/rect, not “first substring match”).

Reuse the existing hit map / token layout so highlight and click targets stay aligned (including Status icon prefix and truncated URI/CLI visible tokens).

### Input / shortcuts

No new keys. Mouse double-click path unchanged except setting/clearing `copiedHit` and rendering.

## Testing

- Successful copy sets `copiedHit.Text` to the copied token.
- Clear tick / clear message empties `copiedHit`.
- Changing selected instance clears highlight.
- Optional: rendered details output contains highlight styling for the copied span.

## Product rules

- User-facing strings remain English.
- Full URI copy still via `c`; highlight only applies to double-click token copies.
