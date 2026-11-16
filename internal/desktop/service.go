package desktop

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
	"github.com/Opsord/local-database-manager/internal/updater"
	"github.com/Opsord/local-database-manager/internal/version"
)

// EventEmitter delivers backend events to the UI layer (Wails runtime in production, stub in tests).
type EventEmitter func(event string, data ...any)

// AppMetadata exposes workspace configuration and supported engine versions to the frontend.
type AppMetadata struct {
	AppVersion              string   `json:"app_version"`
	CheckUpdates            bool     `json:"check_updates"`
	ProjectRoot             string   `json:"project_root"`
	InstancesDir            string   `json:"instances_dir"`
	PostgresVersions        []string `json:"postgres_versions"`
	SQLServerVersions       []string `json:"sqlserver_versions"`
	DefaultPostgresVersion  string   `json:"default_postgres_version"`
	DefaultSQLServerVersion string   `json:"default_sqlserver_version"`
	EngineHealthIntervalMs  int64    `json:"engine_health_interval_ms"`
}

// UpdateProgressEvent is emitted while downloading, verifying, and installing an app update.
type UpdateProgressEvent struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
}

// InstanceDTO is the JSON-serializable instance view returned to the desktop UI.
type InstanceDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	EngineType    string `json:"engine_type"`
	Runtime       string `json:"runtime"`
	EnvFilePath   string `json:"env_file_path"`
	ContainerName string `json:"container_name"`
	ProjectName   string `json:"project_name"`
	Port          int    `json:"port"`
	Database      string `json:"database"`
	User          string `json:"user"`
	Password      string `json:"password"`
	Schema        string `json:"schema"`
	Volume        string `json:"volume"`
	Version       string `json:"version"`
	Status        string `json:"status"`
	CPUUsage      string `json:"cpu_usage"`
	MemoryUsage   string `json:"memory_usage"`
	MemoryLimit   string `json:"memory_limit"`
	ConnectionURI string `json:"connection_uri"`
	BackendEnv    string `json:"backend_env"`
	CLICommand    string `json:"cli_command"`
}

// EnginesHealthDTO holds daemon health statuses for supported runtimes.
type EnginesHealthDTO struct {
	Docker string `json:"docker"`
	Podman string `json:"podman"`
}

// SaveInstanceRequest represents the create/edit wizard payload from the UI.
type SaveInstanceRequest struct {
	IsEdit        bool   `json:"is_edit"`
	OriginalName  string `json:"original_name"`
	Engine        string `json:"engine"`
	Runtime       string `json:"runtime"`
	Version       string `json:"version"`
	Name          string `json:"name"`
	ContainerName string `json:"container_name"`
	Port          string `json:"port"`
	Database      string `json:"database"`
	Password      string `json:"password"`
	MemoryLimit   string `json:"memory_limit"`
}

// SaveInstanceResponse describes the saved instance and whether a container restart prompt is needed.
type SaveInstanceResponse struct {
	Name               string `json:"name"`
	EnvPath            string `json:"env_path"`
	DerivedVolume      string `json:"derived_volume"`
	SourceVolume       string `json:"source_volume"`
	VolumeChanged      bool   `json:"volume_changed"`
	NeedsRestartPrompt bool   `json:"needs_restart_prompt"`
	CleanupOldEnvPath  string `json:"cleanup_old_env_path"`
}

// LogLineEvent is emitted whenever a new log line is read from the active container stream.
type LogLineEvent struct {
	StreamID     uint64 `json:"stream_id"`
	InstanceName string `json:"instance_name"`
	Line         string `json:"line"`
}

// LogClosedEvent is emitted when a container log stream ends.
type LogClosedEvent struct {
	StreamID     uint64 `json:"stream_id"`
	InstanceName string `json:"instance_name"`
	Error        string `json:"error,omitempty"`
}

// Service provides all desktop application operations backed by internal/core.
type Service struct {
	projectRoot  string
	instancesDir string
	cfg          config.Config
	runner       core.InstanceRunner

	emitMu  sync.RWMutex
	emitter EventEmitter

	pendingMu    sync.Mutex
	pendingEdits map[string]*core.DatabaseInstance

	logMu       sync.Mutex
	logStreamID uint64
	logCmd      *exec.Cmd

	quitMu sync.RWMutex
	quitFn func()

	updateMu      sync.Mutex
	lastUpdate    *updater.ReleaseInfo
	updateBaseURL string
}

// NewService constructs a desktop Service for projectRoot.
func NewService(projectRoot string, cfg config.Config, runner core.InstanceRunner) *Service {
	if runner == nil {
		runner = core.NewRunner(projectRoot)
	}
	return &Service{
		projectRoot:  projectRoot,
		instancesDir: filepath.Join(projectRoot, "instances"),
		cfg:          cfg,
		runner:       runner,
		pendingEdits: make(map[string]*core.DatabaseInstance),
	}
}

// SetEventEmitter registers the callback used to push asynchronous events (such as live logs) to the UI.
func (s *Service) SetEventEmitter(fn EventEmitter) {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	s.emitter = fn
}

// SetQuitFunc registers the callback used to close the desktop window cleanly (e.g. after updating).
func (s *Service) SetQuitFunc(fn func()) {
	s.quitMu.Lock()
	defer s.quitMu.Unlock()
	s.quitFn = fn
}

func (s *Service) emit(event string, data ...any) {
	s.emitMu.RLock()
	fn := s.emitter
	s.emitMu.RUnlock()
	if fn != nil {
		fn(event, data...)
	}
}

// GetAppMetadata returns workspace paths, supported versions, and polling intervals.
func (s *Service) GetAppMetadata() AppMetadata {
	intervalMs := s.cfg.EngineHealthInterval.Milliseconds()
	if intervalMs <= 0 {
		intervalMs = 5000
	}
	return AppMetadata{
		AppVersion:              version.Display(),
		CheckUpdates:            s.cfg.CheckUpdates,
		ProjectRoot:             s.projectRoot,
		InstancesDir:            s.instancesDir,
		PostgresVersions:        append([]string(nil), core.PostgresVersions...),
		SQLServerVersions:       append([]string(nil), core.SQLServerVersions...),
		DefaultPostgresVersion:  core.DefaultPostgresVersion,
		DefaultSQLServerVersion: core.DefaultSQLServerVersion,
		EngineHealthIntervalMs:  intervalMs,
	}
}

// ListInstances scans instances/*.env and inspects status and resource usage in parallel
// only for instances whose container runtime daemon is currently online.
func (s *Service) ListInstances() ([]InstanceDTO, error) {
	instances, err := core.ScanInstances(s.instancesDir)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return []InstanceDTO{}, nil
	}

	ctx := context.Background()
	runtimes := make(map[string]core.EngineHealth)
	for _, inst := range instances {
		rt := strings.TrimSpace(inst.Runtime)
		if rt == "" {
			rt = "docker"
		}
		if _, seen := runtimes[rt]; !seen {
			runtimes[rt] = core.EngineOffline
		}
	}

	var healthMu sync.Mutex
	var healthWg sync.WaitGroup
	for rt := range runtimes {
		healthWg.Add(1)
		go func(runtimeName string) {
			defer healthWg.Done()
			h := s.runner.CheckEngineHealth(ctx, runtimeName)
			healthMu.Lock()
			runtimes[runtimeName] = h
			healthMu.Unlock()
		}(rt)
	}
	healthWg.Wait()

	var wg sync.WaitGroup
	for _, inst := range instances {
		rt := strings.TrimSpace(inst.Runtime)
		if rt == "" {
			rt = "docker"
		}
		if runtimes[rt] != core.EngineOnline {
			inst.Status = core.StatusStopped
			inst.CPUUsage = "-"
			inst.MemoryUsage = "-"
			continue
		}
		wg.Add(1)
		go func(i *core.DatabaseInstance) {
			defer wg.Done()
			s.refreshInstanceMetrics(ctx, i)
		}(inst)
	}
	wg.Wait()

	dtos := make([]InstanceDTO, 0, len(instances))
	for _, inst := range instances {
		dtos = append(dtos, toInstanceDTO(inst))
	}
	return dtos, nil
}

func (s *Service) refreshInstanceMetrics(ctx context.Context, inst *core.DatabaseInstance) {
	inst.Status = s.runner.CheckStatus(ctx, inst)
	if inst.Status != core.StatusStopped {
		stats := s.runner.GetStats(ctx, inst)
		inst.CPUUsage = stats.CPU
		inst.MemoryUsage = stats.Memory
	} else {
		inst.CPUUsage = "-"
		inst.MemoryUsage = "-"
	}
}

func toInstanceDTO(inst *core.DatabaseInstance) InstanceDTO {
	return InstanceDTO{
		ID:            inst.ID,
		Name:          inst.Name,
		EngineType:    inst.EngineType,
		Runtime:       inst.Runtime,
		EnvFilePath:   inst.EnvFilePath,
		ContainerName: inst.ContainerName,
		ProjectName:   inst.ProjectName,
		Port:          inst.Port,
		Database:      inst.Database,
		User:          inst.User,
		Password:      inst.Password,
		Schema:        inst.Schema,
		Volume:        inst.Volume,
		Version:       inst.Version,
		Status:        string(inst.Status),
		CPUUsage:      inst.CPUUsage,
		MemoryUsage:   inst.MemoryUsage,
		MemoryLimit:   inst.MemoryLimit,
		ConnectionURI: inst.ConnectionURI(),
		BackendEnv:    inst.BackendEnvBlock(),
		CLICommand:    inst.CLICommand(),
	}
}

func (s *Service) findInstance(name string) (*core.DatabaseInstance, error) {
	instances, err := core.ScanInstances(s.instancesDir)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		if inst.Name == name {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", core.ErrInstanceNotFound, name)
}

// GetEnginesHealth inspects Docker and Podman daemon readiness in parallel.
func (s *Service) GetEnginesHealth() EnginesHealthDTO {
	ctx := context.Background()
	var wg sync.WaitGroup
	var dockerHealth, podmanHealth core.EngineHealth

	wg.Add(2)
	go func() {
		defer wg.Done()
		dockerHealth = s.runner.CheckEngineHealth(ctx, "docker")
	}()
	go func() {
		defer wg.Done()
		podmanHealth = s.runner.CheckEngineHealth(ctx, "podman")
	}()
	wg.Wait()

	return EnginesHealthDTO{
		Docker: string(dockerHealth),
		Podman: string(podmanHealth),
	}
}

// StartEngine launches the container engine daemon for runtimeName ("docker" | "podman").
func (s *Service) StartEngine(runtimeName string) error {
	return s.runner.StartEngine(context.Background(), runtimeName)
}

// StopEngine stops the container engine daemon for runtimeName ("docker" | "podman").
func (s *Service) StopEngine(runtimeName string) error {
	return s.runner.StopEngine(context.Background(), runtimeName)
}

// StartInstance starts the named database instance and returns its refreshed state.
func (s *Service) StartInstance(name string) (InstanceDTO, error) {
	inst, err := s.findInstance(name)
	if err != nil {
		return InstanceDTO{}, err
	}
	ctx := context.Background()
	if err := s.runner.Start(ctx, inst); err != nil {
		if errors.Is(err, core.ErrEngineOffline) {
			return InstanceDTO{}, fmt.Errorf("%s: %w", core.ErrCodeEngineOffline, err)
		}
		return InstanceDTO{}, err
	}
	s.refreshInstanceMetrics(ctx, inst)
	return toInstanceDTO(inst), nil
}

// StopInstance stops the named database instance and returns its refreshed state.
func (s *Service) StopInstance(name string) (InstanceDTO, error) {
	inst, err := s.findInstance(name)
	if err != nil {
		return InstanceDTO{}, err
	}
	ctx := context.Background()
	if err := s.runner.Stop(ctx, inst); err != nil {
		return InstanceDTO{}, err
	}
	s.refreshInstanceMetrics(ctx, inst)
	return toInstanceDTO(inst), nil
}

// PurgeInstance removes the container and its associated volume while keeping the .env file.
func (s *Service) PurgeInstance(name string) (InstanceDTO, error) {
	inst, err := s.findInstance(name)
	if err != nil {
		return InstanceDTO{}, err
	}
	ctx := context.Background()
	if err := s.runner.DownVolumes(ctx, inst); err != nil {
		return InstanceDTO{}, err
	}
	s.refreshInstanceMetrics(ctx, inst)
	return toInstanceDTO(inst), nil
}

// DeleteInstance verifies engine health, purges container + volume, and deletes the .env file.
func (s *Service) DeleteInstance(name string) error {
	inst, err := s.findInstance(name)
	if err != nil {
		return err
	}
	ctx := context.Background()
	runtimeName := inst.Runtime
	if runtimeName == "" {
		runtimeName = "docker"
	}
	health := s.runner.CheckEngineHealth(ctx, runtimeName)
	if health == core.EngineNotInstalled {
		return fmt.Errorf("%w: %s — start is unavailable; cannot delete", core.ErrEngineNotInstalled, runtimeName)
	}
	if health == core.EngineOffline {
		return fmt.Errorf("%s: %w: %s is offline — start it from Engines before deleting", core.ErrCodeEngineOffline, core.ErrEngineOffline, runtimeName)
	}
	if err := s.runner.DownVolumes(ctx, inst); err != nil {
		return err
	}
	if err := os.Remove(inst.EnvFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SuggestFreePort finds the next unallocated TCP port starting at startPort.
func (s *Service) SuggestFreePort(startPort int) int {
	if startPort <= 0 {
		startPort = 5432
	}
	instances, _ := core.ScanInstances(s.instancesDir)
	return core.FindNextFreePort(startPort, instances)
}

// PreviewDerivedVolume returns the deterministic volume name for the given parameters.
func (s *Service) PreviewDerivedVolume(engine, name, version string) string {
	var ver string
	if strings.ToLower(strings.TrimSpace(engine)) == "postgres" {
		ver = version
	}
	return core.DeriveVolumeName(engine, name, ver)
}

// GetEngineDefaults computes default container name, free port, database, password, memory, and volume.
func (s *Service) GetEngineDefaults(engine, name, version string) map[string]string {
	prefix, _, defaultPort, defaultPass, defaultMem := core.EngineDefaults(engine)
	cleanName := core.SanitizeIdent(name)

	portNum, _ := strconv.Atoi(defaultPort)
	if portNum <= 0 {
		portNum = 5432
	}
	freePort := s.SuggestFreePort(portNum)

	containerName := ""
	dbName := ""
	if cleanName != "" {
		containerName = fmt.Sprintf("%s-%s", prefix, cleanName)
		dbName = fmt.Sprintf("%s_db", cleanName)
	}
	if strings.ToLower(strings.TrimSpace(engine)) == "sqlserver" && core.NormalizeSQLServerVersion(version) == core.SQLServerAzureEdge {
		defaultMem = "1G"
	}

	return map[string]string{
		"container_name": containerName,
		"port":           strconv.Itoa(freePort),
		"database":       dbName,
		"password":       defaultPass,
		"memory_limit":   defaultMem,
		"derived_volume": s.PreviewDerivedVolume(engine, cleanName, version),
	}
}

// SaveInstance creates or updates an instance .env file on disk.
func (s *Service) SaveInstance(req SaveInstanceRequest) (SaveInstanceResponse, error) {
	cleanName := core.SanitizeIdent(req.Name)
	if cleanName == "" {
		return SaveInstanceResponse{}, fmt.Errorf("instance name cannot be empty")
	}

	portNum, err := strconv.Atoi(strings.TrimSpace(req.Port))
	if err != nil || portNum < 1 || portNum > 65535 {
		return SaveInstanceResponse{}, fmt.Errorf("port must be a number between 1 and 65535")
	}

	existing, err := core.ScanInstances(s.instancesDir)
	if err != nil {
		return SaveInstanceResponse{}, err
	}

	for _, inst := range existing {
		if inst.Name == cleanName {
			if req.IsEdit && cleanName == req.OriginalName {
				continue
			}
			return SaveInstanceResponse{}, fmt.Errorf("instance name '%s' is already taken", cleanName)
		}
	}

	var spec core.InstanceFormSpec
	var origSnapshot *core.DatabaseInstance
	sourceVolume := ""

	if req.IsEdit {
		var orig *core.DatabaseInstance
		for _, inst := range existing {
			if inst.Name == req.OriginalName {
				orig = inst
				break
			}
		}
		if orig == nil {
			return SaveInstanceResponse{}, fmt.Errorf("%w: %s", core.ErrInstanceNotFound, req.OriginalName)
		}

		status := s.runner.CheckStatus(context.Background(), orig)
		wasRunning := status == core.StatusReady || status == core.StatusStarting
		sourceVolume = orig.Volume

		if wasRunning {
			copyOrig := *orig
			origSnapshot = &copyOrig
		}

		// Runtime is locked on edit, matching TUI wizard semantics.
		spec = core.InstanceFormSpec{
			Kind:          "edit",
			SourceName:    orig.Name,
			SourceEnvPath: orig.EnvFilePath,
			SourceEngine:  orig.EngineType,
			SourceVolume:  orig.Volume,
			SourceUser:    orig.User,
			SourceSchema:  orig.Schema,
			SourceRawEnv:  orig.RawEnv,
			WasRunning:    wasRunning,
			Engine:        req.Engine,
			Runtime:       orig.Runtime,
			Version:       req.Version,
			Name:          cleanName,
			ContainerName: req.ContainerName,
			Port:          strconv.Itoa(portNum),
			Database:      req.Database,
			Password:      req.Password,
			MemoryLimit:   req.MemoryLimit,
		}
	} else {
		prefix, _, _, defaultPass, defaultMem := core.EngineDefaults(req.Engine)
		containerName := strings.TrimSpace(req.ContainerName)
		if containerName == "" {
			containerName = fmt.Sprintf("%s-%s", prefix, cleanName)
		}
		dbName := strings.TrimSpace(req.Database)
		if dbName == "" {
			dbName = fmt.Sprintf("%s_db", cleanName)
		}
		pass := strings.TrimSpace(req.Password)
		if pass == "" {
			pass = defaultPass
		}
		memLimit := strings.TrimSpace(req.MemoryLimit)
		if memLimit == "" {
			if strings.ToLower(strings.TrimSpace(req.Engine)) == "sqlserver" && core.NormalizeSQLServerVersion(req.Version) == core.SQLServerAzureEdge {
				memLimit = "1G"
			} else {
				memLimit = defaultMem
			}
		}

		spec = core.InstanceFormSpec{
			Kind:          "create",
			Engine:        req.Engine,
			Runtime:       req.Runtime,
			Version:       req.Version,
			Name:          cleanName,
			ContainerName: containerName,
			Port:          strconv.Itoa(portNum),
			Database:      dbName,
			Password:      pass,
			MemoryLimit:   memLimit,
		}
	}

	res, err := core.SaveInstanceEnv(s.instancesDir, spec)
	if err != nil {
		return SaveInstanceResponse{}, err
	}

	if origSnapshot != nil {
		s.pendingMu.Lock()
		s.pendingEdits[origSnapshot.Name] = origSnapshot
		s.pendingMu.Unlock()
	}

	return SaveInstanceResponse{
		Name:               res.Name,
		EnvPath:            res.EnvPath,
		DerivedVolume:      res.DerivedVolume,
		SourceVolume:       sourceVolume,
		VolumeChanged:      res.VolumeChanged,
		NeedsRestartPrompt: res.NeedsRestartPrompt,
		CleanupOldEnvPath:  res.CleanupOldEnvPath,
	}, nil
}

// RestartAfterEdit stops the prior container configuration, cleans up renamed .env files, and starts the updated instance.
func (s *Service) RestartAfterEdit(oldName, newName, oldEnvPath string) error {
	ctx := context.Background()

	s.pendingMu.Lock()
	oldInst := s.pendingEdits[oldName]
	delete(s.pendingEdits, oldName)
	s.pendingMu.Unlock()

	if oldInst == nil && oldEnvPath != "" {
		if parsed, err := core.ParseEnvFile(oldEnvPath); err == nil {
			oldInst = parsed
		}
	}
	if oldInst == nil {
		if found, err := s.findInstance(oldName); err == nil {
			oldInst = found
		}
	}

	var stopErr error
	if oldInst != nil {
		stopErr = s.runner.Stop(ctx, oldInst)
	}

	newEnvPath := filepath.Join(s.instancesDir, newName+".env")
	newInst, err := core.ParseEnvFile(newEnvPath)
	if err != nil {
		return err
	}

	startErr := s.runner.Start(ctx, newInst)
	if startErr != nil {
		if stopErr != nil {
			return fmt.Errorf("stop: %v; start: %v", stopErr, startErr)
		}
		return startErr
	}

	if oldEnvPath != "" && oldEnvPath != newEnvPath {
		_ = os.Remove(oldEnvPath)
	}
	return nil
}

// DiscardRenameCleanup removes an old renamed .env file when the user declines restarting immediately.
func (s *Service) DiscardRenameCleanup(oldEnvPath string) error {
	if oldEnvPath == "" {
		return nil
	}
	cleanDir := filepath.Clean(s.instancesDir) + string(os.PathSeparator)
	cleanTarget := filepath.Clean(oldEnvPath)
	if !strings.HasPrefix(cleanTarget, cleanDir) {
		return fmt.Errorf("refusing to delete path outside instances directory")
	}
	if err := os.Remove(cleanTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// StartLogStream begins streaming container logs for name and emits logs:line and logs:closed events.
func (s *Service) StartLogStream(name string) error {
	inst, err := s.findInstance(name)
	if err != nil {
		return err
	}

	s.logMu.Lock()
	s.stopLogStreamLocked()
	s.logStreamID++
	streamID := s.logStreamID
	s.logMu.Unlock()

	s.emit("logs:line", LogLineEvent{
		StreamID:     streamID,
		InstanceName: inst.Name,
		Line:         fmt.Sprintf("Starting logs stream for %s (%s)...", inst.Name, inst.ContainerName),
	})

	cmd := s.runner.LogsCommand(inst)
	if cmd == nil {
		errMsg := "logs command is not available"
		s.emit("logs:line", LogLineEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Line:         "Error: " + errMsg,
		})
		s.emit("logs:closed", LogClosedEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Error:        errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.emit("logs:line", LogLineEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Line:         fmt.Sprintf("Error opening stdout pipe: %v", err),
		})
		s.emit("logs:closed", LogClosedEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Error:        err.Error(),
		})
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		s.emit("logs:line", LogLineEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Line:         fmt.Sprintf("Error starting logs command: %v", err),
		})
		s.emit("logs:closed", LogClosedEvent{
			StreamID:     streamID,
			InstanceName: inst.Name,
			Error:        err.Error(),
		})
		return err
	}

	s.logMu.Lock()
	if s.logStreamID != streamID {
		s.logMu.Unlock()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		return nil
	}
	s.logCmd = cmd
	s.logMu.Unlock()

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			s.logMu.Lock()
			active := s.logStreamID == streamID
			s.logMu.Unlock()
			if !active {
				break
			}
			s.emit("logs:line", LogLineEvent{
				StreamID:     streamID,
				InstanceName: inst.Name,
				Line:         scanner.Text(),
			})
		}

		scanErr := scanner.Err()
		_ = cmd.Wait()

		s.logMu.Lock()
		stillActive := s.logStreamID == streamID
		if stillActive {
			s.logCmd = nil
		}
		s.logMu.Unlock()

		if stillActive {
			errText := ""
			if scanErr != nil {
				errText = scanErr.Error()
			}
			s.emit("logs:closed", LogClosedEvent{
				StreamID:     streamID,
				InstanceName: inst.Name,
				Error:        errText,
			})
		}
	}()

	return nil
}

// StopLogStream terminates any active container log stream.
func (s *Service) StopLogStream() error {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.stopLogStreamLocked()
	return nil
}

func (s *Service) stopLogStreamLocked() {
	s.logStreamID++
	if s.logCmd != nil && s.logCmd.Process != nil {
		_ = s.logCmd.Process.Kill()
	}
	s.logCmd = nil
}

// CopyText copies text to the operating system clipboard.
func (s *Service) CopyText(text string) error {
	return core.CopyToClipboard(text)
}

// OpenEnvInEditor opens the instance's .env file in the user's configured editor.
func (s *Service) OpenEnvInEditor(name string) error {
	inst, err := s.findInstance(name)
	if err != nil {
		return err
	}
	return core.OpenInEditor(inst.EnvFilePath)
}

// Shutdown releases background log streams on application exit.
func (s *Service) Shutdown() {
	_ = s.StopLogStream()
}

// CheckForUpdates checks GitHub Releases for a newer desktop release and caches the result.
func (s *Service) CheckForUpdates() (updater.ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.updateMu.Lock()
	baseURL := s.updateBaseURL
	s.updateMu.Unlock()

	info, err := updater.Check(ctx, updater.CheckOptions{
		Target:            updater.TargetDesktop,
		IncludePrerelease: s.cfg.UpdateChannel == "prerelease",
		BaseURL:           baseURL,
	})
	if err != nil {
		return updater.ReleaseInfo{}, err
	}

	s.updateMu.Lock()
	infoCopy := info
	s.lastUpdate = &infoCopy
	s.updateMu.Unlock()

	return info, nil
}

// ApplyUpdate downloads, verifies, and installs the cached (or newly checked) desktop release.
func (s *Service) ApplyUpdate() error {
	s.updateMu.Lock()
	cached := s.lastUpdate
	s.updateMu.Unlock()

	var info updater.ReleaseInfo
	if cached != nil {
		info = *cached
	} else {
		checked, err := s.CheckForUpdates()
		if err != nil {
			return err
		}
		info = checked
	}

	if !info.UpdateAvailable {
		return fmt.Errorf("already running the latest version (%s)", info.CurrentVersion)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := updater.Apply(ctx, info, updater.ApplyOptions{
		OnProgress: func(stage updater.ProgressStage, percent int, msg string) {
			s.emit("update:progress", UpdateProgressEvent{
				Stage:   string(stage),
				Percent: percent,
				Message: msg,
			})
		},
	})
	if err != nil {
		return err
	}

	s.updateMu.Lock()
	if s.lastUpdate != nil {
		s.lastUpdate.UpdateAvailable = false
	}
	s.updateMu.Unlock()

	return nil
}

// RestartApp closes active log streams, launches a new instance of the executable, and exits.
func (s *Service) RestartApp() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable for restart: %w", err)
	}

	s.Shutdown()

	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunch desktop application: %w", err)
	}

	s.quitMu.RLock()
	quitFn := s.quitFn
	s.quitMu.RUnlock()
	if quitFn != nil {
		quitFn()
	}
	return nil
}
