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

---

## Cursor Cloud specific instructions

Standard commands (test/run/build) are documented above and in `README.md`. Go deps are refreshed automatically by the environment update script (`go mod download`). Notes below are the non-obvious gotchas for this VM.

### Docker (required for real end-to-end container lifecycle)
The app shells out to `docker` / `podman` directly (see `internal/core/runner.go`) to manage database containers. Docker CE is preinstalled in the base image, but a few caveats apply:

- **No systemd in this container**, so `systemctl start docker` does NOT work. Start the daemon manually (persist it in a tmux session), e.g.:
  ```bash
  sudo dockerd > /tmp/dockerd.log 2>&1 &
  ```
- **The daemon socket must be accessible without sudo**, because the app invokes `docker` as the `ubuntu` user (not via sudo). Group membership changes don't take effect in existing sessions here, so after starting the daemon run:
  ```bash
  sudo chmod 666 /var/run/docker.sock
  ```
- **Storage/driver config is pinned in `/etc/docker/daemon.json`**: `fuse-overlayfs` storage driver with the containerd snapshotter disabled. This is required because Docker 29 + this VM's kernel cannot use the default `overlay2`. Do not remove this or the daemon will fail to start containers.
- Verify with `docker info` (expect `Docker: ONLINE` in the TUI header) and `docker run --rm hello-world`.

### Running / demoing the TUI
- It is a full-screen Bubble Tea alt-screen TUI; interact with it in a real terminal (computer-use), not by piping.
- Wizard flow (`n`): after typing the instance Name, the remaining fields auto-fill, so you can press `Enter` through them to the review step.
- Instance `.env` files created by the wizard land in `instances/*.env` and are git-ignored — never commit them.
- The list may show a container as `RUNNING` immediately; the `READY` (TCP accepting-connections) state is re-probed on refresh (`r`).

### Lint note
`gofmt -l .` reports `internal/app/tui.go` as unformatted; this is pre-existing in the repo, not something introduced by setup. `go vet ./...` passes clean.
