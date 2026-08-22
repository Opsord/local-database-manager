package app

import (
	"fmt"
	"strings"
	"time"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			if m.confirmPurge {
				m.confirmPurge = false
				m.statusMsg = ""
			}
			return m, nil

		case "down", "j":
			if m.selectedIndex < len(m.instances)-1 {
				m.selectedIndex++
			}
			if m.confirmPurge {
				m.confirmPurge = false
				m.statusMsg = ""
			}
			return m, nil

		case " ":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			m.confirmPurge = false
			if inst.Status == core.StatusReady || inst.Status == core.StatusStarting {
				m.statusMsg = fmt.Sprintf("Stopping '%s'...", inst.Name)
			} else {
				m.statusMsg = fmt.Sprintf("Starting '%s'...", inst.Name)
			}
			m.statusIsErr = false
			return m, m.toggleInstanceCmd(inst)

		case "c":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			uri := inst.ConnectionURI()
			if err := core.CopyToClipboard(uri); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
				m.statusIsErr = true
			} else {
				m.statusMsg = fmt.Sprintf("✔ URI for '%s' copied to clipboard!", inst.Name)
				m.statusIsErr = false
			}
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

		case "E", "x":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			block := inst.BackendEnvBlock()
			if err := core.CopyToClipboard(block); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
				m.statusIsErr = true
			} else {
				m.statusMsg = fmt.Sprintf("✔ Backend .env block for '%s' copied to clipboard!", inst.Name)
				m.statusIsErr = false
			}
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

		case "e":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			if err := core.OpenInEditor(inst.EnvFilePath); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to open editor: %v", err)
				m.statusIsErr = true
			} else {
				m.statusMsg = fmt.Sprintf("Opening %s in editor...", inst.Name)
				m.statusIsErr = false
			}
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

		case "d":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			if !m.confirmPurge {
				m.confirmPurge = true
				m.statusMsg = fmt.Sprintf("Purge container and volume for '%s'? Press 'y' to confirm, 'n' to cancel", inst.Name)
				m.statusIsErr = true
				return m, nil
			}

		case "y":
			if m.confirmPurge {
				inst := m.selectedInstance()
				m.confirmPurge = false
				if inst != nil {
					m.statusMsg = fmt.Sprintf("Purging '%s' and removing volume...", inst.Name)
					m.statusIsErr = false
					return m, m.purgeInstanceCmd(inst)
				}
			}

		case "n":
			if m.confirmPurge {
				m.confirmPurge = false
				m.statusMsg = "Action cancelled"
				m.statusIsErr = false
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			}
			// Open Wizard
			m.mode = ModeWizard
			m.wizard = newWizardModel(m.projectRoot, m.instancesDir, m.instances)
			return m, nil

		case "l":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			m.mode = ModeLogs
			return m, m.startLogsCmd(inst)

		case "r":
			m.statusMsg = "Reloading instances and inspecting runtimes..."
			m.statusIsErr = false
			return m, m.reloadInstancesCmd()
		}
	}

	return m, nil
}

func (m *AppModel) toggleInstanceCmd(inst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		var err error
		var actionName string
		if inst.Status == core.StatusReady || inst.Status == core.StatusStarting {
			actionName = "stopped"
			err = m.runner.Stop(inst)
		} else {
			actionName = "started"
			err = m.runner.Start(inst)
		}

		if err != nil {
			return errMsg{err}
		}

		// Update state
		inst.Status = m.runner.CheckStatus(inst)
		if inst.Status != core.StatusStopped {
			inst.MemoryUsage = m.runner.GetMemoryUsage(inst)
		} else {
			inst.MemoryUsage = "-"
		}

		return actionDoneMsg{
			msg: fmt.Sprintf("✔ Instance '%s' %s successfully!", inst.Name, actionName),
		}
	}
}

func (m *AppModel) purgeInstanceCmd(inst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		err := m.runner.DownVolumes(inst)
		if err != nil {
			return errMsg{err}
		}
		inst.Status = m.runner.CheckStatus(inst)
		inst.MemoryUsage = "-"
		return actionDoneMsg{
			msg: fmt.Sprintf("✔ Container and volume for '%s' purged successfully!", inst.Name),
		}
	}
}

func (m *AppModel) viewMain() string {
	leftWidth := m.width/3 - 2
	if leftWidth < 35 {
		leftWidth = 35
	}
	rightWidth := m.width - leftWidth - 6
	if rightWidth < 40 {
		rightWidth = 40
	}
	contentHeight := m.height - 6
	if contentHeight < 15 {
		contentHeight = 15
	}
	if contentHeight > 24 {
		contentHeight = 24
	}

	// 1. Top Header Banner with Engine Health Badges
	dockerStatusStr := StoppedStyle.Render("🔴 Docker: OFFLINE")
	if m.dockerHealth == core.EngineOnline {
		dockerStatusStr = RunningStyle.Render("🟢 Docker: ONLINE")
	} else if m.dockerHealth == core.EngineNotInstalled {
		dockerStatusStr = UnknownStyle.Render("⚪ Docker: NOT INSTALLED")
	}

	podmanStatusStr := StoppedStyle.Render("🔴 Podman: OFFLINE")
	if m.podmanHealth == core.EngineOnline {
		podmanStatusStr = RunningStyle.Render("🟢 Podman: ONLINE")
	} else if m.podmanHealth == core.EngineNotInstalled {
		podmanStatusStr = UnknownStyle.Render("⚪ Podman: NOT INSTALLED")
	}

	header := lipgloss.NewStyle().
		Width(m.width - 2).
		Background(lipgloss.Color("#1E272E")).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("🗄️  LOCAL DATABASE MANAGER"),
				lipgloss.NewStyle().Foreground(MutedColor).Render("   │   "),
				dockerStatusStr,
				lipgloss.NewStyle().Foreground(MutedColor).Render("   "),
				podmanStatusStr,
			),
		)

	// 2. Left Panel (Instances list)
	var listItems []string
	if len(m.instances) == 0 {
		listItems = append(listItems, NormalItemStyle.Render(" No instances configured."))
		listItems = append(listItems, NormalItemStyle.Render(" Press 'n' to create one."))
	} else {
		for i, inst := range m.instances {
			statusIcon := "🔴"
			if inst.Status == core.StatusReady {
				statusIcon = "🟢"
			} else if inst.Status == core.StatusStarting {
				statusIcon = "🟡"
			}

			runtimeTag := "[Docker]"
			if inst.Runtime == "podman" {
				runtimeTag = "[Podman]"
			}

			engineLabel := "Postgres"
			if inst.EngineType == "sqlserver" {
				engineLabel = "SQLServer"
			}

			line := fmt.Sprintf("%s %-8s %-9s : %s", statusIcon, runtimeTag, engineLabel, inst.Name)
			if i == m.selectedIndex {
				line = SelectedItemStyle.Width(leftWidth - 4).Render("> " + line)
			} else {
				line = NormalItemStyle.Render("  " + line)
			}
			listItems = append(listItems, line)
		}
	}

	leftBox := ActivePanelStyle.
		Width(leftWidth).
		Height(contentHeight).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				TitleStyle.Render(" DB Instances "),
				"",
				lipgloss.JoinVertical(lipgloss.Left, listItems...),
			),
		)

	// 3. Right Panel (Details - Compact 2-column key-value grid)
	var rightContent string
	inst := m.selectedInstance()
	if inst == nil {
		rightContent = NormalItemStyle.Render("Select an instance to view its details.")
	} else {
		statusFormatted := StoppedStyle.Render("🔴 STOPPED")
		if inst.Status == core.StatusReady {
			statusFormatted = RunningStyle.Render("🟢 READY")
		} else if inst.Status == core.StatusStarting {
			statusFormatted = StartingStyle.Render("🟡 STARTING")
		} else if inst.Status == core.StatusUnknown {
			statusFormatted = UnknownStyle.Render("🟡 UNKNOWN")
		}

		engineDesc := fmt.Sprintf("%s (%s)", strings.ToUpper(inst.EngineType), strings.ToUpper(inst.Runtime))
		memFormatted := fmt.Sprintf("%s (Limit: %s)", inst.MemoryUsage, inst.MemoryLimit)

		col1W := 32
		col2W := rightWidth - col1W - 8
		if col2W < 20 {
			col2W = 20
		}

		row1 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(col1W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Engine:"), ValueStyle.Render(engineDesc))),
			lipgloss.NewStyle().Width(col2W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Container:"), ValueStyle.Render(inst.ContainerName))),
		)
		row2 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(col1W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Status:"), statusFormatted)),
			lipgloss.NewStyle().Width(col2W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("RAM (Limit):"), ValueStyle.Render(memFormatted))),
		)
		row3 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(col1W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Database:"), ValueStyle.Render(inst.Database))),
			lipgloss.NewStyle().Width(col2W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Host Port:"), ValueStyle.Render(fmt.Sprintf("%d", inst.Port)))),
		)
		row4 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(col1W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("User:"), ValueStyle.Render(inst.User))),
			lipgloss.NewStyle().Width(col2W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Schema:"), ValueStyle.Render(inst.Schema))),
		)
		row5 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(col1W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Volume:"), ValueStyle.Render(inst.Volume))),
			lipgloss.NewStyle().Width(col2W).Render(fmt.Sprintf("%s %s", LabelStyle.Render("Project:"), ValueStyle.Render(inst.ProjectName))),
		)

		details := []string{
			row1,
			row2,
			row3,
			row4,
			row5,
			"",
			fmt.Sprintf("%s %s", LabelStyle.Render("URI:"), URIBoxStyle.Width(rightWidth-18).Render(inst.ConnectionURI())),
			fmt.Sprintf("%s %s", LabelStyle.Render("CLI:"), CLIBoxStyle.Width(rightWidth-18).Render(inst.CLICommand())),
		}
		rightContent = lipgloss.JoinVertical(lipgloss.Left, details...)
	}

	rightBox := PanelStyle.
		Width(rightWidth).
		Height(contentHeight).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				TitleStyle.Render(" Details & Connection "),
				"",
				rightContent,
			),
		)

	mainSplit := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	// 4. Status Bar & Shortcuts
	statusLine := m.statusMsg
	if statusLine == "" {
		statusLine = "Ready."
	}
	if m.statusIsErr {
		statusLine = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render(statusLine)
	} else {
		statusLine = lipgloss.NewStyle().Foreground(SecondaryColor).Render(statusLine)
	}

	shortcuts := []string{
		fmt.Sprintf("%s %s", KeyStyle.Render("[↑/↓]"), KeyDescStyle.Render("Navigate")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[Space]"), KeyDescStyle.Render("Start/Stop")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[c]"), KeyDescStyle.Render("Copy URI")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[E]"), KeyDescStyle.Render("Export .env")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[l]"), KeyDescStyle.Render("Logs")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[n]"), KeyDescStyle.Render("New")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[e]"), KeyDescStyle.Render("Edit .env")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[d]"), KeyDescStyle.Render("Down -v")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[r]"), KeyDescStyle.Render("Reload")),
		fmt.Sprintf("%s %s", KeyStyle.Render("[q]"), KeyDescStyle.Render("Quit")),
	}
	shortcutsBar := strings.Join(shortcuts, "  ")

	footer := StatusBarStyle.Width(m.width - 2).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			fmt.Sprintf("Status: %s", statusLine),
			shortcutsBar,
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainSplit, footer)
}
