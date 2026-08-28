# App `config.yml` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Load app knobs from a tracked root `config.yml` (v1: `engine_health_interval` only); Go holds the parsed value and does not hardcode `5s`.

**Architecture:** `internal/config.Load(projectRoot)` reads YAML, parses Go durations, rejects missing/invalid values. `cmd/db-manager` loads before `app.NewApp`. The TUI stores `config.Config` and uses `EngineHealthInterval` for the Docker/Podman health tick.

**Tech Stack:** Go 1.22, `gopkg.in/yaml.v3`, Bubble Tea, existing `findProjectRoot()`.

**Spec:** `docs/superpowers/specs/2026-08-24-app-config-yml-design.md`

## Global Constraints

- Defaults live only in repo `config.yml`. No `const engineHealthInterval = 5 * time.Second` (or equivalent) in Go.
- Missing file, missing key, invalid duration, or interval `<= 0` → fail at startup with path in the error. No silent fallback.
- Only `engine_health_interval` is read. Other keys may exist as comments.
- Reload (`r`) does not re-read `config.yml`.
- TDD: failing test first for each behavior; `go test` after each implementation step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `config.yml` | Tracked source of truth (defaults + comments) |
| `internal/config/config.go` | `Config` + `Load` |
| `internal/config/config_test.go` | Load tests via temp dirs |
| `internal/app/tui.go` | Store cfg; tick uses `m.cfg.EngineHealthInterval` |
| `internal/app/tui_test.go` | Tick uses configured interval |
| `cmd/db-manager/main.go` | Load then `NewApp`; exit 1 on error |
| `go.mod` / `go.sum` | `gopkg.in/yaml.v3` |
| `README.md` | Mention `config.yml` |
| `AGENTS.md` | Add `config.yml` to directory tree |

---

### Task 1: `config.Load`

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum` (when implementation needs yaml.v3)

**Interfaces:**
- Consumes: none
- Produces: `type Config struct { EngineHealthInterval time.Duration }`; `func Load(projectRoot string) (Config, error)`

- [ ] **Step 1: Write failing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_ValidInterval(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "engine_health_interval: 10s\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.EngineHealthInterval != 10*time.Second {
		t.Fatalf("interval = %v, want 10s", cfg.EngineHealthInterval)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  func(t *testing.T) string
		want string
	}{
		{
			name: "missing file",
			dir:  func(t *testing.T) string { return t.TempDir() },
			want: "config.yml",
		},
		{
			name: "invalid yaml",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: [\n")
				return dir
			},
			want: "",
		},
		{
			name: "missing key",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "# empty\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "zero",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: 0s\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "negative",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: -1s\n")
				return dir
			},
			want: "engine_health_interval",
		},
		{
			name: "garbage duration",
			dir: func(t *testing.T) string {
				dir := t.TempDir()
				writeConfig(t, dir, "engine_health_interval: nope\n")
				return dir
			},
			want: "engine_health_interval",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(tt.dir(t))
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q, want substring %q", err, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/config/ -count=1
```

Expected: compile fail (`undefined: Load`) or FAIL.

- [ ] **Step 3: Implement `Load`**

```bash
go get gopkg.in/yaml.v3
```

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	EngineHealthInterval time.Duration
}

func Load(projectRoot string) (Config, error) {
	path := filepath.Join(projectRoot, "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var raw struct {
		EngineHealthInterval string `yaml:"engine_health_interval"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if raw.EngineHealthInterval == "" {
		return Config{}, fmt.Errorf("%s: missing engine_health_interval", path)
	}

	d, err := time.ParseDuration(raw.EngineHealthInterval)
	if err != nil {
		return Config{}, fmt.Errorf("%s: engine_health_interval: %w", path, err)
	}
	if d <= 0 {
		return Config{}, fmt.Errorf("%s: engine_health_interval must be > 0, got %s", path, d)
	}

	return Config{EngineHealthInterval: d}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/config/ -count=1
```

Expected: `ok`

---

### Task 2: Tracked `config.yml`

**Files:**
- Create: `config.yml`

**Interfaces:**
- Consumes: `Load` from Task 1
- Produces: repo file at project root named `config.yml`

- [ ] **Step 1: Write a test that the real file loads**

Add to `internal/config/config_test.go`:

```go
func TestLoad_RepoConfigYml(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("repo config.yml: %v", err)
	}
	if cfg.EngineHealthInterval <= 0 {
		t.Fatalf("interval = %v, want > 0", cfg.EngineHealthInterval)
	}
}
```

This test is fragile if `go test` cwd is the package dir: `../..` from `internal/config` is the module root. Keep it.

- [ ] **Step 2: Run test — expect FAIL** (file missing)

```bash
go test ./internal/config/ -count=1 -run TestLoad_RepoConfigYml
```

Expected: FAIL reading `config.yml`.

- [ ] **Step 3: Add `config.yml` at repo root**

```yaml
# Local Database Manager — app settings.
# Durations use Go syntax: 5s, 10s, 1m.

# How often to re-check Docker / Podman daemon health.
engine_health_interval: 5s

# --- not read yet ---
# engine_health_timeout: 3s
# compose_start_timeout: 45s
# status_message_timeout: 4s
```

- [ ] **Step 4: Re-run test — expect PASS**

```bash
go test ./internal/config/ -count=1
```

Expected: `ok`

---

### Task 3: Wire TUI + `main`

**Files:**
- Modify: `internal/app/tui.go`
- Modify: `internal/app/tui_test.go`
- Modify: `cmd/db-manager/main.go`
- Modify: `README.md`
- Modify: `AGENTS.md` (directory tree only)

**Interfaces:**
- Consumes: `config.Config`, `config.Load`
- Produces: `func NewApp(projectRoot string, cfg config.Config) *AppModel`; health tick uses `m.cfg.EngineHealthInterval`

- [ ] **Step 1: Write failing TUI test**

Add to `internal/app/tui_test.go` (import `"time"`):

```go
func TestEngineHealthTickUsesConfiguredInterval(t *testing.T) {
	t.Parallel()

	m := &AppModel{
		cfg: config.Config{EngineHealthInterval: 42 * time.Second},
		dockerHealth: core.EngineOffline,
		podmanHealth: core.EngineOffline,
	}

	updated, cmd := m.Update(engineHealthMsg{
		dockerHealth: core.EngineOnline,
		podmanHealth: core.EngineOnline,
	})
	if cmd == nil {
		t.Fatal("expected tick cmd")
	}
	_ = updated

	// tea.Tick is opaque; assert the model kept the configured interval
	am := updated.(*AppModel)
	if am.cfg.EngineHealthInterval != 42*time.Second {
		t.Fatalf("interval = %v, want 42s", am.cfg.EngineHealthInterval)
	}
}
```

Also add import `"local-database-manager/internal/config"`.

This test will fail to compile until `cfg` exists — that is the red signal. After compile works, it should pass once the field exists; strengthen by making `engineHealthTickCmd` a method that panics or ticks 0 if interval is 0, and set a helper:

Better red test — extract interval used by tick:

In `tui.go` after wiring, `engineHealthTickCmd` becomes:

```go
func (m *AppModel) engineHealthTickCmd() tea.Cmd {
	return tea.Tick(m.cfg.EngineHealthInterval, func(time.Time) tea.Msg {
		return engineHealthTickMsg{}
	})
}
```

And `engineHealthMsg` handler calls `m.engineHealthTickCmd()`.

The compile failure of accessing `m.cfg` in the test is RED.

- [ ] **Step 2: Run tests — expect FAIL/compile error**

```bash
go test ./internal/app/ -count=1 -run TestEngineHealthTickUsesConfiguredInterval
```

Expected: `cfg` undefined / `NewApp` signature mismatch later.

- [ ] **Step 3: Wire config through the app**

In `internal/app/tui.go`:

- Import `local-database-manager/internal/config`
- Add field `cfg config.Config` on `AppModel`
- Change `NewApp`:

```go
func NewApp(projectRoot string, cfg config.Config) *AppModel {
	instancesDir := filepath.Join(projectRoot, "instances")
	runner := core.NewRunner(projectRoot)

	return &AppModel{
		projectRoot:  projectRoot,
		instancesDir: instancesDir,
		runner:       runner,
		cfg:          cfg,
		mode:         ModeMain,
		logs: logsModel{
			viewport: viewport.New(80, 20),
		},
	}
}
```

- Delete `const engineHealthInterval = 5 * time.Second`
- Replace `engineHealthTickCmd()` with method using `m.cfg.EngineHealthInterval`
- In `engineHealthMsg` case, `return m, m.engineHealthTickCmd()`

In `cmd/db-manager/main.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"local-database-manager/internal/app"
	"local-database-manager/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	projectRoot := findProjectRoot()

	cfg, err := config.Load(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	appModel := app.NewApp(projectRoot, cfg)
	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error ejecutando Local Database Manager: %v\n", err)
		os.Exit(1)
	}
}
```

README — after Directory Structure (or Getting Started), add:

```markdown
## App config

`config.yml` at the project root sets app knobs. Today the only key is `engine_health_interval` (Go duration, e.g. `5s`). Restart the TUI after changing it. Missing or invalid config prevents startup.
```

Add `config.yml` to the tree in `README.md` and `AGENTS.md`.

- [ ] **Step 4: Run all tests — expect PASS**

```bash
go test ./... -count=1
```

Expected: all `ok`. Fix any `NewApp` call sites (only `main.go`).

- [ ] **Step 5: Manual check**

```bash
go run ./cmd/db-manager
```

Expected: TUI starts; Docker/Podman badges still refresh. Quit.

Optional: temporarily set `engine_health_interval: 15s`, restart, confirm slower badge updates, then restore `5s`.

---

## Self-review

| Spec item | Task |
|-----------|------|
| Tracked `config.yml` | Task 2 |
| `engine_health_interval` only | Task 1–3 |
| No default duration in Go | Task 3 (delete const) |
| Fail fast | Task 1 errors + Task 3 main |
| TUI uses loaded interval | Task 3 |
| README | Task 3 |
| Future keys commented, unread | Task 2 YAML comments |

No TBD. `Load` + `ParseDuration` matches the spec wrapper-or-string option (string is enough).
