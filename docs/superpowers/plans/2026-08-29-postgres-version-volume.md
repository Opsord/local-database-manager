# Postgres Version + Automatic Volumes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users pick Postgres majors 14–18 in create/edit, parametrize compose with `POSTGRES_VERSION`, and derive volume names automatically (read-only preview).

**Architecture:** Store `POSTGRES_VERSION` in the instance `.env` and use `image: postgres:${POSTGRES_VERSION:-18}` in Postgres compose files. Add `DatabaseInstance.Version` plus `DeriveVolumeName`. Wizard gains a Version cycle step; remove editable Volume input and always write the derived volume on save. Edit shows a status warning when the derived volume changes.

**Tech Stack:** Go 1.22+, Bubble Tea, Lip Gloss, Docker/Podman Compose, existing `internal/app` / `internal/core`.

**Spec:** `docs/superpowers/specs/2026-08-29-postgres-version-volume-design.md`

## Global Constraints

- All user-facing UI copy remains English.
- Postgres versions curated only: `14`, `15`, `16`, `17`, `18` (default `18`).
- Volume is never a focusable wizard step; always derived on save.
- Postgres volume: `pgdata_<instance_name>_<version>`; SQL Server: `sqlserver_<instance_name>`.
- No SQL Server version selector; no data migration / `pg_upgrade`.
- Old `.env` without `POSTGRES_VERSION`: parse as `18`; compose default `${POSTGRES_VERSION:-18}` so Start still works.
- TDD: failing test first; `go test` after each implementation step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/core/volume.go` | `DefaultPostgresVersion`, `PostgresVersions`, `DeriveVolumeName`, `NormalizePostgresVersion` |
| `internal/core/volume_test.go` | Unit tests for derive/normalize |
| `internal/core/instance.go` | `Version` field; parse `POSTGRES_VERSION` with default |
| `internal/core/instance_test.go` | Version default / explicit parse cases |
| `engines/postgres/docker-compose.yml` | `image: postgres:${POSTGRES_VERSION:-18}` |
| `engines/postgres/podman-compose.yml` | Same |
| `instances/.env.template` | Document `POSTGRES_VERSION` + auto volume |
| `internal/app/view_wizard.go` | `StepVersion`; drop editable volume; remap inputs; save |
| `internal/app/view_wizard_test.go` | Version cycle, derived volume, save content, edit warning |
| `internal/app/helpers.go` | Details panel `Version:` row for Postgres |
| `README.md` | Short feature note |

### Wizard input remap (breaking for tests)

| Index | Before | After |
|-------|--------|-------|
| 0 | Name | Name |
| 1 | Container | Container |
| 2 | Port | Port |
| 3 | Database | Database |
| 4 | Volume (editable) | Password |
| 5 | Password | Memory |
| 6 | Memory | *(removed)* |

Volume becomes a computed string via `w.derivedVolume()`, not an input.
Version uses `selectedVersionIdx` + `PostgresVersions` (not a textinput).

Step order (Postgres): Engine → Runtime → **Version** → Name → Container → Port → Database → Password → Memory → Review.  
SQL Server: skip Version (Runtime → Name …).

---

### Task 1: Core version/volume helpers + compose

**Files:**
- Create: `internal/core/volume.go`, `internal/core/volume_test.go`
- Modify: `internal/core/instance.go`, `internal/core/instance_test.go`
- Modify: `engines/postgres/docker-compose.yml`, `engines/postgres/podman-compose.yml`
- Modify: `instances/.env.template`

**Interfaces:**
- Produces:
  - `const DefaultPostgresVersion = "18"`
  - `var PostgresVersions = []string{"14", "15", "16", "17", "18"}`
  - `func NormalizePostgresVersion(v string) string` — trim; if empty or not in list → `DefaultPostgresVersion`
  - `func DeriveVolumeName(engine, instanceName, version string) string` — postgres → `pgdata_<name>_<normalizedVersion>`; sqlserver → `sqlserver_<name>`; other → `data_<name>`
  - `DatabaseInstance.Version string` — postgres only; empty for sqlserver
  - Compose image line uses `${POSTGRES_VERSION:-18}`

- [ ] **Step 1: Write failing tests**

```go
// internal/core/volume_test.go
package core

import "testing"

func TestNormalizePostgresVersion(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", DefaultPostgresVersion},
		{"16", "16"},
		{" 17 ", "17"},
		{"99", DefaultPostgresVersion},
		{"latest", DefaultPostgresVersion},
	}
	for _, tc := range cases {
		if got := NormalizePostgresVersion(tc.in); got != tc.want {
			t.Fatalf("NormalizePostgresVersion(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeriveVolumeName(t *testing.T) {
	t.Parallel()
	if got := DeriveVolumeName("postgres", "shop", "16"); got != "pgdata_shop_16" {
		t.Fatalf("got %q", got)
	}
	if got := DeriveVolumeName("postgres", "shop", ""); got != "pgdata_shop_18" {
		t.Fatalf("empty version should normalize, got %q", got)
	}
	if got := DeriveVolumeName("sqlserver", "shop", "16"); got != "sqlserver_shop" {
		t.Fatalf("sqlserver must ignore version, got %q", got)
	}
}
```

```go
// Add to internal/core/instance_test.go
func TestParseEnvFile_PostgresVersionDefault(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "legacy.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-legacy
COMPOSE_PROJECT_NAME=pg-legacy
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=pgdata_legacy
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := ParseEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Version != DefaultPostgresVersion {
		t.Fatalf("Version=%q, want %q", inst.Version, DefaultPostgresVersion)
	}
}

func TestParseEnvFile_PostgresVersionExplicit(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "v16.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-v16
COMPOSE_PROJECT_NAME=pg-v16
POSTGRES_VERSION=16
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=pgdata_v16_16
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := ParseEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Version != "16" {
		t.Fatalf("Version=%q, want 16", inst.Version)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/ -run "TestNormalizePostgresVersion|TestDeriveVolumeName|TestParseEnvFile_PostgresVersion" -v`  
Expected: FAIL (undefined symbols / missing Version)

- [ ] **Step 3: Implement helpers + parser + compose + template**

`internal/core/volume.go`:

```go
package core

import "strings"

const DefaultPostgresVersion = "18"

var PostgresVersions = []string{"14", "15", "16", "17", "18"}

func NormalizePostgresVersion(v string) string {
	v = strings.TrimSpace(v)
	for _, allowed := range PostgresVersions {
		if v == allowed {
			return v
		}
	}
	return DefaultPostgresVersion
}

func DeriveVolumeName(engine, instanceName, version string) string {
	name := strings.TrimSpace(instanceName)
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "postgres":
		return "pgdata_" + name + "_" + NormalizePostgresVersion(version)
	case "sqlserver":
		return "sqlserver_" + name
	default:
		return "data_" + name
	}
}
```

In `instance.go`, add `Version string` to `DatabaseInstance`. In the postgres branch of `ParseEnvFile`:

```go
inst.Version = NormalizePostgresVersion(rawEnv["POSTGRES_VERSION"])
inst.Volume = rawEnv["POSTGRES_VOLUME"]
```

Leave sqlserver `Version` empty.

Compose (both postgres yml files): change `image: postgres:18` → `image: postgres:${POSTGRES_VERSION:-18}`.

Update `instances/.env.template` Postgres example:

```env
# POSTGRES_VERSION=18          # majors 14-18; managed by wizard
# POSTGRES_VOLUME=pgdata_example_18  # auto-derived by the app; do not invent manually
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/ -v`  
Expected: PASS

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add internal/core/volume.go internal/core/volume_test.go internal/core/instance.go internal/core/instance_test.go engines/postgres/docker-compose.yml engines/postgres/podman-compose.yml instances/.env.template
git commit -m "feat(core): Postgres version field and derived volume helpers"
```

---

### Task 2: Wizard — Version step + auto volume + save

**Files:**
- Modify: `internal/app/view_wizard.go`
- Modify: `internal/app/view_wizard_test.go` (remap all `inputs[4|5|6]` volume/password/memory usages)

**Interfaces:**
- Consumes: `core.PostgresVersions`, `core.DefaultPostgresVersion`, `core.DeriveVolumeName`, `core.NormalizePostgresVersion`
- Produces:
  - `StepVersion` inserted after `StepRuntime`; remove `StepVolume` from the step enum (or keep unused — prefer **delete** and renumber)
  - `wizardModel.selectedVersionIdx int`
  - `func (w *wizardModel) derivedVolume() string`
  - `func (w *wizardModel) selectedVersion() string` — returns version string for postgres; ignored for sqlserver
  - Inputs length **6**: `[name, container, port, db, password, memory]`
  - `saveInstance` writes `POSTGRES_VERSION` and derived `POSTGRES_VOLUME` / `SQLSERVER_VOLUME`
  - `cycleOption` on `StepVersion` cycles `selectedVersionIdx`
  - `confirmAdvance`: Runtime → Version (postgres) or Name (sqlserver); Database → Password (skip volume)
  - View: Version option row; Volume preview row (muted, non-focusable) after Database and on Review

Suggested step enum after change:

```go
const (
	StepEngine wizardStep = iota
	StepRuntime
	StepVersion
	StepName
	StepContainerName
	StepPort
	StepDatabase
	StepPassword
	StepMemoryLimit
	StepReview
)
```

`derivedVolume`:

```go
func (w *wizardModel) derivedVolume() string {
	name := strings.TrimSpace(w.inputs[0].Value())
	engine := w.engines[w.selectedEngineIdx]
	ver := ""
	if engine == "postgres" {
		ver = w.selectedVersion()
	}
	return core.DeriveVolumeName(engine, name, ver)
}

func (w *wizardModel) selectedVersion() string {
	if w.selectedVersionIdx < 0 || w.selectedVersionIdx >= len(core.PostgresVersions) {
		return core.DefaultPostgresVersion
	}
	return core.PostgresVersions[w.selectedVersionIdx]
}
```

`saveInstance` postgres fragment must include:

```go
version := w.selectedVersion()
volume := w.derivedVolume()
// ...
POSTGRES_VERSION=%s
...
POSTGRES_VOLUME=%s
```

Remove volume autofill from `applyNameAutofill` / `applyEngineDefaults` (no `inputs[4]` volume). Update password/memory indexes to 4 and 5.

When engine switches away from postgres, Version step is skipped in navigation; when switching to postgres, ensure `selectedVersionIdx` points at default 18 if unset.

- [ ] **Step 1: Write failing tests**

```go
func TestWizardDerivedVolumeIncludesVersion(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.selectedEngineIdx = 0 // postgres
	for i, v := range core.PostgresVersions {
		if v == "16" {
			w.selectedVersionIdx = i
			break
		}
	}
	w.inputs[0].SetValue("shop")
	if got := w.derivedVolume(); got != "pgdata_shop_16" {
		t.Fatalf("derivedVolume=%q", got)
	}
}

func TestWizardSaveWritesPostgresVersionAndVolume(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := newWizardModel("/tmp", dir, nil)
	w.selectedEngineIdx = 0
	for i, v := range core.PostgresVersions {
		if v == "15" {
			w.selectedVersionIdx = i
			break
		}
	}
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[3].SetValue("shop_db")
	w.inputs[4].SetValue("secret")
	w.inputs[5].SetValue("512M")
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "shop.env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "POSTGRES_VERSION=15") {
		t.Fatalf("missing version: %s", s)
	}
	if !strings.Contains(s, "POSTGRES_VOLUME=pgdata_shop_15") {
		t.Fatalf("missing volume: %s", s)
	}
}

func TestWizardSkipsVersionForSQLServer(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.step = StepRuntime
	w.selectedEngineIdx = 1 // sqlserver
	if !w.confirmAdvance() {
		t.Fatal("runtime confirm")
	}
	if w.step != StepName {
		t.Fatalf("sqlserver should skip Version, got step %v", w.step)
	}
}

func TestWizardPostgresRuntimeGoesToVersion(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.step = StepRuntime
	w.selectedEngineIdx = 0
	if !w.confirmAdvance() {
		t.Fatal("runtime confirm")
	}
	if w.step != StepVersion {
		t.Fatalf("want StepVersion, got %v", w.step)
	}
}
```

Also fix any existing tests that set `inputs[4]` as volume / `inputs[5]` password / `inputs[6]` memory — shift to the new indices and assert derived volume instead of editable volume where appropriate.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run "TestWizardDerivedVolume|TestWizardSaveWritesPostgres|TestWizardSkipsVersion|TestWizardPostgresRuntime" -v`  
Expected: FAIL

- [ ] **Step 3: Implement wizard changes**

Update `newWizardModel`:
- `inputs := make([]textinput.Model, 6)` with password at 4 (default `postgres`), memory at 5 (`512M`)
- `selectedVersionIdx` = index of `DefaultPostgresVersion` in `PostgresVersions`
- Remove volume placeholder input

Update `newEditWizardModel` preload (Task 3 will finish Version preload; for create path ensure defaults work):
- Map old password/memory from instance into inputs 4/5
- Do **not** load volume into an input

Update navigation, `cycleOption`, view rows (Version cycle UI like Engine/Runtime; Volume preview using `MutedStyle` / non-active), and `saveInstance`.

Preview row helper idea:

```go
func (m *AppModel) wizardPreviewRow(inner int, label, value string) string {
	return lipgloss.NewStyle().Width(inner).Background(BgSurface).Render(
		LabelStyle.Render(label) + " " + MutedStyle.Render(value),
	)
}
```

(Use existing muted/dim style if one already exists; otherwise `ValueStyle` with lower emphasis.)

- [ ] **Step 4: Run full app + core tests**

Run: `go test ./internal/app/ ./internal/core/ -v`  
Expected: PASS (fix any remapped test fallout in this task)

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add internal/app/view_wizard.go internal/app/view_wizard_test.go
git commit -m "feat(tui): Postgres version step and automatic volume preview"
```

---

### Task 3: Edit preload + volume-change warning + details panel

**Files:**
- Modify: `internal/app/view_wizard.go` (`newEditWizardModel`, save success status)
- Modify: `internal/app/view_wizard_test.go`
- Modify: `internal/app/helpers.go` (details fields)
- Test: add/adjust helpers tests if any assert field labels; else cover via wizard/edit tests

**Interfaces:**
- Consumes: Task 1–2 APIs
- Produces:
  - Edit preload: `selectedVersionIdx` from `NormalizePostgresVersion(inst.Version)`
  - On successful edit save, if `oldVolume != derivedVolume()`: set English status including both names (and still arm restart confirm when `wasRunning`)
  - Details panel includes `Version:` for postgres (value `inst.Version`)

Status copy (exact intent):

```text
Volume will change to pgdata_shop_18. Previous volume pgdata_shop_16 is kept until you Purge it.
```

When volume unchanged, keep existing success messages (`Instance updated` / restart prompt).

When volume changes **and** restart confirm is armed, prefer combining or sequencing: set the volume warning as `statusMsg` and still arm `confirmRestartAfterEdit` with the existing restart prompt text — if both cannot fit one line, use the restart prompt as primary status and append a short note, e.g. `Saved (volume → pgdata_shop_18; old volume kept). Restart container with new config? Press 'y' to confirm, 'n' to cancel`.

Pick **one** approach and use it consistently in code + tests:

**Chosen approach:** If volume changed and restart armed → combined restart status above. If volume changed and not running → volume-only warning. If volume unchanged → existing messages.

- [ ] **Step 1: Write failing tests**

```go
func TestEditWizardPreloadsPostgresVersion(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db",
		Password: "p", Volume: "pgdata_shop_16", MemoryLimit: "512M",
		Version: "16", EnvFilePath: "/tmp/shop.env",
	}
	w := newEditWizardModel("/tmp", "/tmp", nil, inst)
	if w.selectedVersion() != "16" {
		t.Fatalf("version=%q", w.selectedVersion())
	}
}

func TestEditSaveWarnsWhenVolumeChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	old := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_VERSION=16
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_shop_16
`
	if err := os.WriteFile(oldPath, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := core.ParseEnvFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.wizard = newEditWizardModel(dir, dir, []*core.DatabaseInstance{inst}, inst)
	m.mode = ModeWizard
	// bump version to 18
	for i, v := range core.PostgresVersions {
		if v == "18" {
			m.wizard.selectedVersionIdx = i
			break
		}
	}
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview
	_, cmd := m.updateWizard(tea.KeyMsg{Type: tea.KeyEnter})
	_ = cmd
	if !strings.Contains(m.statusMsg, "pgdata_shop_18") || !strings.Contains(m.statusMsg, "pgdata_shop_16") {
		t.Fatalf("expected volume change warning, got %q", m.statusMsg)
	}
}
```

Details: assert `Version:` appears for postgres in `buildDetailRows` / rendered details (whatever helper `helpers.go` exposes — follow existing test patterns; if none, add a small unit test that calls the row builder and checks for `Version:` + `16`).

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/app/ -run "TestEditWizardPreloadsPostgresVersion|TestEditSaveWarnsWhenVolumeChanges" -v`

- [ ] **Step 3: Implement preload, status messages, details Version row**

In `helpers.go` fields list, after Engine (or before Volume):

```go
if inst.EngineType == "postgres" {
	// insert Version field — either build fields dynamically or append conditionally
}
```

Prefer building the slice dynamically so SQL Server omits Version.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ ./internal/core/ -v`  
Expected: PASS

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add internal/app/view_wizard.go internal/app/view_wizard_test.go internal/app/helpers.go
git commit -m "feat(tui): edit version preload, volume-change warning, details Version"
```

---

### Task 4: README + regression sweep

**Files:**
- Modify: `README.md`
- Touch any remaining broken tests from volume/input remap

**Interfaces:**
- Produces: README bullet noting Postgres version selection (14–18) and auto-named volumes including version suffix

- [ ] **Step 1: Update README features / wizard description**

Add under features (English):

```markdown
- 📦 **Postgres Version Selection:** Choose majors 14–18 in the create/edit wizard; volumes are auto-named (`pgdata_<name>_<version>`).
```

- [ ] **Step 2: Full test suite**

Run: `go test ./... -v`  
Expected: PASS  
Fix any remaining failures from step remumbering or string assertions (`StepVolume`, `POSTGRES_VOLUME=pgdata_shop` without version, etc.).

- [ ] **Step 3: Manual smoke checklist** (document in PR/status; do not block commit)

1. Create Postgres v16 → inspect `.env` for `POSTGRES_VERSION=16` and `POSTGRES_VOLUME=pgdata_<name>_16`
2. Edit → 18 → status warns; new volume name; old volume not deleted
3. Create SQL Server → no Version step; `SQLSERVER_VOLUME=sqlserver_<name>`
4. Start a legacy `.env` without `POSTGRES_VERSION` → still pulls `postgres:18` via compose default

- [ ] **Step 4: Commit** (only if user asked)

```bash
git add README.md internal/app/
git commit -m "docs: note Postgres version selection and auto volumes"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| Curated versions 14–18, default 18 | 1, 2 |
| Create + Edit selection | 2, 3 |
| `POSTGRES_VERSION` in `.env` | 1, 2 |
| Compose `postgres:${POSTGRES_VERSION}` (+ default) | 1 |
| Auto volume naming + read-only preview | 2 |
| SQL Server auto volume without version | 1, 2 |
| Volume-change warning; no migration | 3 |
| Details `Version:` | 3 |
| Legacy missing version → 18 | 1 |
| Purge semantics unchanged | (no code change; verified by existing purge tests) |
| README | 4 |

## Self-review notes

- No TBD placeholders.
- Input remap documented once in File map; Task 2 owns mechanical test updates.
- Compose uses `${POSTGRES_VERSION:-18}` so Start works before the next Edit Save rewrites the `.env`.
- Combined restart + volume-change status approach is explicit in Task 3.
