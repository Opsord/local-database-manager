# Local Database Manager 🗄️

A lightweight, high-performance interactive Terminal User Interface (TUI) built in **Go** with **Bubble Tea / Charm** for centralized lifecycle management of local database containers running under **Docker** and **Podman**.

---

## Features

- ⚡ **Ultra-lightweight & Blazing Fast:** < 15 MB idle RAM footprint, 0% CPU in background.
- 📁 **Centralized Instances:** All declarative configurations live in `instances/*.env`.
- 🏷️ **Custom Container Naming:** Define clean container names (`CONTAINER_NAME`), preventing collisions or default directory prefixes.
- 🩺 **Runtime Daemon Health Monitoring:** Live status badges in the header (`🟢 Docker: ONLINE`, `🔴 Podman: OFFLINE`).
- 📋 **One-Key Connection Copy:** Copy standard connection URIs (`postgresql://...`, `sqlserver://...`) directly to the clipboard with `c`.
- 🪄 **Interactive Creation Wizard (`n`):** Step-by-step form to spin up new database instances with automatic free port detection.
- 📜 **Real-time Log Streamer (`l`):** Fullscreen live container logs viewport with smooth scrolling.
- 🧹 **Safe Volume Purge (`d`):** Destroy container and associated data volumes with confirmation.

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
│   ├── core/                        # Domain logic (instance model, parser, ports, runner)
│   └── app/                         # Bubble Tea UI presentation layer
│
├── go.mod
├── go.sum
├── AGENTS.md
└── README.md
```

---

## Keyboard Shortcuts

| Key | Action | Description |
| :--- | :--- | :--- |
| `↑` / `↓` or `k` / `j` | Navigate | Move cursor through configured database instances |
| `Space` | Start / Stop | Toggle container state (`up -d` / `down`) |
| `c` | Copy URI | Copy full connection URI (`postgresql://...`) to clipboard |
| `l` | Live Logs | Open real-time log viewer (`Esc` / `q` to return) |
| `n` | New Instance | Open interactive instance creation wizard |
| `e` | Edit `.env` | Open instance configuration in your default editor (`code` / `notepad`) |
| `d` | Down -v (Purge) | Destroy container and persistent data volume (`y/N` prompt) |
| `r` | Reload | Rescan `instances/` directory and refresh daemon health |
| `q` / `Ctrl+C` | Quit | Exit application |

---

## Getting Started

### 1. Run in Development
```bash
go run ./cmd/db-manager
```

### 2. Build Executable Binary
```bash
go build -o db-manager.exe ./cmd/db-manager
```

---

## Terminal Shortcut Setup (`dbm`)

To run `dbm` from any folder in PowerShell:

1. Open PowerShell and run:
```powershell
$functionCode = @"

function dbm {
    Push-Location "C:\Users\andre\Programacion\Personal\local-database-manager"
    try {
        .\db-manager.exe `$args
    } finally {
        Pop-Location
    }
}
"@

if (!(Test-Path $PROFILE)) { New-Item -ItemType File -Path $PROFILE -Force }
Add-Content -Path $PROFILE -Value $functionCode
. $PROFILE
```

2. Simply type `dbm` anywhere in your terminal:
```powershell
dbm
```

---

## License
MIT
