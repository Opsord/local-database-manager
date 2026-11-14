package app

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type engineStartedMsg struct {
	runtime   string
	err       error
	retryInst *core.DatabaseInstance
}

type engineStoppedMsg struct {
	runtime string
	err     error
}

type engineMenuRow struct {
	runtime    string
	label      string
	actionable bool
	op         string // "start" | "stop"
}

func (m *AppModel) engineMenuRows() []engineMenuRow {
	return []engineMenuRow{
		engineRow("docker", m.dockerHealth),
		engineRow("podman", m.podmanHealth),
	}
}

func engineRow(runtime string, h core.EngineHealth) engineMenuRow {
	name := "Docker"
	if runtime == "podman" {
		name = "Podman"
	}
	switch h {
	case core.EngineOffline:
		return engineMenuRow{runtime: runtime, label: "Start " + name, actionable: true, op: "start"}
	case core.EngineOnline:
		return engineMenuRow{runtime: runtime, label: "Stop " + name, actionable: true, op: "stop"}
	default:
		return engineMenuRow{runtime: runtime, label: name + ": not installed", actionable: false}
	}
}

func (m *AppModel) startEngineCmd(runtime string, retryInst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		// #region agent log
		core.AgentDebugLog("E", "view_engine.go:startEngineCmd", "invoked", map[string]any{"runtime": runtime})
		// #endregion
		err := m.runner.StartEngine(context.Background(), runtime)
		// #region agent log
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		core.AgentDebugLog("E", "view_engine.go:startEngineCmd", "finished", map[string]any{"runtime": runtime, "err": errStr})
		// #endregion
		return engineStartedMsg{runtime: runtime, err: err, retryInst: retryInst}
	}
}

func (m *AppModel) stopEngineCmd(runtime string) tea.Cmd {
	return func() tea.Msg {
		err := m.runner.StopEngine(context.Background(), runtime)
		return engineStoppedMsg{runtime: runtime, err: err}
	}
}

func engineStartStatusMsg(engineRuntime string) string {
	if engineRuntime == "podman" {
		return "Starting Podman machine..."
	}
	return "Starting Docker..."
}

func engineStopStatusMsg(engineRuntime string) string {
	if engineRuntime == "podman" {
		return "Stopping Podman machine..."
	}
	if runtime.GOOS == "windows" {
		return "Stopping Docker Desktop..."
	}
	return "Stopping Docker..."
}

func engineDisplayName(engineRuntime string) string {
	if engineRuntime == "podman" {
		return "Podman"
	}
	return "Docker"
}

func (m *AppModel) armEngineStopConfirm(row engineMenuRow) {
	m.clearConfirms()
	m.confirmEngineStop = true
	m.pendingStopRuntime = row.runtime
	m.statusMsg = fmt.Sprintf("Stop %s? Press 'y' to confirm, 'n' to cancel", engineDisplayName(row.runtime))
	m.statusIsErr = true
}

func (m *AppModel) confirmEngineStopYes() tea.Cmd {
	runtime := m.pendingStopRuntime
	m.confirmEngineStop = false
	m.pendingStopRuntime = ""
	m.engineStarting = true
	m.statusMsg = engineStopStatusMsg(runtime)
	m.statusIsErr = false
	return m.stopEngineCmd(runtime)
}

func (m *AppModel) cancelEngineStopConfirm() tea.Cmd {
	m.confirmEngineStop = false
	m.pendingStopRuntime = ""
	m.statusMsg = "Action cancelled"
	m.statusIsErr = false
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
}

func (m *AppModel) updateEngineMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	rows := m.engineMenuRows()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			m.clearConfirms()
			m.mode = ModeMain
			return m, nil

		case "up", "k":
			if m.confirmEngineStop {
				m.clearConfirms()
				m.statusMsg = ""
			}
			if m.engineMenuIndex > 0 {
				m.engineMenuIndex--
			} else {
				m.engineMenuIndex = len(rows) - 1
			}
			return m, nil

		case "down", "j":
			if m.confirmEngineStop {
				m.clearConfirms()
				m.statusMsg = ""
			}
			if m.engineMenuIndex < len(rows)-1 {
				m.engineMenuIndex++
			} else {
				m.engineMenuIndex = 0
			}
			return m, nil

		case "y":
			if m.confirmEngineStop {
				if m.engineStarting {
					return m, nil
				}
				return m, m.confirmEngineStopYes()
			}

		case "n":
			if m.confirmEngineStop {
				return m, m.cancelEngineStopConfirm()
			}

		case "enter":
			if m.engineStarting {
				return m, nil
			}
			if m.engineMenuIndex >= 0 && m.engineMenuIndex < len(rows) {
				row := rows[m.engineMenuIndex]
				if !row.actionable {
					return m, nil
				}
				if row.op == "stop" {
					m.armEngineStopConfirm(row)
					return m, nil
				}
				m.engineStarting = true
				m.statusMsg = engineStartStatusMsg(row.runtime)
				m.statusIsErr = false
				m.mode = ModeMain
				return m, m.startEngineCmd(row.runtime, nil)
			}
		}
	}

	return m, nil
}

func (m *AppModel) viewEngineDock(innerWidth, dockHeight int) string {
	rows := m.engineMenuRows()

	title := panelTitle("Container Engines", innerWidth)
	hints := surfaceLine(innerWidth, MutedStyle.Render("Use [↑/↓] to navigate, [Enter] to start/stop, [y/n] to confirm stop, [Esc] to return."))

	bodyHeight := dockHeight - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var rowLines []string
	for i, row := range rows {
		selected := i == m.engineMenuIndex
		bg := BgSurface
		if selected {
			bg = SelectedBg
		}
		prefix := "  "
		if selected {
			prefix = "> "
		}
		labelStyle := lipgloss.NewStyle().Foreground(MutedColor).Background(bg)
		if row.actionable {
			labelStyle = lipgloss.NewStyle().Bold(true).Foreground(FgText).Background(bg)
		}
		content := labelStyle.Render(prefix + row.label)
		rowLines = append(rowLines, lipgloss.NewStyle().
			Width(innerWidth).
			MaxWidth(innerWidth).
			Background(bg).
			Render(content))
	}

	body := lipgloss.NewStyle().
		Width(innerWidth).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Background(BgSurface).
		Render(lipgloss.JoinVertical(lipgloss.Left, rowLines...))

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hints)
}
