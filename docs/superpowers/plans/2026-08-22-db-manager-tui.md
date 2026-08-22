# Local Database Manager TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir una herramienta CLI TUI interactiva y ultra ligera en Go (Bubble Tea + Lip Gloss) para gestionar bases de datos locales en Docker y Podman con una arquitectura centralizada en `instances/*.env` y plantillas en `engines/`.

**Architecture:** Arquitectura modular en capas: `internal/core` gestiona el escaneo de archivos `.env`, invocación de comandos de Compose (`docker` / `podman`), cálculo de puertos libres y generación de connection strings; `internal/app` gestiona la TUI en Bubble Tea con vistas divididas, modal asistente de creación (`wizard`) y visor de logs.

**Tech Stack:** Go 1.22+, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbles`, `github.com/atotto/clipboard`.

**Spec:** `docs/superpowers/specs/2026-08-22-db-manager-tui-design.md`

## Global Constraints
- El ejecutable debe compilar en Windows como un único `.exe` independiente.
- Las instancias deben residir exclusivamente en `instances/*.env`.
- Los nombres de los contenedores no deben depender del directorio padre (`container_name: ${CONTAINER_NAME}` y `-p <COMPOSE_PROJECT_NAME>`).
- El indicador de estado (`🟢` / `🔴`) debe mostrarse al inicio de cada fila en la lista.

---

### Task 1: Migration & Directory Structure (`engines/` y `instances/`)

**Files:**
- Create: `engines/postgres/docker-compose.yml`
- Create: `engines/postgres/podman-compose.yml`
- Create: `engines/sql-server/docker-compose.yml`
- Create: `engines/sql-server/docker-entrypoint.sh`
- Create: `instances/super_calendar.env`
- Create: `instances/requerimientos.env`
- Create: `instances/.env.template`

**Interfaces:**
- Consumes: Contenido existente en `db-instances/`
- Produces: Estructura desacoplada `engines/` y `instances/` con variables `ENGINE`, `RUNTIME`, `CONTAINER_NAME`, `COMPOSE_PROJECT_NAME`.

- [ ] **Step 1: Crear directorios `engines/` e `instances/` y copiar scripts base**
Crear carpetas `engines/postgres/init`, `engines/sql-server/init`, `instances/`.

- [ ] **Step 2: Configurar `engines/postgres/docker-compose.yml` y `podman-compose.yml`**
Asegurar que los servicios usen `container_name: ${CONTAINER_NAME}` y lean variables estándar.

- [ ] **Step 3: Configurar `engines/sql-server/docker-compose.yml` y scripts**
Adaptar para soporte de `container_name: ${CONTAINER_NAME}` y scripts `init/`.

- [ ] **Step 4: Crear archivos migrados en `instances/`**
Crear `instances/super_calendar.env` y `instances/requerimientos.env` con sus credenciales y variables declarativas.

- [ ] **Step 5: Commit de la estructura migrada**
```bash
git add engines/ instances/
git commit -m "refactor: restructure database engines and centralized instances directory"
```

---

### Task 2: Go Module & Core Domain Models (`internal/core/instance.go`)

**Files:**
- Create: `go.mod`
- Create: `internal/core/instance.go`
- Create: `internal/core/instance_test.go`

**Interfaces:**
- Produces:
  - `type ContainerStatus string` (`StatusRunning`, `StatusStopped`, `StatusUnknown`)
  - `type DatabaseInstance struct { ... }`
  - `func ParseEnvFile(path string) (*DatabaseInstance, error)`
  - `func (d *DatabaseInstance) ConnectionURI() string`
  - `func (d *DatabaseInstance) CLICommand() string`

- [ ] **Step 1: Inicializar go.mod y dependencias**
```bash
go mod init local-database-manager
go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss github.com/charmbracelet/bubbles github.com/atotto/clipboard
```

- [ ] **Step 2: Escribir pruebas unitarias que fallen para `ParseEnvFile` y `ConnectionURI`**
En `internal/core/instance_test.go`, probar parseo de variables PostgreSQL y SQL Server, y armado de URI con `postgresql://` y `sqlcmd`.

- [ ] **Step 3: Implementar `internal/core/instance.go`**
Escribir el parser de archivos `.env` y los métodos `ConnectionURI()` y `CLICommand()`.

- [ ] **Step 4: Ejecutar pruebas unitarias para verificar que pasen**
```bash
go test ./internal/core -v
```

- [ ] **Step 5: Commit**
```bash
git add go.mod go.sum internal/core/instance.go internal/core/instance_test.go
git commit -m "feat(core): implement database instance domain model and env parser"
```

---

### Task 3: Scanner & Free Port Inspector (`internal/core/scanner.go`, `internal/core/ports.go`)

**Files:**
- Create: `internal/core/ports.go`
- Create: `internal/core/ports_test.go`
- Create: `internal/core/scanner.go`
- Create: `internal/core/scanner_test.go`

**Interfaces:**
- Consumes: `internal/core/instance.go`
- Produces:
  - `func IsPortAvailable(port int) bool`
  - `func FindNextFreePort(defaultPort int, instances []*DatabaseInstance) int`
  - `func ScanInstances(instancesDir string) ([]*DatabaseInstance, error)`

- [ ] **Step 1: Escribir pruebas para detección de puertos y escaneo de instancias**
En `ports_test.go` y `scanner_test.go`, simular archivos `.env` y verificar escaneo y asignación de puertos.

- [ ] **Step 2: Implementar `internal/core/ports.go`**
Verificar disponibilidad de socket TCP local en `127.0.0.1:<port>` y evitar colisiones con instancias existentes.

- [ ] **Step 3: Implementar `internal/core/scanner.go`**
Recorrer `instances/*.env` (ignorando `.template.env`), parsear cada uno y retornar la lista ordenada.

- [ ] **Step 4: Ejecutar pruebas unitarias**
```bash
go test ./internal/core -v
```

- [ ] **Step 5: Commit**
```bash
git add internal/core/ports.go internal/core/ports_test.go internal/core/scanner.go internal/core/scanner_test.go
git commit -m "feat(core): implement instance scanner and port collision detector"
```

---

### Task 4: Runner & Lifecycle Adapter (`internal/core/runner.go`)

**Files:**
- Create: `internal/core/runner.go`
- Create: `internal/core/runner_test.go`

**Interfaces:**
- Consumes: `internal/core/instance.go`
- Produces:
  - `type Runner struct { ProjectRoot string }`
  - `func (r *Runner) Start(inst *DatabaseInstance) error`
  - `func (r *Runner) Stop(inst *DatabaseInstance) error`
  - `func (r *Runner) DownVolumes(inst *DatabaseInstance) error`
  - `func (r *Runner) CheckStatus(inst *DatabaseInstance) ContainerStatus`
  - `func (r *Runner) LogsCommand(inst *DatabaseInstance) *exec.Cmd`

- [ ] **Step 1: Escribir pruebas unitarias para la construcción de comandos de ejecución**
Verificar que los argumentos pasados a `docker compose` o `podman-compose` incluyan `-p <name>`, `-f engines/...`, `--env-file instances/...`.

- [ ] **Step 2: Implementar `internal/core/runner.go`**
Ejecutar comandos del sistema de forma segura con `os/exec`, capturando salida y errores.

- [ ] **Step 3: Ejecutar pruebas unitarias**
```bash
go test ./internal/core -v
```

- [ ] **Step 4: Commit**
```bash
git add internal/core/runner.go internal/core/runner_test.go
git commit -m "feat(core): implement docker/podman compose lifecycle runner"
```

---

### Task 5: Windows Clipboard & Editor Helpers (`internal/core/clipboard.go`)

**Files:**
- Create: `internal/core/clipboard.go`

**Interfaces:**
- Produces:
  - `func CopyToClipboard(text string) error`
  - `func OpenInEditor(filePath string) error`

- [ ] **Step 1: Implementar `internal/core/clipboard.go`**
Usar `github.com/atotto/clipboard` con fallback a `clip.exe` en Windows. Implementar apertura de editor vía `code` o `notepad`.

- [ ] **Step 2: Commit**
```bash
git add internal/core/clipboard.go
git commit -m "feat(core): implement clipboard and editor helpers"
```

---

### Task 6: TUI Styles & Main Split View (`internal/app/styles.go`, `internal/app/tui.go`, `internal/app/view_main.go`)

**Files:**
- Create: `internal/app/styles.go`
- Create: `internal/app/tui.go`
- Create: `internal/app/view_main.go`

**Interfaces:**
- Consumes: `internal/core`
- Produces:
  - `type AppModel struct { ... }` (Implementa `tea.Model`)
  - `func NewApp(projectRoot string) *AppModel`
  - Renderizado de lista con icono `🟢`/`🔴` a la izquierda y panel de detalles a la derecha.

- [ ] **Step 1: Definir paleta de estilos y bordes con Lip Gloss**
Estilos modernos con colores consistentes (verde para running, rojo para stopped, azul/cian para selección).

- [ ] **Step 2: Implementar navegación y renderizado de la vista principal**
Manejo de teclas `↑`, `↓`, `k`, `j`, `Espacio` (Start/Stop), `c` (Copiar), `r` (Recargar), `d` (Purge confirm), `q` (Salir).

- [ ] **Step 3: Commit**
```bash
git add internal/app/styles.go internal/app/tui.go internal/app/view_main.go
git commit -m "feat(tui): implement split-view main screen and list navigation"
```

---

### Task 7: TUI Creation Wizard Modal (`internal/app/view_wizard.go`)

**Files:**
- Create: `internal/app/view_wizard.go`

**Interfaces:**
- Consumes: `internal/core/ports.go`, `internal/core/instance.go`
- Produces:
  - Formulario modal paso a paso para crear un nuevo `instances/<name>.env`.
  - Auto-sugerencia de nombre de contenedor y puerto libre.

- [ ] **Step 1: Implementar el sub-modelo del Wizard (`wizardModel`)**
Campos: Selección de motor (`postgres` / `sqlserver`), runtime (`docker` / `podman`), nombre de instancia, nombre de contenedor, puerto libre sugerido, nombre de BD y volumen.

- [ ] **Step 2: Integrar escritura del archivo `.env` y refresco de la lista principal**
Al confirmar, guarda el archivo en `instances/<name>.env` y selecciona la nueva instancia en la lista.

- [ ] **Step 3: Commit**
```bash
git add internal/app/view_wizard.go
git commit -m "feat(tui): implement interactive instance creation wizard modal"
```

---

### Task 8: TUI Logs Viewer Viewport (`internal/app/view_logs.go`)

**Files:**
- Create: `internal/app/view_logs.go`

**Interfaces:**
- Consumes: `bubbles/viewport`, `internal/core/runner.go`
- Produces:
  - Modal/pantalla completa con visor de logs en tiempo real (`compose logs --tail=100 -f`).
  - Navegación con scroll y salida con `Esc` / `q`.

- [ ] **Step 1: Implementar `logsModel` con `bubbles/viewport` y canal asíncrono de streaming**
Capturar la salida del comando de logs y actualizar el viewport.

- [ ] **Step 2: Integrar tecla `l` en la vista principal para abrir el visor de logs**

- [ ] **Step 3: Commit**
```bash
git add internal/app/view_logs.go
git commit -m "feat(tui): implement full-screen real-time logs streamer"
```

---

### Task 9: Entrypoint CLI & Integración Global (`cmd/db-manager/main.go`)

**Files:**
- Create: `cmd/db-manager/main.go`
- Create: `README.md` (Documentación de uso de la herramienta)

**Interfaces:**
- Produces:
  - Ejecutable `db-manager.exe`

- [ ] **Step 1: Implementar `cmd/db-manager/main.go`**
Configurar banderas CLI (ej. ruta personalizada de instancias si se desea) y arrancar `tea.NewProgram(app, tea.WithAltScreen())`.

- [ ] **Step 2: Crear documentación en `README.md`**
Explicar cómo compilar, atajos de teclado y estructura de carpetas.

- [ ] **Step 3: Verificar compilación y pruebas globales**
```bash
go test ./... -v
go build -o db-manager.exe ./cmd/db-manager
```

- [ ] **Step 4: Commit**
```bash
git add cmd/db-manager/main.go README.md
git commit -m "feat: complete db-manager CLI entrypoint and documentation"
```
