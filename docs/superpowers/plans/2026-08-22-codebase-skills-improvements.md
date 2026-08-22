# Codebase Skills Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the database manager codebase by applying the project's specialized skills (`bash-defensive-patterns`, `golang-patterns`, and `golang-testing`): adding strict mode, signal trapping, and timeout bounds to shell entrypoints, context-aware command execution with sentinel errors in Go, and comprehensive fuzzing and benchmarks.

**Architecture:** 
1. **Engine Shell Layer (`bash-defensive-patterns`):** Add `set -Eeuo pipefail`, signal traps for graceful `SIGTERM` shutdown in container background processes, finite retry loops with timeout bounds, and required parameter assertions.
2. **Core Domain Layer (`golang-patterns`):** Define standard sentinel errors, decouple the runner with an `InstanceRunner` interface, and execute all external Docker/Podman processes using `exec.CommandContext` with bounded timeouts to prevent daemon hang deadlocks.
3. **Quality & Test Layer (`golang-testing`):** Implement `FuzzParseEnvFile` for malformed input resilience, table-driven subtests with `t.Parallel()` and `t.Helper()`, and performance benchmarks for scanning and port allocation.

**Tech Stack:** Go 1.22+, standard library (`context`, `os/exec`, `testing`, `errors`), Bash.

---

## Task 1: Defensive Bash Scripts (`bash-defensive-patterns`)

**Files:**
- Modify: `engines/sql-server/docker-entrypoint.sh`
- Modify: `engines/postgres/init/01-schema.sh`

**Interfaces:**
- Consumes: Environment variables (`SA_PASSWORD`, `SQLSERVER_DB`, `SQLSERVER_SCHEMA`, `POSTGRES_USER`, `POSTGRES_DB`, `POSTGRES_SCHEMA`).
- Produces: Resilient container initialization that cleanly forwards `SIGTERM` signals and fails fast on errors.

- [ ] **Step 1: Update `engines/postgres/init/01-schema.sh` with defensive strict mode and variable fallback**

```bash
#!/bin/bash
set -Eeuo pipefail

SCHEMA_NAME="${POSTGRES_SCHEMA:-public}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-postgres}"

psql -v ON_ERROR_STOP=1 --username "$DB_USER" --dbname "$DB_NAME" <<-EOSQL
    CREATE SCHEMA IF NOT EXISTS "$SCHEMA_NAME";
EOSQL
```

- [ ] **Step 2: Update `engines/sql-server/docker-entrypoint.sh` with strict mode, signal trap, finite retry timeout, and parameter validation**

```bash
#!/bin/bash
set -Eeuo pipefail

# Validate required variables
: "${SA_PASSWORD:?SA_PASSWORD environment variable is required}"
: "${SQLSERVER_DB:?SQLSERVER_DB environment variable is required}"

SA_PASS="${SA_PASSWORD}"
DB_NAME="${SQLSERVER_DB}"
SCHEMA_NAME="${SQLSERVER_SCHEMA:-dbo}"

# Start SQL Server in background
/opt/mssql/bin/sqlservr &
sqlservr_pid=$!

# Signal handler for graceful shutdown
cleanup() {
    echo "Caught shutdown signal. Forwarding SIGTERM to SQL Server (PID $sqlservr_pid)..."
    kill -TERM "$sqlservr_pid" 2>/dev/null || true
    wait "$sqlservr_pid" 2>/dev/null || true
}
trap cleanup SIGTERM SIGINT

# Locate sqlcmd binary safely
SQLCMD_BIN="$(command -v sqlcmd 2>/dev/null || ls /opt/mssql-tools*/bin/sqlcmd 2>/dev/null | head -n1 || true)"
if [[ -z "$SQLCMD_BIN" ]]; then
    echo "ERROR: sqlcmd binary not found. Initialization scripts cannot be executed." >&2
    wait "$sqlservr_pid"
    exit 1
fi
echo "Using sqlcmd at: $SQLCMD_BIN"

echo "Waiting for SQL Server to become available (max 60s)..."
MAX_ATTEMPTS=60
attempt=1
until "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C -Q "SELECT 1" >/dev/null 2>&1; do
    if (( attempt >= MAX_ATTEMPTS )); then
        echo "ERROR: SQL Server failed to become available after ${MAX_ATTEMPTS} seconds." >&2
        kill -TERM "$sqlservr_pid" 2>/dev/null || true
        wait "$sqlservr_pid" 2>/dev/null || true
        exit 1
    fi
    sleep 1
    ((attempt++))
done

echo "SQL Server is ready. Running initialization scripts..."
if [[ -d "/init" ]]; then
    for f in /init/*.sql; do
        [[ -e "$f" ]] || continue
        echo "Executing initialization script: $f"
        "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C \
            -v DB="$DB_NAME" SCHEMA="$SCHEMA_NAME" -i "$f" || echo "WARNING: Script $f returned non-zero exit status"
    done
fi

echo "Initialization complete. Server running."

# Wait for background sqlservr process
wait "$sqlservr_pid"
```

- [ ] **Step 3: Verify syntax of shell scripts**

Run: `git diff engines/`
Expected: Clean changes adhering to `bash-defensive-patterns`.

- [ ] **Step 4: Commit changes**

```bash
git add engines/sql-server/docker-entrypoint.sh engines/postgres/init/01-schema.sh
git commit -m "refactor(engines): apply defensive bash patterns, signal traps, and bounded timeouts"
```

---

## Task 2: Go Contexts, Decoupled Interface & Sentinel Errors (`golang-patterns`)

**Files:**
- Create: `internal/core/errors.go`
- Modify: `internal/core/runner.go`
- Modify: `internal/core/runner_test.go`
- Modify: `internal/app/tui.go`

**Interfaces:**
- Consumes: Standard Go `context.Context` and `errors`.
- Produces: Exported `InstanceRunner` interface, `ErrEngineOffline`, `ErrEngineNotInstalled`, and context-bounded runner methods.

- [ ] **Step 1: Create `internal/core/errors.go` with domain sentinel errors**

```go
package core

import "errors"

var (
	// ErrEngineOffline is returned when the container engine daemon is unreachable.
	ErrEngineOffline = errors.New("container engine daemon is offline")

	// ErrEngineNotInstalled is returned when docker/podman is not found in PATH.
	ErrEngineNotInstalled = errors.New("container engine not found in system PATH")

	// ErrPortInUse is returned when a port is not available.
	ErrPortInUse = errors.New("requested port is already in use")

	// ErrInstanceNotFound is returned when an instance cannot be located.
	ErrInstanceNotFound = errors.New("database instance not found")
)
```

- [ ] **Step 2: Update `internal/core/runner.go` with `InstanceRunner` interface and `context.Context` bounds**

```go
package core

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EngineHealth represents the status of the container runtime daemon.
type EngineHealth string

const (
	EngineOnline       EngineHealth = "ONLINE"
	EngineOffline      EngineHealth = "OFFLINE"
	EngineNotInstalled EngineHealth = "NOT_INSTALLED"
)

// InstanceRunner defines the interface for database lifecycle operations.
type InstanceRunner interface {
	CheckEngineHealth(ctx context.Context, runtimeName string) EngineHealth
	Start(ctx context.Context, inst *DatabaseInstance) error
	Stop(ctx context.Context, inst *DatabaseInstance) error
	DownVolumes(ctx context.Context, inst *DatabaseInstance) error
	CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus
	GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string
	LogsCommand(inst *DatabaseInstance) *exec.Cmd
}

// Runner manages starting, stopping, and inspecting database containers.
type Runner struct {
	ProjectRoot string
}

// NewRunner creates a new Runner configured for the specified project root.
func NewRunner(projectRoot string) *Runner {
	return &Runner{ProjectRoot: projectRoot}
}

// CheckEngineHealth checks if the container runtime daemon is active.
func (r *Runner) CheckEngineHealth(ctx context.Context, runtimeName string) EngineHealth {
	bin := "docker"
	if runtimeName == "podman" {
		bin = "podman"
	}

	if _, err := exec.LookPath(bin); err != nil {
		return EngineNotInstalled
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, "info")
	if err := cmd.Run(); err != nil {
		return EngineOffline
	}
	return EngineOnline
}

// GetComposeFile resolves the appropriate compose file path for an instance.
func (r *Runner) GetComposeFile(inst *DatabaseInstance) string {
	engineFolder := inst.EngineType
	if engineFolder == "sqlserver" {
		engineFolder = "sql-server"
	}

	if inst.Runtime == "podman" {
		return filepath.Join(r.ProjectRoot, "engines", engineFolder, "podman-compose.yml")
	}

	return filepath.Join(r.ProjectRoot, "engines", engineFolder, "docker-compose.yml")
}

// BuildComposeArgs returns the binary name and base command arguments.
func (r *Runner) BuildComposeArgs(inst *DatabaseInstance, extraArgs ...string) (string, []string) {
	binary := "docker"
	if inst.Runtime == "podman" {
		binary = "podman"
	}

	composeFile := r.GetComposeFile(inst)
	projectName := inst.ProjectName
	if projectName == "" {
		projectName = inst.Name
	}

	args := []string{
		"compose",
		"-p", projectName,
		"-f", composeFile,
		"--env-file", inst.EnvFilePath,
	}

	args = append(args, extraArgs...)
	return binary, args
}

// Start launches the container in detached mode (up -d).
func (r *Runner) Start(ctx context.Context, inst *DatabaseInstance) error {
	health := r.CheckEngineHealth(ctx, inst.Runtime)
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, inst.Runtime)
	}
	if health == EngineOffline {
		return fmt.Errorf("%w: %s daemon is not running", ErrEngineOffline, inst.Runtime)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	bin, args := r.BuildComposeArgs(inst, "up", "-d")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusStarting
	return nil
}

// Stop halts the container (down).
func (r *Runner) Stop(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	bin, args := r.BuildComposeArgs(inst, "down")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// DownVolumes stops and deletes container volumes (down -v).
func (r *Runner) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	bin, args := r.BuildComposeArgs(inst, "down", "-v")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to purge container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// CheckStatus checks if the container is running and whether the DB port is ready.
func (r *Runner) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	bin, args := r.BuildComposeArgs(inst, "ps", "--format", "{{.State}}")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		return StatusStopped
	}

	output := strings.ToLower(strings.TrimSpace(outBuf.String()))
	if strings.Contains(output, "running") || strings.Contains(output, "up") {
		if isTCPPortReady(inst.Port) {
			return StatusReady
		}
		return StatusStarting
	}
	return StatusStopped
}

func isTCPPortReady(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// GetMemoryUsage retrieves memory consumption stats for the container.
func (r *Runner) GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string {
	if inst.Status == StatusStopped {
		return "-"
	}

	bin := "docker"
	if inst.Runtime == "podman" {
		bin = "podman"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, "stats", "--no-stream", "--format", "{{.MemUsage}}", inst.ContainerName)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		return "-"
	}

	res := strings.TrimSpace(outBuf.String())
	if res == "" {
		return "-"
	}
	return res
}

// LogsCommand prepares a command for streaming container logs.
func (r *Runner) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	bin, args := r.BuildComposeArgs(inst, "logs", "--tail=100", "-f")
	return exec.Command(bin, args...)
}
```

- [ ] **Step 3: Update `internal/app/tui.go` and `internal/app/view_main.go` to pass `context.Background()` to runner calls**

- [ ] **Step 4: Run unit tests to verify `internal/core` passes**

Run: `go test ./internal/core -v`
Expected: PASS

- [ ] **Step 5: Commit changes**

```bash
git add internal/core/errors.go internal/core/runner.go internal/core/runner_test.go internal/app/
git commit -m "feat(core): introduce InstanceRunner interface, sentinel errors, and context timeouts"
```

---

## Task 3: Fuzzing, Subtests & Benchmarks (`golang-testing`)

**Files:**
- Create: `internal/core/instance_fuzz_test.go`
- Create: `internal/core/instance_bench_test.go`
- Modify: `internal/core/ports_test.go`
- Modify: `internal/core/scanner_test.go`

**Interfaces:**
- Consumes: Go testing framework (`testing.F`, `testing.B`, `t.Parallel()`, `t.Helper()`).
- Produces: Stress-tested parser and memory/speed benchmarks.

- [ ] **Step 1: Create `internal/core/instance_fuzz_test.go`**

```go
package core

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseEnvFile(f *testing.F) {
	// Seed corpus with valid and edge case configs
	f.Add([]byte("ENGINE=postgres\nPOSTGRES_PORT=5432\nPOSTGRES_DB=testdb\n"))
	f.Add([]byte("ENGINE=sqlserver\nSQLSERVER_PORT=1433\nSA_PASSWORD=Secret123!\n"))
	f.Add([]byte("# Comment only\nKEY=value # inline comment\nQUOTED=\"hello world\"\n"))
	f.Add([]byte("EMPTY=\nINVALID LINE WITHOUT EQUALS\n===EXTRA===\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		envPath := filepath.Join(tmpDir, "fuzz.env")
		if err := os.WriteFile(envPath, data, 0644); err != nil {
			return
		}

		// ParseEnvFile must never panic on arbitrary input
		inst, err := ParseEnvFile(envPath)
		if err == nil && inst != nil {
			_ = inst.ConnectionURI()
			_ = inst.CLICommand()
			_ = inst.BackendEnvBlock()
		}
	})
}
```

- [ ] **Step 2: Create `internal/core/instance_bench_test.go`**

```go
package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkParseEnvFile(b *testing.B) {
	tmpDir := b.TempDir()
	envPath := filepath.Join(tmpDir, "bench.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-bench
COMPOSE_PROJECT_NAME=pg-bench
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=bench_db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_bench
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write bench env: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParseEnvFile(envPath)
		if err != nil {
			b.Fatalf("ParseEnvFile failed: %v", err)
		}
	}
}

func BenchmarkFindNextFreePort(b *testing.B) {
	var instances []*DatabaseInstance
	for p := 5432; p < 5442; p++ {
		instances = append(instances, &DatabaseInstance{
			Port: p,
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = FindNextFreePort(5432, instances)
	}
}

func BenchmarkScanInstances(b *testing.B) {
	tmpDir := b.TempDir()
	for i := 0; i < 10; i++ {
		path := filepath.Join(tmpDir, "inst_"+strconv.Itoa(i)+".env")
		_ = os.WriteFile(path, []byte("ENGINE=postgres\nPOSTGRES_PORT=5432\n"), 0644)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = ScanInstances(tmpDir)
	}
}
```

- [ ] **Step 3: Update `ports_test.go` and `scanner_test.go` with `t.Parallel()` and `t.Helper()`**

- [ ] **Step 4: Run fuzzing and benchmarks**

Run: `go test -fuzz=FuzzParseEnvFile -fuzztime=5s ./internal/core`
Run: `go test -bench=. -benchmem ./internal/core`
Run: `go test -race ./...`
Expected: PASS with 0 allocations hotspots and 0 fuzz crashes.

- [ ] **Step 5: Commit changes**

```bash
git add internal/core/*_test.go
git commit -m "test(core): add fuzz tests, performance benchmarks, and parallel subtests"
```

---

## Task 4: Compilation, TUI Verification & Final Delivery

**Files:**
- Modify: `cmd/db-manager/main.go`
- Modify: `README.md`

- [ ] **Step 1: Run full test suite with race detector**

Run: `go test -race -v ./...`
Expected: ALL PASS

- [ ] **Step 2: Build binary `db-manager.exe`**

Run: `go build -o db-manager.exe ./cmd/db-manager`
Expected: Clean compilation with 0 warnings.

- [ ] **Step 3: Commit and push all changes**

```bash
git add .
git commit -m "chore: finalize skills audit improvements and rebuild binary"
git push origin main
```
