package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type AppMode int

const (
	ModeMain AppMode = iota
	ModeWizard
	ModeLogs
	ModeActionMenu
	ModeHelp
)

type clearStatusMsg struct{}

type actionDoneMsg struct {
	msg string
}

// AppModel is the root Bubble Tea application state.
type AppModel struct {
	projectRoot  string
	instancesDir string
	runner       core.InstanceRunner

	mode          AppMode
	instances     []*core.DatabaseInstance
	selectedIndex int

	// Search / Filter
	isFiltering   bool
	filterInput   string

	// Action Menu
	actionMenuIndex int

	dockerHealth core.EngineHealth
	podmanHealth core.EngineHealth

	statusMsg    string
	statusIsErr  bool
	confirmPurge bool

	width  int
	height int

	// Sub-models
	wizard wizardModel
	logs   logsModel
}

// NewApp instantiates a new AppModel.
func NewApp(projectRoot string) *AppModel {
	instancesDir := filepath.Join(projectRoot, "instances")
	runner := core.NewRunner(projectRoot)

	return &AppModel{
		projectRoot:  projectRoot,
		instancesDir: instancesDir,
		runner:       runner,
		mode:         ModeMain,
		logs: logsModel{
			viewport: viewport.New(80, 20),
		},
	}
}

// Init initializes the model and triggers instance scanning and engine health checks.
func (m *AppModel) Init() tea.Cmd {
	return m.reloadInstancesCmd()
}

func (m *AppModel) reloadInstancesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		dockerHealth := m.runner.CheckEngineHealth(ctx, "docker")
		podmanHealth := m.runner.CheckEngineHealth(ctx, "podman")

		instances, err := core.ScanInstances(m.instancesDir)
		if err != nil {
			return errMsg{err}
		}

		// Inspect current status and stats of all instances
		for _, inst := range instances {
			inst.Status = m.runner.CheckStatus(ctx, inst)
			if inst.Status != core.StatusStopped {
				inst.MemoryUsage = m.runner.GetMemoryUsage(ctx, inst)
			} else {
				inst.MemoryUsage = "-"
			}
		}

		return instancesLoadedMsg{
			instances:    instances,
			dockerHealth: dockerHealth,
			podmanHealth: podmanHealth,
		}
	}
}

type instancesLoadedMsg struct {
	instances    []*core.DatabaseInstance
	dockerHealth core.EngineHealth
	podmanHealth core.EngineHealth
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
		m.dockerHealth = msg.dockerHealth
		m.podmanHealth = msg.podmanHealth

		list := m.filteredInstances()
		if m.selectedIndex >= len(list) {
			m.selectedIndex = len(list) - 1
		}
		if m.selectedIndex < 0 && len(list) > 0 {
			m.selectedIndex = 0
		}
		return m, nil

	case actionDoneMsg:
		m.statusMsg = msg.msg
		m.statusIsErr = false
		return m, tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case errMsg:
		m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		m.statusIsErr = true
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
	}

	// Route based on active mode
	switch m.mode {
	case ModeWizard:
		return m.updateWizard(msg)
	case ModeLogs:
		return m.updateLogs(msg)
	case ModeActionMenu:
		return m.updateActionMenu(msg)
	case ModeHelp:
		return m.updateHelp(msg)
	default:
		return m.updateMain(msg)
	}
}

// View renders the current active UI screen.
func (m *AppModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	switch m.mode {
	case ModeWizard:
		return m.viewWizard()
	case ModeLogs:
		return m.viewLogs()
	case ModeActionMenu:
		return m.viewActionMenu()
	case ModeHelp:
		return m.viewHelp()
	default:
		return m.viewMain()
	}
}
