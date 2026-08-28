package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// CheckEngineHealth checks if the container runtime daemon (Docker or Podman) is active.
func (r *Runner) CheckEngineHealth(ctx context.Context, runtimeName string) EngineHealth {
	bin := runtimeName
	if bin == "" {
		bin = "docker"
	}

	if _, err := exec.LookPath(bin); err != nil {
		return EngineNotInstalled
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
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
	binary := inst.Runtime
	if binary == "" {
		binary = "docker"
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

// CheckStatus checks if the container is running and whether the DB port is ready to accept connections.
func (r *Runner) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	bin := "docker"
	if inst.Runtime == "podman" {
		bin = "podman"
	}

	cmd := exec.CommandContext(cmdCtx, bin, "ps", "--filter", fmt.Sprintf("name=^/%s$|^%s$", inst.ContainerName, inst.ContainerName), "--format", "{{.State}}")
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		// Fallback to compose ps
		bin, args := r.BuildComposeArgs(inst, "ps", "--format", "{{.State}}")
		cmd = exec.CommandContext(cmdCtx, bin, args...)
		cmd.Stdout = &outBuf
		if err := cmd.Run(); err != nil {
			return StatusStopped
		}
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
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
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

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
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
