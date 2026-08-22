package app

import (
	"fmt"
	"path/filepath"
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
)

type clearStatusMsg struct{}

type actionDoneMsg struct {
	msg string
}

// AppModel is the root Bubble Tea application state.
type AppModel struct {
	projectRoot  string
	instancesDir string
	runner       *core.Runner

	mode          AppMode
	instances     []*core.DatabaseInstance
	selectedIndex int

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
		dockerHealth := m.runner.CheckEngineHealth("docker")
		podmanHealth := m.runner.CheckEngineHealth("podman")

		instances, err := core.ScanInstances(m.instancesDir)
		if err != nil {
			return errMsg{err}
		}

		// Inspect current status and stats of all instances
		for _, inst := range instances {
			inst.Status = m.runner.CheckStatus(inst)
			if inst.Status != core.StatusStopped {
				inst.MemoryUsage = m.runner.GetMemoryUsage(inst)
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

		if m.selectedIndex >= len(m.instances) {
			m.selectedIndex = len(m.instances) - 1
		}
		if m.selectedIndex < 0 && len(m.instances) > 0 {
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
	default:
		return m.viewMain()
	}
}

func (m *AppModel) selectedInstance() *core.DatabaseInstance {
	if len(m.instances) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.instances) {
		return nil
	}
	return m.instances[m.selectedIndex]
}
