package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
	"github.com/Opsord/local-database-manager/internal/updater"
	"github.com/Opsord/local-database-manager/internal/version"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AppMode int

const (
	ModeMain AppMode = iota
	ModeWizard
	ModeLogs
	ModeActionMenu
	ModeEngineMenu
	ModeHelp
	ModeError
)

type clearStatusMsg struct{}

type actionDoneMsg struct {
	msg         string
	instName    string
	hasUpdate   bool
	status      core.ContainerStatus
	cpuUsage    string
	memoryUsage string
}

type deleteDoneMsg struct {
	name         string
	runtime      string
	skippedPurge bool
	err          error
}

type restartAfterEditDoneMsg struct {
	name string
	err  error
}

type engineHealthMsg struct {
	dockerHealth core.EngineHealth
	podmanHealth core.EngineHealth
	scheduleTick bool
}

type engineHealthTickMsg struct{}

type quitAfterStopMsg struct{}

type updateCheckMsg struct {
	info   updater.ReleaseInfo
	err    error
	manual bool
}

type updateAppliedMsg struct {
	version string
	err     error
}

// AppModel is the root Bubble Tea application state.
type AppModel struct {
	projectRoot  string
	instancesDir string
	runner       core.InstanceRunner
	cfg          config.Config

	mode          AppMode
	instances     []*core.DatabaseInstance
	selectedIndex int

	// Search / Filter
	isFiltering   bool
	filterInput   string

	// Action Menu
	actionMenuIndex int

	// Engine Menu
	engineMenuIndex int
	engineStarting  bool

	dockerHealth core.EngineHealth
	podmanHealth core.EngineHealth

	statusMsg    string
	statusIsErr  bool
	confirmPurge bool
	confirmDelete bool

	confirmEngineStart   bool
	pendingEngineRuntime string
	pendingStartInst     *core.DatabaseInstance

	confirmEngineStop   bool
	pendingStopRuntime  string

	confirmQuitStopEngine bool
	pendingQuitRuntime    string

	confirmRestartAfterEdit bool
	pendingRestartOld       *core.DatabaseInstance
	pendingRestartNewName   string
	pendingDeleteEnvPath    string

	updateInfo     *updater.ReleaseInfo
	updateChecking bool
	updating       bool
	confirmUpdate  bool

	width  int
	height int

	detailHits  []copyHit
	detailClick clickTracker
	copiedHit   *copyHit

	lastError       string
	errorReturnMode AppMode

	// Sub-models
	wizard     wizardModel
	logs       logsModel
	errorModal errorModalModel
}

func (m *AppModel) hasActiveConfirm() bool {
	return m.confirmPurge || m.confirmDelete || m.confirmEngineStart ||
		m.confirmEngineStop || m.confirmRestartAfterEdit ||
		m.confirmQuitStopEngine || m.confirmUpdate
}

func (m *AppModel) setError(msg string) {
	m.statusMsg = msg
	m.statusIsErr = true
	m.lastError = msg
}

func (m *AppModel) finishEditRenameCleanup() {
	if m.pendingDeleteEnvPath == "" {
		return
	}
	_ = os.Remove(m.pendingDeleteEnvPath)
	m.pendingDeleteEnvPath = ""
}

func (m *AppModel) clearConfirms() {
	if m.confirmRestartAfterEdit {
		m.finishEditRenameCleanup()
	}
	m.confirmPurge = false
	m.confirmDelete = false
	m.confirmEngineStart = false
	m.pendingStartInst = nil
	m.pendingEngineRuntime = ""
	m.confirmEngineStop = false
	m.pendingStopRuntime = ""
	m.confirmQuitStopEngine = false
	m.pendingQuitRuntime = ""
	m.confirmRestartAfterEdit = false
	m.pendingRestartOld = nil
	m.pendingRestartNewName = ""
	m.confirmUpdate = false
}

// NewApp instantiates a new AppModel.
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

// Init initializes the model, instantly loads instances and launches parallel inspections in background.
func (m *AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.reloadInstancesCmd(), m.checkEngineHealthCmd(true)}
	if m.cfg.CheckUpdates {
		cmds = append(cmds, m.checkUpdateCmd(false))
	}
	return tea.Batch(cmds...)
}

func (m *AppModel) checkUpdateCmd(manual bool) tea.Cmd {
	includePre := m.cfg.UpdateChannel == "prerelease"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		info, err := updater.Check(ctx, updater.CheckOptions{
			Target:            updater.TargetCLI,
			IncludePrerelease: includePre,
		})
		return updateCheckMsg{
			info:   info,
			err:    err,
			manual: manual,
		}
	}
}

func (m *AppModel) applyUpdateCmd() tea.Cmd {
	if m.updateInfo == nil {
		return nil
	}
	info := *m.updateInfo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := updater.Apply(ctx, info, updater.ApplyOptions{})
		return updateAppliedMsg{
			version: info.LatestVersion,
			err:     err,
		}
	}
}

func (m *AppModel) checkEngineHealthCmd(scheduleTick bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var wg sync.WaitGroup
		var dockerHealth core.EngineHealth
		var podmanHealth core.EngineHealth

		wg.Add(2)
		go func() {
			defer wg.Done()
			dockerHealth = m.runner.CheckEngineHealth(ctx, "docker")
		}()
		go func() {
			defer wg.Done()
			podmanHealth = m.runner.CheckEngineHealth(ctx, "podman")
		}()
		wg.Wait()

		return engineHealthMsg{
			dockerHealth: dockerHealth,
			podmanHealth: podmanHealth,
			scheduleTick: scheduleTick,
		}
	}
}

func (m *AppModel) engineHealthTickCmd() tea.Cmd {
	return tea.Tick(m.cfg.EngineHealthInterval, func(time.Time) tea.Msg {
		return engineHealthTickMsg{}
	})
}

func (m *AppModel) reloadInstancesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		instances, err := core.ScanInstances(m.instancesDir)
		if err != nil {
			return errMsg{err}
		}

		var wg sync.WaitGroup
		for _, inst := range instances {
			wg.Add(1)
			go func(i *core.DatabaseInstance) {
				defer wg.Done()
				i.Status = m.runner.CheckStatus(ctx, i)
				if i.Status != core.StatusStopped {
					stats := m.runner.GetStats(ctx, i)
					i.CPUUsage = stats.CPU
					i.MemoryUsage = stats.Memory
				} else {
					i.CPUUsage = "-"
					i.MemoryUsage = "-"
				}
			}(inst)
		}
		wg.Wait()

		return instancesLoadedMsg{instances: instances}
	}
}

type instancesLoadedMsg struct {
	instances []*core.DatabaseInstance
}

type errMsg struct {
	err error
}

func (e errMsg) Error() string { return e.err.Error() }

// filteredInstances returns the list of instances matching filterInput
func (m *AppModel) filteredInstances() []*core.DatabaseInstance {
	if !m.isFiltering || strings.TrimSpace(m.filterInput) == "" {
		return m.instances
	}
	query := strings.ToLower(strings.TrimSpace(m.filterInput))
	var filtered []*core.DatabaseInstance
	for _, inst := range m.instances {
		if strings.Contains(strings.ToLower(inst.Name), query) ||
			strings.Contains(strings.ToLower(inst.EngineType), query) ||
			strings.Contains(strings.ToLower(inst.Runtime), query) ||
			strings.Contains(strings.ToLower(inst.Database), query) ||
			strings.Contains(fmt.Sprintf("%d", inst.Port), query) {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}

func (m *AppModel) selectedInstance() *core.DatabaseInstance {
	list := m.filteredInstances()
	if len(list) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(list) {
		return nil
	}
	return list[m.selectedIndex]
}

// Update handles application state transitions and messages.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logs.viewport.Width = msg.Width - 4
		m.logs.viewport.Height = msg.Height - 6
		return m, nil

	case instancesLoadedMsg:
		m.instances = msg.instances
		if m.statusMsg == "Reloading instances and inspecting runtimes..." {
			m.statusMsg = ""
		}

		list := m.filteredInstances()
		if m.selectedIndex >= len(list) {
			m.selectedIndex = len(list) - 1
		}
		if m.selectedIndex < 0 && len(list) > 0 {
			m.selectedIndex = 0
		}
		return m, nil

	case engineHealthMsg:
		m.dockerHealth = msg.dockerHealth
		m.podmanHealth = msg.podmanHealth
		if msg.scheduleTick {
			return m, m.engineHealthTickCmd()
		}
		return m, nil

	case engineHealthTickMsg:
		return m, m.checkEngineHealthCmd(true)

	case actionDoneMsg:
		if msg.hasUpdate {
			for _, inst := range m.instances {
				if inst.Name == msg.instName {
					inst.Status = msg.status
					inst.CPUUsage = msg.cpuUsage
					inst.MemoryUsage = msg.memoryUsage
					break
				}
			}
		}
		m.statusMsg = msg.msg
		m.statusIsErr = false
		return m, tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

	case deleteDoneMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}
		if msg.skippedPurge {
			rt := msg.runtime
			if rt == "" {
				rt = "engine"
			}
			m.statusMsg = fmt.Sprintf("Instance '%s' deleted (.env removed; %s not installed, container purge skipped)", msg.name, rt)
		} else {
			m.statusMsg = fmt.Sprintf("Instance '%s' deleted (container, volume, and .env removed)", msg.name)
		}
		m.statusIsErr = false
		return m, tea.Batch(
			m.reloadInstancesCmd(),
			tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
		)

	case restartAfterEditDoneMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Error restarting '%s': %v", msg.name, msg.err))
			return m, m.reloadInstancesCmd()
		}
		m.finishEditRenameCleanup()
		m.statusMsg = fmt.Sprintf("Instance '%s' restarted successfully!", msg.name)
		m.statusIsErr = false
		return m, tea.Batch(
			m.reloadInstancesCmd(),
			tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
		)

	case clearStatusMsg:
		if !m.statusIsErr {
			m.statusMsg = ""
		}
		m.clearCopiedHit()
		return m, nil

	case errMsg:
		m.setError(fmt.Sprintf("Error: %v", msg.err))
		return m, nil

	case offlineStartMsg:
		if m.engineStarting {
			return m, nil
		}
		m.clearConfirms()
		m.confirmEngineStart = true
		m.pendingStartInst = msg.inst
		m.pendingEngineRuntime = msg.inst.Runtime
		name := "Docker"
		if msg.inst.Runtime == "podman" {
			name = "Podman"
		}
		m.statusMsg = fmt.Sprintf("%s is offline. Start engine and retry? Press 'y' to confirm, 'n' to cancel", name)
		m.statusIsErr = true
		return m, nil

	case engineStartedMsg:
		m.engineStarting = false
		runtimeName := msg.runtime
		if runtimeName == "podman" {
			runtimeName = "Podman"
		} else {
			runtimeName = "Docker"
		}
		if msg.err != nil {
			m.setError(fmt.Sprintf("Failed to start %s: %v", runtimeName, msg.err))
			return m, m.checkEngineHealthCmd(false)
		}
		m.statusMsg = fmt.Sprintf("%s is online", runtimeName)
		m.statusIsErr = false
		cmds := []tea.Cmd{
			m.checkEngineHealthCmd(false),
			tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
		}
		if msg.retryInst != nil {
			cmds = append(cmds, m.toggleInstanceCmd(msg.retryInst))
		}
		return m, tea.Batch(cmds...)

	case engineStoppedMsg:
		m.engineStarting = false
		runtimeName := msg.runtime
		if runtimeName == "podman" {
			runtimeName = "Podman"
		} else {
			runtimeName = "Docker"
		}
		if msg.err != nil {
			m.setError(fmt.Sprintf("Failed to stop %s: %v", runtimeName, msg.err))
			return m, m.checkEngineHealthCmd(false)
		}
		m.statusMsg = fmt.Sprintf("%s is offline", runtimeName)
		m.statusIsErr = false
		return m, tea.Batch(
			m.checkEngineHealthCmd(false),
			tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
		)

	case quitAfterStopMsg:
		return m, tea.Quit

	case updateCheckMsg:
		m.updateChecking = false
		if msg.err != nil {
			if msg.manual {
				m.setError(fmt.Sprintf("Update check failed: %v", msg.err))
				return m, nil
			}
			return m, nil
		}
		infoCopy := msg.info
		m.updateInfo = &infoCopy
		if msg.manual {
			if msg.info.UpdateAvailable {
				m.clearConfirms()
				m.confirmUpdate = true
				m.statusMsg = fmt.Sprintf("Update %s available (current: %s). Install now? Press 'y' to confirm, 'n' to cancel", msg.info.LatestVersion, msg.info.CurrentVersion)
				m.statusIsErr = true
				return m, nil
			}
			m.statusMsg = fmt.Sprintf("Up to date (%s)", version.Display())
			m.statusIsErr = false
			return m, tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
		}
		return m, nil

	case updateAppliedMsg:
		m.updating = false
		if msg.err != nil {
			m.setError(fmt.Sprintf("Update failed: %v", msg.err))
			return m, nil
		}
		if m.updateInfo != nil {
			m.updateInfo.UpdateAvailable = false
		}
		m.statusMsg = fmt.Sprintf("Updated to %s! Restart db-manager to use the new version.", msg.version)
		m.statusIsErr = false
		return m, nil

	case tea.MouseMsg:
		switch m.mode {
		case ModeMain, ModeWizard, ModeActionMenu, ModeEngineMenu:
			model, cmd, handled := m.handleStatusErrorMouse(msg)
			if handled {
				return model, cmd
			}
			model, cmd, handled = m.handleDetailsMouse(msg)
			if handled {
				return model, cmd
			}
		}
	}

	// Route based on active mode
	switch m.mode {
	case ModeWizard:
		return m.updateWizard(msg)
	case ModeLogs:
		return m.updateLogs(msg)
	case ModeActionMenu:
		return m.updateActionMenu(msg)
	case ModeEngineMenu:
		return m.updateEngineMenu(msg)
	case ModeHelp:
		return m.updateHelp(msg)
	case ModeError:
		return m.updateErrorModal(msg)
	default:
		return m.updateMain(msg)
	}
}

// View renders the current active UI screen.
func (m *AppModel) View() string {
	if m.width == 0 {
		return lipgloss.NewStyle().Background(BgDark).Foreground(FgText).Render("Initializing...")
	}

	switch m.mode {
	case ModeWizard:
		return m.wrapScreen(m.viewMain())
	case ModeLogs:
		return m.wrapScreen(m.viewLogs())
	case ModeHelp:
		return m.wrapScreen(m.renderOverlay(m.viewHelp()))
	case ModeError:
		return m.wrapScreen(m.renderOverlay(m.viewErrorModal()))
	case ModeActionMenu:
		return m.wrapScreen(m.viewMain())
	case ModeEngineMenu:
		return m.wrapScreen(m.viewMain())
	default:
		return m.wrapScreen(m.viewMain())
	}
}
