package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// EngineHealth represents the status of the container runtime daemon.
type EngineHealth string

const (
	EngineOnline       EngineHealth = "ONLINE"
	EngineOffline      EngineHealth = "OFFLINE"
	EngineNotInstalled EngineHealth = "NOT_INSTALLED"
)

// Runner manages starting, stopping, and inspecting database containers.
type Runner struct {
	ProjectRoot string
}

// NewRunner creates a new Runner configured for the specified project root.
func NewRunner(projectRoot string) *Runner {
	return &Runner{ProjectRoot: projectRoot}
}

// CheckEngineHealth checks if the container runtime daemon (Docker or Podman) is active.
func (r *Runner) CheckEngineHealth(runtimeName string) EngineHealth {
	bin := "docker"
	if runtimeName == "podman" {
		bin = "podman"
	}

	if _, err := exec.LookPath(bin); err != nil {
		return EngineNotInstalled
	}

	cmd := exec.Command(bin, "info")
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
		podmanFile := filepath.Join(r.ProjectRoot, "engines", engineFolder, "podman-compose.yml")
		return podmanFile
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
func (r *Runner) Start(inst *DatabaseInstance) error {
	health := r.CheckEngineHealth(inst.Runtime)
	if health == EngineNotInstalled {
		return fmt.Errorf("%s is not installed or not found in PATH", inst.Runtime)
	}
	if health == EngineOffline {
		if inst.Runtime == "podman" {
			return fmt.Errorf("Podman service is not running (run 'podman machine start')")
		}
		return fmt.Errorf("Docker engine is not running (start Docker Desktop or the docker service)")
	}

	bin, args := r.BuildComposeArgs(inst, "up", "-d")
	cmd := exec.Command(bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusRunning
	return nil
}

// Stop halts the container (down).
func (r *Runner) Stop(inst *DatabaseInstance) error {
	bin, args := r.BuildComposeArgs(inst, "down")
	cmd := exec.Command(bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusStopped
	return nil
}

// DownVolumes stops and deletes container volumes (down -v).
func (r *Runner) DownVolumes(inst *DatabaseInstance) error {
	bin, args := r.BuildComposeArgs(inst, "down", "-v")
	cmd := exec.Command(bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to purge container: %s (%w)", strings.TrimSpace(errBuf.String()), err)
	}
	inst.Status = StatusStopped
	return nil
}

// CheckStatus checks if the container is currently running.
func (r *Runner) CheckStatus(inst *DatabaseInstance) ContainerStatus {
	bin, args := r.BuildComposeArgs(inst, "ps", "--format", "{{.State}}")
	cmd := exec.Command(bin, args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		return StatusStopped
	}

	output := strings.ToLower(strings.TrimSpace(outBuf.String()))
	if strings.Contains(output, "running") || strings.Contains(output, "up") {
		return StatusRunning
	}
	return StatusStopped
}

// LogsCommand prepares a command for streaming container logs.
func (r *Runner) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	bin, args := r.BuildComposeArgs(inst, "logs", "--tail=100", "-f")
	return exec.Command(bin, args...)
}
