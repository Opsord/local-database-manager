# Design: Engine Start (Docker Desktop / Podman Machine)

## Goal

Let the user **start** an installed but offline container engine from the TUI—Docker Desktop on Windows or the default Podman machine—and **detect install status** (already partly done). Provide a manual Engines menu and an offer to start+retry when an instance Start fails because the runtime is offline.

## Non-goals

- Installing Docker or Podman.
- `podman machine init` / create (v1 is start-only).
- Stopping / restarting engines.
- A dedicated full-screen “Engine status” screen with streamed logs.
- Guaranteeing Docker Desktop starts on every Windows setup (best-effort launch + poll).
- Linux/macOS-specific start paths beyond what is portable; primary target is Windows for Docker Desktop launch.

## Detection (existing + reuse)

Keep the three health states from `CheckEngineHealth`:

| State | Meaning |
|-------|---------|
| `ONLINE` | Binary in PATH and `info` succeeds |
| `OFFLINE` | Binary in PATH but daemon/`info` fails |
| `NOT_INSTALLED` | Binary not in PATH |

Header badges remain the source of truth. After any start attempt (success or failure), force a one-shot health refresh so badges update.

## Operations (`internal/core`)

Add `StartEngine(ctx context.Context, runtimeName string) error` on `InstanceRunner` / `Runner`.

| Runtime | Behavior |
|---------|----------|
| Already `ONLINE` | No-op; return nil (or treat as success) |
| `NOT_INSTALLED` | Return error wrapping `ErrEngineNotInstalled`; do not attempt start |
| Podman + `OFFLINE` | Run `podman machine start` (default machine). No `init` in v1 |
| Docker + `OFFLINE` (Windows) | Launch Docker Desktop (typical install path / resolve executable), then poll `docker info` until online or timeout |
| Start failure / timeout | Return a typed or wrapped start-failed error; leave health as OFFLINE after refresh |

**Timeout:** fixed constant for v1 (e.g. ~90s). Optional `config.yml` knob is out of scope unless needed later.

**Concurrency:** TUI must not start two engine starts at once (disable menu / ignore while a start is in flight).

## UI

### Engines menu (`e` on main)

Small overlay (same pattern as the instance action menu):

| Health | Menu row |
|--------|----------|
| OFFLINE | Actionable: `Start Docker` / `Start Podman` |
| ONLINE | Informational / disabled: `Docker: online` |
| NOT_INSTALLED | Informational / disabled: `Docker: not installed` |

- `Enter` runs the focused actionable item; `Esc` closes.
- Footer shortcut: `[e] Engines`.
- While starting: status line e.g. `Starting Podman machine…` / `Starting Docker Desktop…`; do not allow a second start.

### Offline instance Start → offer retry

When Start (or toggle start) fails because the instance runtime is offline (`ErrEngineOffline`):

1. Prompt: `Podman is offline. Start engine and retry?` (or Docker), with confirm / cancel (`y`/`n` or Enter/Esc—match existing confirm patterns).
2. On confirm: `StartEngine` for that runtime; on success, retry the instance start once.
3. On cancel or engine start failure: show error status; do not retry.

### Copy

All user-facing strings remain **English**.

## Architecture

| Area | Change |
|------|--------|
| `internal/core/runner.go` (+ tests) | `StartEngine`; Docker Desktop launch + poll; `podman machine start` |
| `internal/core` errors | Reuse `ErrEngineNotInstalled` / `ErrEngineOffline`; add start-failed if useful |
| `InstanceRunner` | Add `StartEngine`; update stubs in app tests |
| `internal/app` | `ModeEngineMenu` (or equivalent), status during start, offline confirm+retry, footer `[e]`, health refresh after start |

No new packages. Prefer existing overlay / confirm UX patterns.

## Testing

- Core: not installed → error, no command; online → no-op; offline podman → invokes `machine start` (stubbable); docker start path unit-tested where feasible (path resolution / poll success-failure).
- TUI: `e` opens menu; disabled rows for online / not installed; start updates status and refreshes health; offline instance start shows confirm; confirm runs StartEngine then retries once.

## Product rules

- Do not commit secrets.
- Do not change compose/instance create wizard beyond engine-start integration at Start failure.
