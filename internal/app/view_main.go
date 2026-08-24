package app

import (
	"context"
	"fmt"
	"time"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.isFiltering {
			switch msg.String() {
			case "esc":
				m.isFiltering = false
				m.filterInput = ""
				m.selectedIndex = 0
				return m, nil
			case "enter":
				m.isFiltering = false
				return m, nil
			case "backspace":
				if len(m.filterInput) > 0 {
					m.filterInput = m.filterInput[:len(m.filterInput)-1]
					m.selectedIndex = 0
				}
				return m, nil
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
				return m, nil
			case "down", "j":
				list := m.filteredInstances()
				if m.selectedIndex < len(list)-1 {
					m.selectedIndex++
				}
				return m, nil
			default:
				if len(msg.String()) == 1 && msg.Runes != nil && len(msg.Runes) > 0 {
					m.filterInput += msg.String()
					m.selectedIndex = 0
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.mode = ModeHelp
			return m, nil

		case "/":
			m.isFiltering = true
			m.filterInput = ""
			m.selectedIndex = 0
			return m, nil

		case "enter":
			inst := m.selectedInstance()
			if inst != nil {
				m.mode = ModeActionMenu
				m.actionMenuIndex = 0
				return m, nil
			}

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
			list := m.filteredInstances()
			if m.selectedIndex < len(list)-1 {
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
				m.statusMsg = fmt.Sprintf("URI for '%s' copied to clipboard!", inst.Name)
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
				m.statusMsg = fmt.Sprintf("Backend .env block for '%s' copied to clipboard!", inst.Name)
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
		ctx := context.Background()
		var err error
		var actionName string
		if inst.Status == core.StatusReady || inst.Status == core.StatusStarting {
			actionName = "stopped"
			err = m.runner.Stop(ctx, inst)
		} else {
			actionName = "started"
			err = m.runner.Start(ctx, inst)
		}

		if err != nil {
			return errMsg{err}
		}

		inst.Status = m.runner.CheckStatus(ctx, inst)
		if inst.Status != core.StatusStopped {
			inst.MemoryUsage = m.runner.GetMemoryUsage(ctx, inst)
		} else {
			inst.MemoryUsage = "-"
		}

		return actionDoneMsg{
			msg: fmt.Sprintf("Instance '%s' %s successfully!", inst.Name, actionName),
		}
	}
}

func (m *AppModel) purgeInstanceCmd(inst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.runner.DownVolumes(ctx, inst)
		if err != nil {
			return errMsg{err}
		}
		inst.Status = m.runner.CheckStatus(ctx, inst)
		inst.MemoryUsage = "-"
		return actionDoneMsg{
			msg: fmt.Sprintf("Container and volume for '%s' purged successfully!", inst.Name),
		}
	}
}

func (m *AppModel) viewMain() string {
	inner := screenInnerWidth(m.width)
	leftWidth, rightWidth, gapW := splitPanelWidths(inner)
	contentHeight := mainContentHeight(m.height)

	dockerBadge := engineBadge("Docker", m.dockerHealth)
	podmanBadge := engineBadge("Podman", m.podmanHealth)

	header := HeaderStyle.
		Width(inner).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				lipgloss.NewStyle().Bold(true).Foreground(FgText).Background(HeaderBg).Render("LOCAL DATABASE MANAGER"),
				lipgloss.NewStyle().Foreground(MutedColor).Background(HeaderBg).Render("  |  "),
				dockerBadge,
				lipgloss.NewStyle().Foreground(MutedColor).Background(HeaderBg).Render(" "),
				podmanBadge,
			),
		)

	leftInner := panelInnerWidth(leftWidth)
	rightInner := panelInnerWidth(rightWidth)

	var listItems []string
	filteredList := m.filteredInstances()
	inst := m.selectedInstance()

	if m.isFiltering {
		filterBox := FilterBoxStyle.
			Width(leftInner).
			Render(fmt.Sprintf("/ %s|", m.filterInput))
		listItems = append(listItems, filterBox, surfaceBlankLine(leftInner))
	}

	if len(filteredList) == 0 {
		if m.isFiltering {
			listItems = append(listItems, surfaceLine(leftInner, MutedStyle.Render("  No matches found.")))
			listItems = append(listItems, surfaceLine(leftInner, MutedStyle.Render("  Press [Esc] to reset.")))
		} else {
			listItems = append(listItems, surfaceLine(leftInner, NormalItemStyle.Render("  No instances configured.")))
			listItems = append(listItems, surfaceLine(leftInner, MutedStyle.Render("  Press 'n' to create one.")))
		}
	} else {
		for i, item := range filteredList {
			runtimeTag := "Docker"
			if item.Runtime == "podman" {
				runtimeTag = "Podman"
			}

			engineLabel := "Postgres"
			if item.EngineType == "sqlserver" {
				engineLabel = "SQLServer"
			}

			line := renderListLine(item.Status, runtimeTag, engineLabel, item.Name, leftInner, i == m.selectedIndex)
			listItems = append(listItems, line)
		}
	}

	leftTitle := "DB Instances"
	if m.isFiltering {
		leftTitle = fmt.Sprintf("Filter (%d/%d)", len(filteredList), len(m.instances))
	}

	leftBox := panelBoxStyle(true).
		Width(leftWidth).
		Height(contentHeight).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				panelTitle(leftTitle, leftInner),
				lipgloss.JoinVertical(lipgloss.Left, listItems...),
			),
		)

	var rightContent string
	if inst == nil {
		rightContent = surfaceLine(rightInner, MutedStyle.Render("Select an instance from the left list to view details."))
	} else {
		codeBoxWidth := rightInner - 16
		if codeBoxWidth < 20 {
			codeBoxWidth = 20
		}

		details := renderDetailRows(inst, rightWidth)
		for i, row := range details {
			details[i] = surfaceLine(rightInner, row)
		}
		details = append(details, surfaceBlankLine(rightInner))
		details = append(details, surfaceLine(rightInner, detailField("URI:",
			lipgloss.JoinHorizontal(lipgloss.Top,
				URIBoxStyle.Render(truncateMiddle(inst.ConnectionURI(), codeBoxWidth)),
				surfaceGap(1),
				MutedStyle.Render("[c] copy"),
			),
		)))
		details = append(details, surfaceLine(rightInner, detailField("CLI:",
			CLIBoxStyle.Render(truncateMiddle(inst.CLICommand(), codeBoxWidth)),
		)))
		rightContent = lipgloss.JoinVertical(lipgloss.Left, details...)
	}

	rightBox := panelBoxStyle(false).
		Width(rightWidth).
		Height(contentHeight).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				panelTitle("Details & Config", rightInner),
				rightContent,
			),
		)

	panelGap := lipgloss.NewStyle().Background(BgDark).Width(gapW).Render(" ")
	mainSplit := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, panelGap, rightBox)

	statusLine := m.statusMsg
	if statusLine == "" {
		statusLine = "Ready."
	}
	if m.statusIsErr {
		statusLine = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Background(BgSurface).Render(statusLine)
	} else {
		statusLine = lipgloss.NewStyle().Foreground(SecondaryColor).Background(BgSurface).Render(statusLine)
	}

	shortcut := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
	}
	shortcuts := []string{
		shortcut("[↑/↓]", "Nav"),
		shortcut("[Enter]", "Actions"),
		shortcut("[/]", "Search"),
		shortcut("[Space]", "Toggle"),
		shortcut("[c]", "URI"),
		shortcut("[l]", "Logs"),
		shortcut("[d]", "Purge"),
		shortcut("[n]", "New"),
		shortcut("[?]", "Help"),
		shortcut("[q]", "Quit"),
	}
	shortcutsBar := formatShortcutBar(inner-2, shortcuts)

	footerInner := inner - 2
	statusRow := surfaceLine(footerInner, lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(FgText).Background(BgSurface).Render("Status: "),
		statusLine,
	))
	footer := StatusBarStyle.Width(inner).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			statusRow,
			surfaceLine(footerInner, shortcutsBar),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainSplit, footer)
}
