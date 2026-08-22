# AGENTS.md

## Project Overview
**Local Database Manager (`db-manager`)** is a lightweight, interactive Terminal User Interface (TUI) built with **Go** and the **Bubble Tea / Charm** ecosystem. It provides centralized lifecycle management (start, stop, logs, purge, connection strings, and instance creation) for local database containers running under **Docker** and **Podman**.

---

## Directory Structure

```text
local-database-manager/
├── engines/                         # Master compose definitions and init scripts
│   ├── postgres/
│   │   ├── docker-compose.yml       # Postgres Docker Compose template
│   │   ├── podman-compose.yml       # Postgres Podman Compose template
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
│   └── db-manager/
│       └── main.go                  # CLI entrypoint and project root detection
│
├── internal/
│   ├── core/                        # Core domain logic
│   │   ├── instance.go              # DatabaseInstance model, .env parser & URI builder
│   │   ├── ports.go                 # Local socket availability & next free port finder
│   │   ├── runner.go                # Docker/Podman compose lifecycle & daemon health check
│   │   ├── scanner.go               # Instance discovery scanner
│   │   └── clipboard.go             # Windows/OS clipboard & editor opener
│   └── app/                         # Bubble Tea UI presentation layer
│       ├── styles.go                # Lip Gloss styles, colors, and layout borders
│       ├── tui.go                   # Root AppModel & state routing
│       ├── view_main.go             # Split view (instances list + details panel)
│       ├── view_wizard.go           # Interactive new instance creation form modal
│       └── view_logs.go             # Real-time container log streamer viewport
│
├── go.mod
├── go.sum
└── README.md
```

---

## Technical Stack & Libraries
- **Language:** Go 1.22+
- **TUI Framework:** `github.com/charmbracelet/bubbletea`
- **Component Primitives:** `github.com/charmbracelet/bubbles` (`textinput`, `viewport`)
- **Styling & Colors:** `github.com/charmbracelet/lipgloss`
- **Clipboard Helper:** `github.com/atotto/clipboard` (with Windows `clip.exe` fallback)

---

## Architecture & Design Rules

1. **Language & UI Copy:**
   - All user-facing UI copy, keyboard shortcuts, messages, and placeholders **MUST be in English** by default.
2. **Container Naming Isolation:**
   - Always specify `CONTAINER_NAME` and `COMPOSE_PROJECT_NAME` in instances.
   - Compose commands must pass `-p <COMPOSE_PROJECT_NAME>` and reference `container_name: ${CONTAINER_NAME}` to avoid colliding or taking the parent folder's name.
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
