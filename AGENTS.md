# AGENTS.md

## Project Overview
**Local Database Manager (`db-manager`)** is a lightweight, interactive Terminal User Interface (TUI), headless CLI, and Desktop application (`db-manager-desktop`) built with **Go**, **Bubble Tea / Charm**, and **Wails v2 + React/Tailwind**. It provides centralized lifecycle management (start, stop, logs, purge, connection strings, and instance creation/editing) for local database containers running under **Docker** and **Podman**.

---

## Directory Structure

```text
local-database-manager/
├── engines/                         # Master compose definitions and init scripts
│   ├── postgres/
│   │   ├── docker-compose.yml       # Postgres Docker Compose template
│   │   └── init/                    # Base initialization scripts (.sh)
│   └── sql-server/
│       ├── docker-compose.yml       # SQL Server Docker Compose template
│       ├── docker-entrypoint.sh     # Health check & init runner
│       └── init/                    # Base initialization scripts (.sql)
│
├── instances/                       # Declarative instance configurations
│   ├── *.env                        # Individual instance env files (git-ignored)
│   └── .env.template                # Reference configuration template (tracked in git)
│
├── cmd/
│   ├── db-manager/
│   │   └── main.go                  # Entrypoint (TUI/CLI mode routing & root detection)
│   └── db-manager-desktop/
│       └── main.go                  # Wails v2 Desktop GUI entrypoint
│
├── frontend/                        # Desktop UI (React + TypeScript + Tailwind via pnpm/Vite)
│   ├── embed.go                     # Embeds frontend/dist for db-manager-desktop only
│   └── src/                         # Zed-inspired UI components, themes & Wails bridge
│
├── scripts/                         # Setup and automation helper scripts
│   ├── setup-alias.ps1              # Windows PowerShell shortcut setup script
│   └── setup-alias.sh               # Bash/Zsh shortcut setup script
│
├── internal/
│   ├── config/
│   │   └── config.go                # config.yml loader (engine health poll interval)
│   ├── logger/
│   │   └── logger.go                # Structured JSON logger (.db-manager.log via slog)
│   ├── cli/
│   │   └── cli.go                   # Headless CLI subcommands (list, start, stop, uri, env, logs)
│   ├── core/                        # Core domain logic & runtime providers
│   │   ├── instance.go              # DatabaseInstance model, .env parser, writer & URI builder
│   │   ├── ports.go                 # Local socket availability & next free port finder
│   │   ├── runner.go                # Runner facade delegating to RuntimeProvider
│   │   ├── runtime_provider.go      # RuntimeProvider interface & daemon polling helpers
│   │   ├── docker_provider.go       # Docker Compose lifecycle & health implementation
│   │   ├── podman_provider.go       # Rootless podman run lifecycle & machine management
│   │   ├── engine_start_windows.go  # Windows Docker Desktop detached launch/stop
│   │   ├── engine_start_other.go    # Non-Windows Docker daemon stub
│   │   ├── scanner.go               # Instance discovery scanner
│   │   ├── volume.go                # Version normalization & volume naming helpers
│   │   ├── errors.go                # Domain error types & AppError
│   │   ├── compose_error.go         # Compose/runtime stderr parser & hints
│   │   └── clipboard.go             # Windows/OS clipboard & editor opener
│   ├── desktop/                     # Decoupled Desktop backend service & tests
│   │   ├── service.go               # Wails-bound service methods & async log streaming
│   │   └── service_test.go          # Unit tests with mock InstanceRunner
│   └── app/                         # Bubble Tea UI presentation layer
│       ├── styles.go                # Lip Gloss styles, colors, and layout borders
│       ├── helpers.go               # Layout calculations, detail rows & truncation
│       ├── detail_copy.go           # Mouse double-click token hit-testing & copy
│       ├── tui.go                   # Root AppModel & state routing
│       ├── view_main.go             # Split view (instances list + details panel)
│       ├── view_wizard.go           # Interactive create/edit instance docked wizard
│       ├── view_engine.go           # Docked container engine start/stop menu
│       ├── view_modal.go            # Docked action menu & help overlay
│       └── view_logs.go             # Real-time container log streamer viewport
│
├── config.yml                       # App settings (engine health poll interval)
├── go.mod
├── go.sum
└── README.md
```

---

## Technical Stack & Libraries
- **Language:** Go 1.26+
- **TUI Framework:** `github.com/charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **Desktop Framework:** `github.com/wailsapp/wails/v2` (native OS WebView2 / WebKit)
- **Desktop Frontend:** React 18, TypeScript, Tailwind CSS, Vite (managed with `pnpm`)
- **Clipboard Helper:** `github.com/atotto/clipboard` (with Windows `clip.exe` fallback)

---

## Architecture & Design Rules

1. **Language & UI Copy:**
   - All user-facing UI copy, keyboard shortcuts, messages, and placeholders **MUST be in English** by default.
2. **Container Naming Isolation & Runtimes:**
   - Always specify `CONTAINER_NAME` and `COMPOSE_PROJECT_NAME` in instances.
   - `DockerProvider` passes `-p <COMPOSE_PROJECT_NAME>` to `docker compose` and references `container_name: ${CONTAINER_NAME}` to avoid colliding with the parent folder's name.
   - `PodmanProvider` executes rootless `podman run -d --replace --name <CONTAINER_NAME>` directly.
3. **Daemon Health Detection:**
   - Check Docker (`docker info`) and Podman (`podman info`) daemon status before attempting container operations and reflect state in the header badges.
4. **Port Allocation:**
   - Use `core.FindNextFreePort()` to suggest available TCP ports without collisions.
5. **Security & Git Hygiene:**
   - Never commit sensitive `instances/*.env` credentials; always keep `instances/.env.template` clean and tracked.

---

## Development Commands

### Run Unit Tests
```bash
go test ./... -v
```

### Run Locally
```bash
go run ./cmd/db-manager
```

### Build Executable Binary
```bash
go build -o db-manager.exe ./cmd/db-manager
```

### Build Desktop App (Frontend + Wails Binary)
```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build
go build -tags "desktop,production" -ldflags "-s -w -H windowsgui" -o db-manager-desktop.exe ./cmd/db-manager-desktop
```
