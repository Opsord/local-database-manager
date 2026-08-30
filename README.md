# Local Database Manager 🗄️

A lightweight, high-performance interactive Terminal User Interface (TUI) built in **Go** with **Bubble Tea / Charm** for centralized lifecycle management of local database containers running under **Docker** and **Podman**.

---

## Features

- ⚡ **Ultra-lightweight & Blazing Fast:** < 15 MB idle RAM footprint, 0% CPU in background.
- 📁 **Centralized Instances:** All declarative configurations live in `instances/*.env`.
- 🏷️ **Custom Container Naming:** Define clean container names (`CONTAINER_NAME`), preventing collisions or default directory prefixes.
- 🩺 **Runtime Daemon Health Monitoring:** Live status badges in the header (`🟢 Docker: ONLINE`, `🔴 Podman: OFFLINE`).
- 🩺 **Database Readiness Probe (Real Health Check):** Distinguishes between `🟢 READY (Accepting Connections)` and `🟡 STARTING (Engine Booting / Init...)` via live TCP socket probing.
- 📊 **Real-time Memory Monitoring:** Displays live container memory consumption (`RAM Usage: 124.5MiB / 15.6GiB`).
- 📋 **One-Key Connection Copy:** Copy standard connection URIs (`postgresql://...`, `sqlserver://...`) directly to the clipboard with `c`.
- 📦 **Backend .env Block Export (`E` / `x`):** Copy a complete multi-variable configuration block formatted ready to paste into backend projects.
- 🪄 **Interactive Creation Wizard (`n`):** Step-by-step form to spin up new database instances with automatic free port detection.
- 📦 **Postgres Version Selection:** Choose majors 14–18 in the create/edit wizard; volumes are auto-named (`pgdata_<name>_<version>`).
- ✏️ **In-TUI Edit Instance:** Docked wizard (via Action Menu) to edit instance fields; **Runtime is locked** after creation (create a new instance to switch Docker/Podman); `[o]` opens external editor; saving while running prompts restart (`y`/`n`).
- 📜 **Real-time Log Streamer (`l`):** Fullscreen live container logs viewport with smooth scrolling.
- 🧹 **Safe Volume Purge (`d`):** Destroy container and associated data volumes with confirmation.
- 🗑️ **Delete Instance (`D`):** Purge container+volume and remove the instance `.env` from the list (engine must be online).
- 🐳 **Engines Panel (`e`):** Docked left panel to start Docker Desktop / Podman when offline, or stop when online (`y/n` confirm).

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
├── config.yml                       # App settings (engine health poll interval)
├── go.mod
├── go.sum
├── AGENTS.md
└── README.md
```

## App config

`config.yml` at the project root sets app knobs. Today the only key is `engine_health_interval` (Go duration, e.g. `5s`). Restart the TUI after changing it. Missing or invalid config prevents startup.

---

## Keyboard Shortcuts

| Key | Action | Description |
| :--- | :--- | :--- |
| `↑` / `↓` or `k` / `j` | Navigate | Move cursor through configured database instances |
| `Enter` | Action Menu | Dock Action Menu in the bottom-right panel (Start/Stop, Copy URI, Logs, **Edit Instance**, Purge, Delete Instance) |
| `o` | External editor | While **Edit Instance** wizard is open, open the `.env` in your system editor |
| `/` | Live Search | Filter database instances in real-time by name, engine, runtime or port |
| `Space` | Start / Stop | Toggle container state (`up -d` / `down`) |
| `c` | Copy URI | Copy full connection URI (`postgresql://...`) to clipboard |
| `E` / `x` | Export .env | Copy complete backend `.env` block to clipboard |
| `l` | Live Logs | Open real-time log viewer (`Esc` / `q` to return) |
| `n` | New Instance | Open interactive instance creation wizard |
| `e` | Engines | Dock Engines panel (left) — Start when offline; Stop when online (`y/n` confirm) |
| `d` | Purge | Destroy container and volume only (`y/N`); keeps `.env` |
| `D` | Delete Instance | Purge + remove `.env` definition (`y/N`); requires online engine |
| `r` | Reload | Rescan `instances/` directory and refresh daemon health |
| `?` | Help | Open shortcuts cheatsheet and driver recommendations |
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
