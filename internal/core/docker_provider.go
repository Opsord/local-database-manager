package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DockerProvider implements RuntimeProvider for the Docker runtime engine.
type DockerProvider struct {
	projectRoot string
}

// NewDockerProvider creates a new DockerProvider configured for projectRoot.
func NewDockerProvider(projectRoot string) *DockerProvider {
	return &DockerProvider{projectRoot: projectRoot}
}

// Name returns the provider name.
func (p *DockerProvider) Name() string {
	return "docker"
}

// Health checks if Docker is installed and running.
func (p *DockerProvider) Health(ctx context.Context) EngineHealth {
	if _, err := exec.LookPath("docker"); err != nil {
		return EngineNotInstalled
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return EngineOffline
	}
	return EngineOnline
}

// StartDaemon starts Docker Desktop if offline.
func (p *DockerProvider) StartDaemon(ctx context.Context) error {
	health := p.Health(ctx)
	if health == EngineOnline {
		return nil
	}
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: docker", ErrEngineNotInstalled)
	}
	slog.Info("Starting Docker daemon")
	return startDockerDaemon(ctx, p.Health)
}

// StopDaemon stops Docker Desktop if online.
func (p *DockerProvider) StopDaemon(ctx context.Context) error {
	health := p.Health(ctx)
	if health == EngineOffline {
		return nil
	}
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: docker", ErrEngineNotInstalled)
	}
	slog.Info("Stopping Docker daemon")
	return stopDockerDaemon(ctx, p.Health)
}

// GetComposeFile resolves the docker-compose.yml path.
func (p *DockerProvider) GetComposeFile(inst *DatabaseInstance) string {
	engineFolder := inst.EngineType
	if engineFolder == "sqlserver" {
		engineFolder = "sql-server"
	}
	return filepath.Join(p.projectRoot, "engines", engineFolder, "docker-compose.yml")
}

// BuildComposeArgs returns the binary and args for a compose command.
func (p *DockerProvider) BuildComposeArgs(inst *DatabaseInstance, extraArgs ...string) (string, []string) {
	composeFile := p.GetComposeFile(inst)
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
	return "docker", args
}

// Start launches the container via docker compose up -d.
func (p *DockerProvider) Start(ctx context.Context, inst *DatabaseInstance) error {
	health := p.Health(ctx)
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: docker", ErrEngineNotInstalled)
	}
	if health == EngineOffline {
		return fmt.Errorf("%w: docker daemon is not running", ErrEngineOffline)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	slog.Info("Starting Docker instance", "name", inst.Name, "container", inst.ContainerName)

	bin, args := p.BuildComposeArgs(inst, "up", "-d")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to start Docker instance", "name", inst.Name, "error", err, "stderr", errBuf.String())
		return ParseComposeError(errBuf.String(), err)
	}

	inst.Status = StatusStarting
	return nil
}

// Stop stops the container via docker compose down.
func (p *DockerProvider) Stop(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	slog.Info("Stopping Docker instance", "name", inst.Name, "container", inst.ContainerName)

	bin, args := p.BuildComposeArgs(inst, "down")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to stop Docker instance", "name", inst.Name, "error", err, "stderr", errBuf.String())
		return ParseComposeError(errBuf.String(), err)
	}

	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// DownVolumes stops the container and removes volumes via docker compose down -v.
func (p *DockerProvider) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	slog.Info("Purging Docker instance volumes", "name", inst.Name, "container", inst.ContainerName)

	bin, args := p.BuildComposeArgs(inst, "down", "-v")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to purge Docker instance", "name", inst.Name, "error", err, "stderr", errBuf.String())
		return ParseComposeError(errBuf.String(), err)
	}

	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// CheckStatus inspects the container state and checks port connectivity.
func (p *DockerProvider) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "docker", "ps", "--filter", fmt.Sprintf("name=^/%s$|^%s$", inst.ContainerName, inst.ContainerName), "--format", "{{.State}}")
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		bin, args := p.BuildComposeArgs(inst, "ps", "--format", "{{.State}}")
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

// GetMemoryUsage gets container memory stats.
func (p *DockerProvider) GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string {
	if inst.Status == StatusStopped {
		return "-"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "docker", "stats", "--no-stream", "--format", "{{.MemUsage}}", inst.ContainerName)
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

// LogsCommand returns the command for streaming container logs.
func (p *DockerProvider) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	bin, args := p.BuildComposeArgs(inst, "logs", "--tail=100", "-f")
	return exec.Command(bin, args...)
}
