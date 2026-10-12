package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PodmanProvider implements RuntimeProvider for the Podman runtime engine.
type PodmanProvider struct {
	projectRoot string
}

// NewPodmanProvider creates a new PodmanProvider configured for projectRoot.
func NewPodmanProvider(projectRoot string) *PodmanProvider {
	return &PodmanProvider{projectRoot: projectRoot}
}

// Name returns the provider name.
func (p *PodmanProvider) Name() string {
	return "podman"
}

// Health checks if Podman is installed and running.
func (p *PodmanProvider) Health(ctx context.Context) EngineHealth {
	if _, err := exec.LookPath("podman"); err != nil {
		return EngineNotInstalled
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "podman", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return EngineOffline
	}
	return EngineOnline
}

// StartDaemon starts the Podman machine if offline.
func (p *PodmanProvider) StartDaemon(ctx context.Context) error {
	health := p.Health(ctx)
	if health == EngineOnline {
		return nil
	}
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: podman", ErrEngineNotInstalled)
	}

	slog.Info("Starting Podman machine")
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	if p.repairPodmanDefaultConnection(cmdCtx) {
		return nil
	}

	stderr, err := p.execPodmanMachine(cmdCtx, "start")
	if err != nil {
		if p.Health(cmdCtx) == EngineOnline {
			return nil
		}
		if isPodmanMachineAlreadyRunning(stderr) {
			_, _ = p.execPodmanMachine(cmdCtx, "stop")
			if p.Health(cmdCtx) == EngineOnline {
				return nil
			}
			stderr, err = p.execPodmanMachine(cmdCtx, "start")
			if err != nil {
				if p.Health(cmdCtx) == EngineOnline {
					return nil
				}
				return fmt.Errorf("%w: podman machine restart after stuck state: %v (%s)", ErrEngineStartFailed, err, stderr)
			}
		} else {
			return fmt.Errorf("%w: podman machine start: %v (%s)", ErrEngineStartFailed, err, stderr)
		}
	}

	if !p.repairPodmanDefaultConnection(cmdCtx) {
		waitErr := waitUntilEngineOnline(cmdCtx, p.Health, "podman")
		if waitErr != nil && !p.repairPodmanDefaultConnection(cmdCtx) {
			return waitErr
		}
	}
	return nil
}

// StopDaemon stops the Podman machine if online.
func (p *PodmanProvider) StopDaemon(ctx context.Context) error {
	health := p.Health(ctx)
	if health == EngineOffline {
		return nil
	}
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: podman", ErrEngineNotInstalled)
	}

	slog.Info("Stopping Podman machine")
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "podman", "machine", "stop")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if p.Health(context.Background()) != EngineOnline {
			return nil
		}
		return fmt.Errorf("%w: podman machine stop: %v (%s)", ErrEngineStartFailed, err, strings.TrimSpace(stderr.String()))
	}
	return waitUntilEngineOffline(cmdCtx, p.Health, "podman")
}

// GetComposeFile resolves the podman-compose.yml path.
func (p *PodmanProvider) GetComposeFile(inst *DatabaseInstance) string {
	engineFolder := inst.EngineType
	if engineFolder == "sqlserver" {
		engineFolder = "sql-server"
	}
	return filepath.Join(p.projectRoot, "engines", engineFolder, "podman-compose.yml")
}

// BuildComposeArgs returns the binary and args for a compose command.
func (p *PodmanProvider) BuildComposeArgs(inst *DatabaseInstance, extraArgs ...string) (string, []string) {
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
	return "podman", args
}

// Start launches the database container via rootless podman run.
func (p *PodmanProvider) Start(ctx context.Context, inst *DatabaseInstance) error {
	health := p.Health(ctx)
	if health == EngineNotInstalled {
		return fmt.Errorf("%w: podman", ErrEngineNotInstalled)
	}
	if health == EngineOffline {
		return fmt.Errorf("%w: podman daemon is not running", ErrEngineOffline)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	slog.Info("Starting Podman instance", "name", inst.Name, "container", inst.ContainerName)

	p.removeStaleRootfulContainer(cmdCtx, inst.ContainerName)

	args, err := p.buildPodmanRunArgs(inst)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(cmdCtx, "podman", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		slog.Error("Failed to start Podman instance", "name", inst.Name, "error", err, "stderr", errBuf.String())
		return ParseComposeError(errBuf.String(), err)
	}

	inst.Status = StatusStarting
	return nil
}

// Stop stops and removes the container via podman rm -f.
func (p *PodmanProvider) Stop(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	slog.Info("Stopping Podman instance", "name", inst.Name, "container", inst.ContainerName)

	if err := p.stopPodmanContainer(cmdCtx, inst); err != nil {
		slog.Error("Failed to stop Podman instance", "name", inst.Name, "error", err)
		return err
	}

	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// DownVolumes stops the container and removes volumes via podman volume rm -f.
func (p *PodmanProvider) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	slog.Info("Purging Podman instance volumes", "name", inst.Name, "container", inst.ContainerName)

	if err := p.purgePodmanContainer(cmdCtx, inst); err != nil {
		slog.Error("Failed to purge Podman instance", "name", inst.Name, "error", err)
		return err
	}

	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// CheckStatus checks if the container is running and if the port is open.
func (p *PodmanProvider) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "podman", "ps", "--filter", fmt.Sprintf("name=^/%s$|^%s$", inst.ContainerName, inst.ContainerName), "--format", "{{.State}}")
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

// GetMemoryUsage returns container memory usage stats.
func (p *PodmanProvider) GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string {
	if inst.Status == StatusStopped {
		return "-"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "podman", "stats", "--no-stream", "--format", "{{.MemUsage}}", inst.ContainerName)
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
func (p *PodmanProvider) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	return exec.Command("podman", "logs", "--tail=100", "-f", inst.ContainerName)
}

// buildPodmanRunArgs builds `podman run` args for an instance.
func (p *PodmanProvider) buildPodmanRunArgs(inst *DatabaseInstance) ([]string, error) {
	name := strings.TrimSpace(inst.ContainerName)
	if name == "" {
		return nil, fmt.Errorf("container name is required")
	}
	if inst.Port <= 0 {
		return nil, fmt.Errorf("host port is required")
	}

	args := []string{
		"run", "-d", "--replace",
		"--name", name,
		"--restart", "unless-stopped",
	}
	if mem := strings.TrimSpace(inst.MemoryLimit); mem != "" {
		args = append(args, "--memory", mem)
	}

	switch inst.EngineType {
	case "sqlserver":
		vol := strings.TrimSpace(inst.Volume)
		if vol == "" {
			vol = "mssqldata"
		}
		engineDir := filepath.Join(p.projectRoot, "engines", "sql-server")
		args = append(args,
			"-e", "ACCEPT_EULA=Y",
			"-e", "MSSQL_PID=Developer",
			"-e", "MSSQL_SA_PASSWORD="+inst.Password,
			"-e", "SA_PASSWORD="+inst.Password,
			"-e", "SQLSERVER_DB="+inst.Database,
			"-e", "SQLSERVER_SCHEMA="+inst.Schema,
			"-p", fmt.Sprintf("127.0.0.1:%d:1433", inst.Port),
			"-v", vol+":/var/opt/mssql",
			"-v", filepath.Join(engineDir, "init")+":/init:ro",
			"-v", filepath.Join(engineDir, "docker-entrypoint.sh")+":/entrypoint.sh:ro",
			"--entrypoint", "/bin/bash",
			"mcr.microsoft.com/mssql/server:2022-latest",
			"/entrypoint.sh",
		)
		if inst.RawEnv != nil {
			if mb := strings.TrimSpace(inst.RawEnv["MSSQL_MEMORY_LIMIT_MB"]); mb != "" {
				args = insertEnvArg(args, "MSSQL_MEMORY_LIMIT_MB="+mb)
			}
		}
	default: // postgres
		version := strings.TrimSpace(inst.Version)
		if version == "" {
			version = "18"
		}
		vol := strings.TrimSpace(inst.Volume)
		if vol == "" {
			vol = "pgdata"
		}
		initDir := filepath.Join(p.projectRoot, "engines", "postgres", "init")
		args = append(args,
			"-e", "POSTGRES_DB="+inst.Database,
			"-e", "POSTGRES_USER="+inst.User,
			"-e", "POSTGRES_PASSWORD="+inst.Password,
			"-e", "POSTGRES_SCHEMA="+inst.Schema,
			"-p", fmt.Sprintf("127.0.0.1:%d:5432", inst.Port),
			"-v", vol+":/var/lib/postgresql",
			"-v", initDir+":/docker-entrypoint-initdb.d:ro",
			"docker.io/library/postgres:"+version,
		)
	}
	return args, nil
}

func (p *PodmanProvider) stopPodmanContainer(ctx context.Context, inst *DatabaseInstance) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", inst.ContainerName)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		lower := strings.ToLower(detail + " " + err.Error())
		if strings.Contains(lower, "no such container") || strings.Contains(lower, "not found") {
			return nil
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("failed to stop container: %s (%w)", detail, err)
	}
	return nil
}

func (p *PodmanProvider) purgePodmanContainer(ctx context.Context, inst *DatabaseInstance) error {
	if err := p.stopPodmanContainer(ctx, inst); err != nil {
		return fmt.Errorf("failed to purge container: %w", err)
	}
	vol := strings.TrimSpace(inst.Volume)
	if vol == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "podman", "volume", "rm", "-f", vol)
	_ = cmd.Run() // volume may already be gone
	return nil
}

func (p *PodmanProvider) removeStaleRootfulContainer(ctx context.Context, name string) {
	if runtime.GOOS != "windows" || strings.TrimSpace(name) == "" {
		return
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return
	}
	host, err := p.podmanMachineDOCKERHost(ctx)
	if err != nil || host == "" {
		return
	}
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+host)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func (p *PodmanProvider) execPodmanMachine(ctx context.Context, subcmd string) (stderr string, err error) {
	cmd := exec.CommandContext(ctx, "podman", "machine", subcmd)
	var errBuf bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(errBuf.String()), err
}

type podmanSystemConnection struct {
	Name    string `json:"Name"`
	URI     string `json:"URI"`
	Default bool   `json:"Default"`
}

func pickPodmanConnectionToDefault(conns []podmanSystemConnection, working map[string]bool) string {
	for _, c := range conns {
		if !working[c.Name] {
			continue
		}
		if c.Default {
			return ""
		}
		return c.Name
	}
	return ""
}

func (p *PodmanProvider) listPodmanConnections(ctx context.Context) ([]podmanSystemConnection, error) {
	cmd := exec.CommandContext(ctx, "podman", "system", "connection", "list", "--format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("podman system connection list: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	var conns []podmanSystemConnection
	if err := json.Unmarshal(stdout.Bytes(), &conns); err != nil {
		return nil, fmt.Errorf("parse podman connections: %w", err)
	}
	return conns, nil
}

func (p *PodmanProvider) podmanInfoOK(ctx context.Context, connection string) bool {
	args := []string{"info"}
	if connection != "" {
		args = []string{"--connection", connection, "info"}
	}
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func (p *PodmanProvider) setPodmanDefaultConnection(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "podman", "system", "connection", "default", name)
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman system connection default %s: %v (%s)", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (p *PodmanProvider) repairPodmanDefaultConnection(ctx context.Context) bool {
	if p.Health(ctx) == EngineOnline {
		return true
	}
	conns, err := p.listPodmanConnections(ctx)
	if err != nil || len(conns) == 0 {
		return false
	}
	working := make(map[string]bool, len(conns))
	for _, c := range conns {
		working[c.Name] = p.podmanInfoOK(ctx, c.Name)
	}
	name := pickPodmanConnectionToDefault(conns, working)
	if name == "" {
		for _, c := range conns {
			if c.Default && working[c.Name] {
				return true
			}
		}
		return false
	}
	if err := p.setPodmanDefaultConnection(ctx, name); err != nil {
		return false
	}
	return p.Health(ctx) == EngineOnline
}

type podmanMachineInspect struct {
	ConnectionInfo struct {
		PodmanPipe *struct {
			Path string `json:"Path"`
		} `json:"PodmanPipe"`
	} `json:"ConnectionInfo"`
}

func (p *PodmanProvider) podmanMachineDOCKERHost(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", "machine", "inspect")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("podman machine inspect: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	var machines []podmanMachineInspect
	if err := json.Unmarshal(stdout.Bytes(), &machines); err != nil {
		return "", fmt.Errorf("parse podman machine inspect: %w", err)
	}
	if len(machines) == 0 || machines[0].ConnectionInfo.PodmanPipe == nil {
		return "", fmt.Errorf("podman machine inspect: missing PodmanPipe")
	}
	host := windowsNamedPipeToDOCKERHost(machines[0].ConnectionInfo.PodmanPipe.Path)
	if host == "" {
		return "", fmt.Errorf("podman machine inspect: invalid PodmanPipe path %q", machines[0].ConnectionInfo.PodmanPipe.Path)
	}
	return host, nil
}

func (p *PodmanProvider) probeDOCKERHost(ctx context.Context, dockerHost string) bool {
	bin, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	cmd := exec.CommandContext(ctx, bin, "version", "--format", "{{.Server.APIVersion}}")
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func (p *PodmanProvider) podmanComposeAPIReady(ctx context.Context) bool {
	if runtime.GOOS != "windows" {
		return true
	}
	host, err := p.podmanMachineDOCKERHost(ctx)
	if err != nil {
		return false
	}
	return p.probeDOCKERHost(ctx, host)
}

func (p *PodmanProvider) ensurePodmanComposeReady(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}
	if p.podmanComposeAPIReady(ctx) {
		return nil
	}
	_, _ = p.execPodmanMachine(ctx, "stop")
	stderr, err := p.execPodmanMachine(ctx, "start")
	if err != nil && !isPodmanMachineAlreadyRunning(stderr) {
		if p.Health(ctx) != EngineOnline {
			return fmt.Errorf("%w: refresh podman machine for compose API: %v (%s)", ErrEngineStartFailed, err, stderr)
		}
	}
	_ = p.repairPodmanDefaultConnection(ctx)

	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if p.podmanComposeAPIReady(ctx) {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: podman Docker API pipe not ready (compose needs npipe forwarding)", ErrEngineStartFailed)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: podman Docker API pipe not ready (compose needs npipe forwarding)", ErrEngineStartFailed)
		case <-time.After(enginePollInterval):
		}
	}
}

func isPodmanMachineAlreadyRunning(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "already running")
}

func isPodmanComposePipeError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "pipe") && (strings.Contains(lower, "eof") || strings.Contains(lower, "error during connect"))
}

func windowsNamedPipeToDOCKERHost(pipePath string) string {
	p := strings.TrimSpace(pipePath)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "/", `\`)
	lower := strings.ToLower(p)
	const prefix = `\\.\pipe\`
	if !strings.HasPrefix(lower, prefix) {
		return ""
	}
	name := p[len(prefix):]
	if name == "" {
		return ""
	}
	return "npipe:////./pipe/" + name
}

func insertEnvArg(args []string, envKV string) []string {
	out := make([]string, 0, len(args)+2)
	inserted := false
	for _, a := range args {
		if !inserted && (strings.HasPrefix(a, "docker.io/") || strings.HasPrefix(a, "mcr.microsoft.com/")) {
			out = append(out, "-e", envKV)
			inserted = true
		}
		out = append(out, a)
	}
	if !inserted {
		out = append(out, "-e", envKV)
	}
	return out
}
