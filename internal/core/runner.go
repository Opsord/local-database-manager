package core

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
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

const (
	engineStartTimeout = 90 * time.Second
	enginePollInterval = 2 * time.Second
)

// InstanceRunner defines the interface for database lifecycle operations.
type InstanceRunner interface {
	CheckEngineHealth(ctx context.Context, runtimeName string) EngineHealth
	StartEngine(ctx context.Context, runtimeName string) error
	StopEngine(ctx context.Context, runtimeName string) error
	Start(ctx context.Context, inst *DatabaseInstance) error
	Stop(ctx context.Context, inst *DatabaseInstance) error
	DownVolumes(ctx context.Context, inst *DatabaseInstance) error
	CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus
	GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string
	LogsCommand(inst *DatabaseInstance) *exec.Cmd
}

// Runner manages starting, stopping, and inspecting database containers by delegating
// to modular RuntimeProvider implementations.
type Runner struct {
	ProjectRoot string
	providers   map[string]RuntimeProvider
}

// NewRunner creates a new Runner configured for the specified project root.
func NewRunner(projectRoot string) *Runner {
	EnsureScriptLineEndings(projectRoot)
	r := &Runner{
		ProjectRoot: projectRoot,
		providers:   make(map[string]RuntimeProvider),
	}
	r.providers["docker"] = NewDockerProvider(projectRoot)
	r.providers["podman"] = NewPodmanProvider(projectRoot)
	return r
}

// EnsureScriptLineEndings ensures all shell scripts under engines/ use LF line endings
// so they can execute cleanly inside Linux containers without carriage return errors.
func EnsureScriptLineEndings(projectRoot string) {
	enginesDir := filepath.Join(projectRoot, "engines")
	_ = filepath.Walk(enginesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".sh") {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			if bytes.Contains(data, []byte("\r\n")) {
				sanitized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
				_ = os.WriteFile(path, sanitized, info.Mode())
			}
		}
		return nil
	})
}

func (r *Runner) providerFor(runtimeName string) RuntimeProvider {
	name := strings.TrimSpace(runtimeName)
	if name == "" {
		name = "docker"
	}
	if p, ok := r.providers[name]; ok {
		return p
	}
	return newUnsupportedProvider(name)
}

// CheckEngineHealth checks if the container runtime daemon is active.
func (r *Runner) CheckEngineHealth(ctx context.Context, runtimeName string) EngineHealth {
	return r.providerFor(runtimeName).Health(ctx)
}

// StartEngine attempts to start the container runtime daemon when it is offline.
func (r *Runner) StartEngine(ctx context.Context, runtimeName string) error {
	return r.providerFor(runtimeName).StartDaemon(ctx)
}

// StopEngine attempts to stop the container runtime daemon when it is online.
func (r *Runner) StopEngine(ctx context.Context, runtimeName string) error {
	return r.providerFor(runtimeName).StopDaemon(ctx)
}

// Start launches the database container in detached mode.
func (r *Runner) Start(ctx context.Context, inst *DatabaseInstance) error {
	return r.providerFor(inst.Runtime).Start(ctx, inst)
}

// Stop halts the database container.
func (r *Runner) Stop(ctx context.Context, inst *DatabaseInstance) error {
	return r.providerFor(inst.Runtime).Stop(ctx, inst)
}

// DownVolumes stops and deletes container volumes.
func (r *Runner) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	return r.providerFor(inst.Runtime).DownVolumes(ctx, inst)
}

// CheckStatus checks if the container is running and whether the DB port is ready.
func (r *Runner) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	return r.providerFor(inst.Runtime).CheckStatus(ctx, inst)
}

// GetMemoryUsage retrieves memory consumption stats for the container.
func (r *Runner) GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string {
	return r.providerFor(inst.Runtime).GetMemoryUsage(ctx, inst)
}

// LogsCommand prepares a command for streaming container logs.
func (r *Runner) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	return r.providerFor(inst.Runtime).LogsCommand(inst)
}

// GetComposeFile resolves the appropriate compose file path for an instance.
func (r *Runner) GetComposeFile(inst *DatabaseInstance) string {
	if p, ok := r.providerFor(inst.Runtime).(*DockerProvider); ok {
		return p.GetComposeFile(inst)
	}
	if p, ok := r.providerFor(inst.Runtime).(*PodmanProvider); ok {
		return p.GetComposeFile(inst)
	}
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
	if p, ok := r.providerFor(inst.Runtime).(*DockerProvider); ok {
		return p.BuildComposeArgs(inst, extraArgs...)
	}
	if p, ok := r.providerFor(inst.Runtime).(*PodmanProvider); ok {
		return p.BuildComposeArgs(inst, extraArgs...)
	}
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

func (r *Runner) buildPodmanRunArgs(inst *DatabaseInstance) ([]string, error) {
	if p, ok := r.providerFor("podman").(*PodmanProvider); ok {
		return p.buildPodmanRunArgs(inst)
	}
	return nil, fmt.Errorf("podman provider not available")
}

func (r *Runner) startDockerEngine(ctx context.Context) error {
	return r.providerFor("docker").StartDaemon(ctx)
}

func (r *Runner) stopDockerEngine(ctx context.Context) error {
	return r.providerFor("docker").StopDaemon(ctx)
}

func (r *Runner) waitUntilOnline(ctx context.Context, runtimeName string) error {
	return waitUntilEngineOnline(ctx, func(c context.Context) EngineHealth {
		return r.CheckEngineHealth(c, runtimeName)
	}, runtimeName)
}

func (r *Runner) waitUntilOffline(ctx context.Context, runtimeName string) error {
	return waitUntilEngineOffline(ctx, func(c context.Context) EngineHealth {
		return r.CheckEngineHealth(c, runtimeName)
	}, runtimeName)
}

func isTCPPortReady(port int) bool {
	addrs := []string{
		fmt.Sprintf("127.0.0.1:%d", port),
		fmt.Sprintf("[::1]:%d", port),
	}
	for _, addr := range addrs {
		conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err != nil {
			continue
		}
		_ = conn.Close()
		return true
	}
	return false
}
