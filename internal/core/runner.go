package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	engineStartTimeout  = 90 * time.Second
	enginePollInterval  = 2 * time.Second
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

	path, lookErr := exec.LookPath(bin)
	if lookErr != nil {
		// #region agent log
		if bin == "podman" {
			AgentDebugLog("A", "runner.go:CheckEngineHealth", "lookpath_miss", map[string]any{"bin": bin, "err": lookErr.Error()})
		}
		// #endregion
		return EngineNotInstalled
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		// #region agent log
		if bin == "podman" {
			AgentDebugLog("A", "runner.go:CheckEngineHealth", "info_failed", map[string]any{
				"bin": bin, "path": path, "err": err.Error(), "ctxErr": fmt.Sprint(cmdCtx.Err()),
			})
		}
		// #endregion
		return EngineOffline
	}
	// #region agent log
	if bin == "podman" {
		AgentDebugLog("A", "runner.go:CheckEngineHealth", "info_ok", map[string]any{"bin": bin, "path": path})
	}
	// #endregion
	return EngineOnline
}

// StartEngine attempts to start the container runtime daemon when it is offline.
func (r *Runner) StartEngine(ctx context.Context, runtimeName string) error {
	bin := runtimeName
	if bin == "" {
		bin = "docker"
	}
	health := r.CheckEngineHealth(ctx, bin)
	// #region agent log
	AgentDebugLog("E", "runner.go:StartEngine", "entry", map[string]any{"bin": bin, "health": string(health)})
	// #endregion
	switch health {
	case EngineOnline:
		return nil
	case EngineNotInstalled:
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, bin)
	}
	switch bin {
	case "podman":
		err := r.startPodmanMachine(ctx)
		// #region agent log
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		AgentDebugLog("E", "runner.go:StartEngine", "podman_done", map[string]any{"err": errStr})
		// #endregion
		return err
	case "docker":
		return r.startDockerEngine(ctx)
	default:
		return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, bin)
	}
}

// StopEngine attempts to stop the container runtime daemon when it is online.
func (r *Runner) StopEngine(ctx context.Context, runtimeName string) error {
	bin := runtimeName
	if bin == "" {
		bin = "docker"
	}
	health := r.CheckEngineHealth(ctx, bin)
	switch health {
	case EngineOffline:
		return nil
	case EngineNotInstalled:
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, bin)
	}
	switch bin {
	case "podman":
		return r.stopPodmanMachine(ctx)
	case "docker":
		return r.stopDockerEngine(ctx)
	default:
		return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, bin)
	}
}

func (r *Runner) stopPodmanMachine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "podman", "machine", "stop")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if r.CheckEngineHealth(context.Background(), "podman") != EngineOnline {
			return nil
		}
		return fmt.Errorf("%w: podman machine stop: %v (%s)", ErrEngineStartFailed, err, strings.TrimSpace(stderr.String()))
	}
	return r.waitUntilOffline(cmdCtx, "podman")
}

func (r *Runner) waitUntilOffline(ctx context.Context, runtimeName string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if r.CheckEngineHealth(ctx, runtimeName) != EngineOnline {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: timed out waiting for %s to stop", ErrEngineStartFailed, runtimeName)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: timed out waiting for %s to stop", ErrEngineStartFailed, runtimeName)
		case <-time.After(enginePollInterval):
		}
	}
}

func isPodmanMachineAlreadyRunning(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "already running")
}

func (r *Runner) execPodmanMachine(ctx context.Context, subcmd string) (stderr string, err error) {
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

// pickPodmanConnectionToDefault returns the first working non-default connection
// name that should become the new default. Empty string means no change needed
// (already healthy via default, or nothing worked).
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

func (r *Runner) listPodmanConnections(ctx context.Context) ([]podmanSystemConnection, error) {
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

func (r *Runner) podmanInfoOK(ctx context.Context, connection string) bool {
	args := []string{"info"}
	if connection != "" {
		args = []string{"--connection", connection, "info"}
	}
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func (r *Runner) setPodmanDefaultConnection(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "podman", "system", "connection", "default", name)
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman system connection default %s: %v (%s)", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// repairPodmanDefaultConnection switches the default connection to one that can
// answer `podman info`. On Podman 6 + WSL the *-root default often points at
// /run/podman/podman.sock which may not exist while the user socket does.
func (r *Runner) repairPodmanDefaultConnection(ctx context.Context) bool {
	if r.CheckEngineHealth(ctx, "podman") == EngineOnline {
		return true
	}
	conns, err := r.listPodmanConnections(ctx)
	if err != nil || len(conns) == 0 {
		// #region agent log
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		AgentDebugLog("G", "runner.go:repairPodmanDefaultConnection", "list_failed", map[string]any{"err": errStr})
		// #endregion
		return false
	}
	working := make(map[string]bool, len(conns))
	for _, c := range conns {
		working[c.Name] = r.podmanInfoOK(ctx, c.Name)
	}
	name := pickPodmanConnectionToDefault(conns, working)
	// #region agent log
	AgentDebugLog("G", "runner.go:repairPodmanDefaultConnection", "probe", map[string]any{
		"working": working, "pick": name, "runId": "post-fix",
	})
	// #endregion
	if name == "" {
		// Either default already works (checked above) or nothing answered.
		for _, c := range conns {
			if c.Default && working[c.Name] {
				return true
			}
		}
		return false
	}
	if err := r.setPodmanDefaultConnection(ctx, name); err != nil {
		// #region agent log
		AgentDebugLog("G", "runner.go:repairPodmanDefaultConnection", "set_default_failed", map[string]any{"err": err.Error(), "name": name})
		// #endregion
		return false
	}
	ok := r.CheckEngineHealth(ctx, "podman") == EngineOnline
	// #region agent log
	AgentDebugLog("G", "runner.go:repairPodmanDefaultConnection", "repaired", map[string]any{"name": name, "online": ok, "runId": "post-fix"})
	// #endregion
	return ok
}

func (r *Runner) startPodmanMachine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	// Machine may already be running with a broken default connection URI.
	if r.repairPodmanDefaultConnection(cmdCtx) {
		return nil
	}

	stderr, err := r.execPodmanMachine(cmdCtx, "start")
	// #region agent log
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	AgentDebugLog("B", "runner.go:startPodmanMachine", "first_start", map[string]any{
		"err": errStr, "stderr": stderr, "alreadyRunning": isPodmanMachineAlreadyRunning(stderr), "runId": "post-fix",
	})
	// #endregion
	if err != nil {
		if r.CheckEngineHealth(cmdCtx, "podman") == EngineOnline {
			return nil
		}
		// Windows/WSL: machine reports "running" but podman info cannot connect
		// (stale SSH/socket). podman machine start then returns "already running".
		if isPodmanMachineAlreadyRunning(stderr) {
			stopStderr, stopErr := r.execPodmanMachine(cmdCtx, "stop")
			// #region agent log
			stopErrStr := ""
			if stopErr != nil {
				stopErrStr = stopErr.Error()
			}
			AgentDebugLog("D", "runner.go:startPodmanMachine", "recovery_stop", map[string]any{
				"err": stopErrStr, "stderr": stopStderr,
			})
			// #endregion
			if stopErr != nil {
				if r.CheckEngineHealth(cmdCtx, "podman") == EngineOnline {
					return nil
				}
			}
			stderr, err = r.execPodmanMachine(cmdCtx, "start")
			// #region agent log
			errStr = ""
			if err != nil {
				errStr = err.Error()
			}
			AgentDebugLog("B", "runner.go:startPodmanMachine", "recovery_start", map[string]any{
				"err": errStr, "stderr": stderr,
			})
			// #endregion
			if err != nil {
				if r.CheckEngineHealth(cmdCtx, "podman") == EngineOnline {
					return nil
				}
				return fmt.Errorf("%w: podman machine restart after stuck state: %v (%s)", ErrEngineStartFailed, err, stderr)
			}
		} else {
			return fmt.Errorf("%w: podman machine start: %v (%s)", ErrEngineStartFailed, err, stderr)
		}
	}

	if !r.repairPodmanDefaultConnection(cmdCtx) {
		waitErr := r.waitUntilOnline(cmdCtx, "podman")
		// #region agent log
		waitErrStr := ""
		if waitErr != nil {
			waitErrStr = waitErr.Error()
		}
		AgentDebugLog("C", "runner.go:startPodmanMachine", "wait_online", map[string]any{"err": waitErrStr, "runId": "post-fix"})
		// #endregion
		if waitErr != nil && !r.repairPodmanDefaultConnection(cmdCtx) {
			return waitErr
		}
	}
	return nil
}

// windowsNamedPipeToDOCKERHost converts a Windows pipe path to a DOCKER_HOST value.
// Example: \\.\pipe\podman-machine-default → npipe:////./pipe/podman-machine-default
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

type podmanMachineInspect struct {
	ConnectionInfo struct {
		PodmanPipe *struct {
			Path string `json:"Path"`
		} `json:"PodmanPipe"`
	} `json:"ConnectionInfo"`
}

func (r *Runner) podmanMachineDOCKERHost(ctx context.Context) (string, error) {
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

func (r *Runner) probeDOCKERHost(ctx context.Context, dockerHost string) bool {
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

func (r *Runner) podmanComposeAPIReady(ctx context.Context) bool {
	if runtime.GOOS != "windows" {
		return true
	}
	host, err := r.podmanMachineDOCKERHost(ctx)
	if err != nil {
		// #region agent log
		AgentDebugLog("H", "runner.go:podmanComposeAPIReady", "inspect_failed", map[string]any{"err": err.Error(), "runId": "post-fix"})
		// #endregion
		return false
	}
	ok := r.probeDOCKERHost(ctx, host)
	// #region agent log
	AgentDebugLog("H", "runner.go:podmanComposeAPIReady", "probe", map[string]any{"host": host, "ok": ok, "runId": "post-fix"})
	// #endregion
	return ok
}

// ensurePodmanComposeReady makes sure Windows Docker-API named-pipe forwarding
// works. `podman compose` shells out to docker-compose, which needs that pipe
// even when `podman info` over SSH already succeeds.
func (r *Runner) ensurePodmanComposeReady(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		// Cannot probe; leave compose to fail with a clearer provider error.
		return nil
	}
	if r.podmanComposeAPIReady(ctx) {
		return nil
	}
	// #region agent log
	AgentDebugLog("H", "runner.go:ensurePodmanComposeReady", "refresh_machine", map[string]any{"runId": "post-fix"})
	// #endregion
	_, _ = r.execPodmanMachine(ctx, "stop")
	stderr, err := r.execPodmanMachine(ctx, "start")
	if err != nil && !isPodmanMachineAlreadyRunning(stderr) {
		if r.CheckEngineHealth(ctx, "podman") != EngineOnline {
			return fmt.Errorf("%w: refresh podman machine for compose API: %v (%s)", ErrEngineStartFailed, err, stderr)
		}
	}
	_ = r.repairPodmanDefaultConnection(ctx)

	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if r.podmanComposeAPIReady(ctx) {
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

func isPodmanComposePipeError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "pipe") && (strings.Contains(lower, "eof") || strings.Contains(lower, "error during connect"))
}

func (r *Runner) waitUntilOnline(ctx context.Context, runtimeName string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if r.CheckEngineHealth(ctx, runtimeName) == EngineOnline {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, runtimeName)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, runtimeName)
		case <-time.After(enginePollInterval):
		}
	}
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

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if inst.Runtime == "podman" {
		if err := r.startPodmanContainer(cmdCtx, inst); err != nil {
			return err
		}
		inst.Status = StatusStarting
		return nil
	}

	bin, args := r.BuildComposeArgs(inst, "up", "-d")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		detail := FormatComposeStderr(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("failed to start container: %s (%w)", detail, err)
	}
	inst.Status = StatusStarting
	return nil
}

// Stop halts the container (down).
func (r *Runner) Stop(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if inst.Runtime == "podman" {
		if err := r.stopPodmanContainer(cmdCtx, inst); err != nil {
			return err
		}
		inst.Status = StatusStopped
		inst.MemoryUsage = "-"
		return nil
	}

	bin, args := r.BuildComposeArgs(inst, "down")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		detail := FormatComposeStderr(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("failed to stop container: %s (%w)", detail, err)
	}
	inst.Status = StatusStopped
	inst.MemoryUsage = "-"
	return nil
}

// DownVolumes stops and deletes container volumes (down -v).
func (r *Runner) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if inst.Runtime == "podman" {
		if err := r.purgePodmanContainer(cmdCtx, inst); err != nil {
			return err
		}
		inst.Status = StatusStopped
		inst.MemoryUsage = "-"
		return nil
	}

	bin, args := r.BuildComposeArgs(inst, "down", "-v")
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		detail := FormatComposeStderr(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("failed to purge container: %s (%w)", detail, err)
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
	if inst.Runtime == "podman" {
		return exec.Command("podman", "logs", "--tail=100", "-f", inst.ContainerName)
	}
	bin, args := r.BuildComposeArgs(inst, "logs", "--tail=100", "-f")
	return exec.Command(bin, args...)
}
