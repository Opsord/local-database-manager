package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
				m.clearCopiedHit()
				return m, nil
			case "enter":
				m.isFiltering = false
				return m, nil
			case "backspace":
				if len(m.filterInput) > 0 {
					m.filterInput = m.filterInput[:len(m.filterInput)-1]
					m.selectedIndex = 0
					m.clearCopiedHit()
				}
				return m, nil
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.clearCopiedHit()
				}
				return m, nil
			case "down", "j":
				list := m.filteredInstances()
				if m.selectedIndex < len(list)-1 {
					m.selectedIndex++
					m.clearCopiedHit()
				}
				return m, nil
			default:
				if len(msg.String()) == 1 && msg.Runes != nil && len(msg.Runes) > 0 {
					m.filterInput += msg.String()
					m.selectedIndex = 0
					m.clearCopiedHit()
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
			m.clearCopiedHit()
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
				m.clearCopiedHit()
			}
			if m.confirmPurge || m.confirmDelete || m.confirmEngineStart || m.confirmEngineStop || m.confirmRestartAfterEdit {
				m.clearConfirms()
				m.statusMsg = ""
			}
			return m, nil

		case "down", "j":
			list := m.filteredInstances()
			if m.selectedIndex < len(list)-1 {
				m.selectedIndex++
				m.clearCopiedHit()
			}
			if m.confirmPurge || m.confirmDelete || m.confirmEngineStart || m.confirmEngineStop || m.confirmRestartAfterEdit {
				m.clearConfirms()
				m.statusMsg = ""
			}
			return m, nil

		case " ":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			m.clearConfirms()
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
			if m.engineStarting {
				return m, nil
			}
			m.clearConfirms()
			m.mode = ModeEngineMenu
			m.engineMenuIndex = 0
			return m, nil

		case "d":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			if !m.confirmPurge {
				m.clearConfirms()
				m.confirmPurge = true
				m.statusMsg = fmt.Sprintf("Purge container and volume for '%s'? Press 'y' to confirm, 'n' to cancel", inst.Name)
				m.statusIsErr = true
				return m, nil
			}

		case "D":
			inst := m.selectedInstance()
			if inst == nil {
				return m, nil
			}
			if !m.confirmDelete {
				m.clearConfirms()
				m.confirmDelete = true
				m.statusMsg = fmt.Sprintf("Delete instance '%s'? This purges container+volume and removes the .env. Press 'y' to confirm, 'n' to cancel", inst.Name)
				m.statusIsErr = true
				return m, nil
			}

		case "y":
			if m.confirmRestartAfterEdit {
				old := m.pendingRestartOld
				newName := m.pendingRestartNewName
				m.confirmRestartAfterEdit = false
				m.pendingRestartOld = nil
				m.pendingRestartNewName = ""
				m.statusMsg = fmt.Sprintf("Restarting '%s'...", newName)
				m.statusIsErr = false
				return m, m.restartAfterEditCmd(old, newName)
			}
			if m.confirmEngineStop {
				if m.engineStarting {
					return m, nil
				}
				return m, m.confirmEngineStopYes()
			}
			if m.confirmEngineStart {
				if m.engineStarting {
					return m, nil
				}
				runtime := m.pendingEngineRuntime
				inst := m.pendingStartInst
				m.confirmEngineStart = false
				m.pendingEngineRuntime = ""
				m.pendingStartInst = nil
				m.engineStarting = true
				m.statusMsg = engineStartStatusMsg(runtime)
				m.statusIsErr = false
				return m, m.startEngineCmd(runtime, inst)
			}
			if m.confirmPurge {
				inst := m.selectedInstance()
				m.confirmPurge = false
				if inst != nil {
					m.statusMsg = fmt.Sprintf("Purging '%s' and removing volume...", inst.Name)
					m.statusIsErr = false
					return m, m.purgeInstanceCmd(inst)
				}
			}
			if m.confirmDelete {
				inst := m.selectedInstance()
				m.confirmDelete = false
				if inst != nil {
					m.statusMsg = fmt.Sprintf("Deleting '%s'...", inst.Name)
					m.statusIsErr = false
					return m, m.deleteInstanceCmd(inst)
				}
			}

		case "n":
			if m.confirmRestartAfterEdit {
				m.confirmRestartAfterEdit = false
				m.pendingRestartOld = nil
				m.pendingRestartNewName = ""
				m.finishEditRenameCleanup()
				m.statusMsg = "Restart cancelled"
				m.statusIsErr = false
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			}
			if m.confirmEngineStop {
				return m, m.cancelEngineStopConfirm()
			}
			if m.confirmEngineStart {
				m.confirmEngineStart = false
				m.pendingStartInst = nil
				m.pendingEngineRuntime = ""
				m.statusMsg = "Action cancelled"
				m.statusIsErr = false
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			}
			if m.confirmPurge {
				m.confirmPurge = false
				m.statusMsg = "Action cancelled"
				m.statusIsErr = false
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			}
			if m.confirmDelete {
				m.confirmDelete = false
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
			return m, tea.Batch(m.reloadInstancesCmd(), m.checkEngineHealthCmd(false))
		}
	}

	return m, nil
}

type offlineStartMsg struct {
	inst *core.DatabaseInstance
	err  error
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
			if actionName == "started" && errors.Is(err, core.ErrEngineOffline) {
				return offlineStartMsg{inst: inst, err: err}
			}
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

func (m *AppModel) restartAfterEditCmd(old *core.DatabaseInstance, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		stopErr := m.runner.Stop(ctx, old)
		newInst, err := core.ParseEnvFile(filepath.Join(m.instancesDir, newName+".env"))
		if err != nil {
			return restartAfterEditDoneMsg{name: newName, err: err}
		}
		startErr := m.runner.Start(ctx, newInst)
		if startErr != nil {
			if stopErr != nil {
				return restartAfterEditDoneMsg{name: newName, err: fmt.Errorf("stop: %v; start: %v", stopErr, startErr)}
			}
			return restartAfterEditDoneMsg{name: newName, err: startErr}
		}
		return restartAfterEditDoneMsg{name: newName}
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

func (m *AppModel) deleteInstanceCmd(inst *core.DatabaseInstance) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		runtime := inst.Runtime
		if runtime == "" {
			runtime = "docker"
		}
		health := m.runner.CheckEngineHealth(ctx, runtime)
		if health == core.EngineNotInstalled {
			return errMsg{fmt.Errorf("%w: %s — start is unavailable; cannot delete", core.ErrEngineNotInstalled, runtime)}
		}
		if health == core.EngineOffline {
			return errMsg{fmt.Errorf("%w: %s is offline — start it from Engines (e) before deleting", core.ErrEngineOffline, runtime)}
		}
		if err := m.runner.DownVolumes(ctx, inst); err != nil {
			return errMsg{err}
		}
		if err := os.Remove(inst.EnvFilePath); err != nil && !os.IsNotExist(err) {
			return errMsg{err}
		}
		return deleteDoneMsg{name: inst.Name}
	}
}

func (m *AppModel) buildRightDetailsContent(rightInner, rightWidth int) string {
	inst := m.selectedInstance()
	if inst == nil {
		return surfaceLine(rightInner, MutedStyle.Render("Select an instance from the left list to view details."))
	}
	codeBoxWidth := rightInner - 16
	if codeBoxWidth < 20 {
		codeBoxWidth = 20
	}

	ox, oy, _ := m.detailsContentOrigin()
	details := renderDetailRowsWithCopiedHit(inst, rightWidth, ox, oy, m.copiedHit)
	fields := plainDetailFields(inst, rightWidth)
	if rightWidth < 70 {
		oy += len(fields)
	} else {
		oy += (len(fields) + 1) / 2
	}
	oy++ // blank row before URI/CLI
	for i, row := range details {
		details[i] = surfaceLine(rightInner, row)
	}
	details = append(details, surfaceBlankLine(rightInner))
	uriPlain := truncateMiddle(inst.ConnectionURI(), codeBoxWidth)
	uriValue := styleValueWithCopiedHit(uriPlain, valueOriginX(ox), oy, URIBoxStyle, m.copiedHit)
	details = append(details, surfaceLine(rightInner, detailField("URI:",
		lipgloss.JoinHorizontal(lipgloss.Top,
			uriValue,
			surfaceGap(1),
			MutedStyle.Render("[c] copy"),
		),
	)))
	oy++
	cliPlain := truncateMiddle(inst.CLICommand(), codeBoxWidth)
	cliValue := styleValueWithCopiedHit(cliPlain, valueOriginX(ox), oy, CLIBoxStyle, m.copiedHit)
	details = append(details, surfaceLine(rightInner, detailField("CLI:", cliValue)))
	return lipgloss.JoinVertical(lipgloss.Left, details...)
}

func mainShortcutEntries() []string {
	shortcut := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
	}
	return []string{
		shortcut("[↑/↓]", "Nav"),
		shortcut("[Enter]", "Actions"),
		shortcut("[/]", "Search"),
		shortcut("[Space]", "Toggle"),
		shortcut("[e]", "Engines"),
		shortcut("[c]", "URI"),
		shortcut("[l]", "Logs"),
		shortcut("[d]", "Purge"),
		shortcut("[D]", "Delete"),
		shortcut("[n]", "New"),
		shortcut("[?]", "Help"),
		shortcut("[q]", "Quit"),
	}
}

func actionShortcutEntries() []string {
	shortcut := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
	}
	return []string{
		shortcut("[↑↓]", "Nav"),
		shortcut("[Enter]", "Run"),
		shortcut("[Esc]", "Close"),
	}
}

func wizardShortcutEntries() []string {
	shortcut := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
	}
	return []string{
		shortcut("[↑↓]", "Rows"),
		shortcut("[←→]", "Options"),
		shortcut("[Enter]", "Next"),
		shortcut("[b]", "Back"),
		shortcut("[Esc]", "Cancel"),
	}
}

func engineShortcutEntries() []string {
	shortcut := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
	}
	return []string{
		shortcut("[↑↓]", "Nav"),
		shortcut("[Enter]", "start/stop"),
		shortcut("[y/n]", "confirm stop"),
		shortcut("[Esc]", "Close"),
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

	var leftColumn string
	if m.mode == ModeEngineMenu {
		listH, engH := splitPanelHalfHeight(contentHeight - 1)
		listBlock := lipgloss.NewStyle().
			Width(leftInner).
			Height(listH).
			MaxHeight(listH).
			Background(BgSurface).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				panelTitle(leftTitle, leftInner),
				lipgloss.JoinVertical(lipgloss.Left, listItems...),
			))
		engBlock := m.viewEngineDock(leftInner, engH)
		leftColumn = ActivePanelStyle.
			Width(leftWidth).
			Height(contentHeight).
			Render(lipgloss.JoinVertical(lipgloss.Left, listBlock, panelSeparator(leftInner), engBlock))
	} else {
		leftColumn = panelBoxStyle(m.mode != ModeWizard && m.mode != ModeActionMenu).
			Width(leftWidth).
			Height(contentHeight).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					panelTitle(leftTitle, leftInner),
					lipgloss.JoinVertical(lipgloss.Left, listItems...),
				),
			)
	}

	detailsContent := m.buildRightDetailsContent(rightInner, rightWidth)

	var rightColumn string
	if m.mode == ModeWizard || m.mode == ModeActionMenu {
		// One bordered panel for the whole right column. Two stacked bordered
		// panels add an extra pair of borders and make the right side taller
		// than the left, which shifts the layout and clips the header title.
		detailsH, dockH := splitPanelHalfHeight(contentHeight - 1) // -1 for separator
		detailsBlock := lipgloss.NewStyle().
			Width(rightInner).
			Height(detailsH).
			MaxHeight(detailsH).
			Background(BgSurface).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				panelTitle("Details & Config", rightInner),
				detailsContent,
			))
		var dockBlock string
		if m.mode == ModeWizard {
			dockBlock = m.viewWizardDock(rightInner, dockH)
		} else {
			dockBlock = m.viewActionDock(rightInner, dockH)
		}
		rightInnerCol := lipgloss.JoinVertical(
			lipgloss.Left,
			detailsBlock,
			panelSeparator(rightInner),
			dockBlock,
		)
		rightColumn = ActivePanelStyle.
			Width(rightWidth).
			Height(contentHeight).
			Render(rightInnerCol)
	} else {
		rightColumn = panelBoxStyle(false).
			Width(rightWidth).
			Height(contentHeight).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					panelTitle("Details & Config", rightInner),
					detailsContent,
				),
			)
	}

	panelGap := lipgloss.NewStyle().Background(BgDark).Width(gapW).Render(" ")
	mainSplit := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, panelGap, rightColumn)

	statusLine := m.statusMsg
	if statusLine == "" {
		statusLine = "Ready."
	}
	if m.statusIsErr {
		statusLine = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Background(BgSurface).Render(statusLine)
	} else {
		statusLine = lipgloss.NewStyle().Foreground(SecondaryColor).Background(BgSurface).Render(statusLine)
	}

	shortcuts := mainShortcutEntries()
	switch m.mode {
	case ModeWizard:
		shortcuts = wizardShortcutEntries()
	case ModeActionMenu:
		shortcuts = actionShortcutEntries()
	case ModeEngineMenu:
		shortcuts = engineShortcutEntries()
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
