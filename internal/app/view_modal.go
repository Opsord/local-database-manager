package app

import (
	"fmt"
	"strings"
	"time"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type actionMenuItem struct {
	label       string
	description string
	shortcut    string
	action      func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd)
}

func (m *AppModel) getActionMenuItems(inst *core.DatabaseInstance) []actionMenuItem {
	toggleLabel := "Start Container"
	if inst.Status == core.StatusReady || inst.Status == core.StatusStarting {
		toggleLabel = "Stop Container"
	}

	return []actionMenuItem{
		{
			label:       toggleLabel,
			description: "Toggle start or stop state for this instance",
			shortcut:    "Space",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				if inst.Status == core.StatusReady || inst.Status == core.StatusStarting {
					m.statusMsg = fmt.Sprintf("Stopping '%s'...", inst.Name)
				} else {
					m.statusMsg = fmt.Sprintf("Starting '%s'...", inst.Name)
				}
				m.statusIsErr = false
				return m, m.toggleInstanceCmd(inst)
			},
		},
		{
			label:       "Copy Connection URI",
			description: "Copy full connection string URL to clipboard",
			shortcut:    "c",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				uri := inst.ConnectionURI()
				if err := core.CopyToClipboard(uri); err != nil {
					m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
					m.statusIsErr = true
				} else {
					m.statusMsg = fmt.Sprintf("URI for '%s' copied to clipboard!", inst.Name)
					m.statusIsErr = false
				}
				return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			},
		},
		{
			label:       "Export Backend .env Block",
			description: "Copy multi-variable configuration block to clipboard",
			shortcut:    "E",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				block := inst.BackendEnvBlock()
				if err := core.CopyToClipboard(block); err != nil {
					m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
					m.statusIsErr = true
				} else {
					m.statusMsg = fmt.Sprintf("Backend .env block for '%s' copied to clipboard!", inst.Name)
					m.statusIsErr = false
				}
				return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			},
		},
		{
			label:       "View Live Logs",
			description: "Stream realtime stdout/stderr logs from container",
			shortcut:    "l",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeLogs
				return m, m.startLogsCmd(inst)
			},
		},
		{
			label:       "Edit Instance",
			description: "Edit instance settings in the docked wizard",
			shortcut:    "",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeWizard
				m.wizard = newEditWizardModel(m.projectRoot, m.instancesDir, m.instances, inst)
				return m, nil
			},
		},
		{
			label:       "Purge Volume & Reset (Down -v)",
			description: "Completely delete container and all associated data volumes",
			shortcut:    "d",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				m.clearConfirms()
				m.confirmPurge = true
				m.statusMsg = fmt.Sprintf("Purge container and volume for '%s'? Press 'y' to confirm, 'n' to cancel", inst.Name)
				m.statusIsErr = true
				return m, nil
			},
		},
		{
			label:       "Delete Instance",
			description: "Purge container+volume and remove the instance .env from the list",
			shortcut:    "D",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				m.clearConfirms()
				m.confirmDelete = true
				m.statusMsg = fmt.Sprintf("Delete instance '%s'? This purges container+volume and removes the .env. Press 'y' to confirm, 'n' to cancel", inst.Name)
				m.statusIsErr = true
				return m, nil
			},
		},
	}
}

func (m *AppModel) updateActionMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	inst := m.selectedInstance()
	if inst == nil {
		m.mode = ModeMain
		return m, nil
	}

	items := m.getActionMenuItems(inst)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			m.mode = ModeMain
			return m, nil

		case "up", "k":
			if m.actionMenuIndex > 0 {
				m.actionMenuIndex--
			} else {
				m.actionMenuIndex = len(items) - 1
			}
			return m, nil

		case "down", "j":
			if m.actionMenuIndex < len(items)-1 {
				m.actionMenuIndex++
			} else {
				m.actionMenuIndex = 0
			}
			return m, nil

		case "enter":
			if m.actionMenuIndex >= 0 && m.actionMenuIndex < len(items) {
				return items[m.actionMenuIndex].action(m, inst)
			}
		default:
			for _, item := range items {
				if actionMenuItemMatchesKey(item, msg.String()) {
					return item.action(m, inst)
				}
			}
		}
	}

	return m, nil
}

func actionMenuItemMatchesKey(item actionMenuItem, key string) bool {
	if strings.EqualFold(item.shortcut, "Space") {
		return key == " "
	}
	return item.shortcut == key
}

func (m *AppModel) viewActionDock(innerWidth, dockHeight int) string {
	inst := m.selectedInstance()
	if inst == nil {
		return ""
	}

	items := m.getActionMenuItems(inst)
	title := surfaceLine(innerWidth, TitleStyle.Render(fmt.Sprintf("Actions: %s (%s)", inst.Name, inst.EngineType)))
	hints := surfaceLine(innerWidth, MutedStyle.Render("Use [↑/↓] to navigate, [Enter] to execute, [Esc] to return."))

	bodyHeight := dockHeight - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	// Styled label + muted description (like Details LabelStyle). Every fragment
	// shares the row background so SGR resets do not punch black holes.
	var rowLines []string
	for i, item := range items {
		selected := i == m.actionMenuIndex
		bg := BgSurface
		if selected {
			bg = SelectedBg
		}
		gap := lipgloss.NewStyle().Background(bg).Render("  ")
		prefix := "  "
		if selected {
			prefix = "> "
		}
		label := lipgloss.NewStyle().Bold(true).Foreground(FgText).Background(bg).Render(prefix + item.label)
		desc := lipgloss.NewStyle().Foreground(MutedColor).Background(bg).Render("— " + item.description)
		parts := []string{label}
		if item.shortcut != "" {
			badge := lipgloss.NewStyle().Foreground(AccentColor).Background(bg).Render(fmt.Sprintf("[%s]", item.shortcut))
			parts = append(parts, badge)
		}
		parts = append(parts, desc)
		content := parts[0]
		for _, p := range parts[1:] {
			content = lipgloss.JoinHorizontal(lipgloss.Top, content, gap, p)
		}
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

func (m *AppModel) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter", "?", "ctrl+c":
			m.mode = ModeMain
			return m, nil
		}
	}
	return m, nil
}

func (m *AppModel) viewHelp() string {
	modalWidth := m.width - 10
	if modalWidth < 55 {
		modalWidth = 55
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	var content []string
	content = append(content, TitleStyle.Render("Keyboard Shortcuts & Guide"))
	content = append(content, SeparatorStyle.Render(strings.Repeat("─", modalWidth-4)))
	content = append(content, surfaceGap(1))

	helpItems := [][]string{
		{"[↑ / ↓] / [k / j]", "Navigate through database instances"},
		{"[Enter]", "Dock Action Menu in bottom-right panel (Start/Stop, Copy URI, Logs, Edit Instance, Purge, Delete Instance)"},
		{"[Edit Instance]", "Docked wizard (from Action Menu); [o] opens external editor"},
		{"[y / n]", "Confirm restart after saving edits to a running instance"},
		{"[/]", "Live Search / Filter instances in real-time"},
		{"[Space]", "Start or Stop container instance"},
		{"[c]", "Copy connection URI to clipboard"},
		{"[E / x]", "Copy backend .env configuration block to clipboard"},
		{"[l]", "Open live log streamer for selected container"},
		{"[n]", "Create new database instance with step-by-step wizard"},
		{"[e]", "Dock Engines panel (left) — Start offline; Stop online (y/n confirm)"},
		{"[d]", "Purge instance container and wipe persistent volume (down -v); keeps the .env definition"},
		{"[D]", "Delete instance: purge container+volume and remove the .env from the list"},
		{"[r]", "Reload instances & recheck Docker / Podman daemon health"},
		{"[?]", "Toggle this help reference screen"},
		{"[q / Ctrl+C]", "Quit application"},
	}

	for _, item := range helpItems {
		keyStr := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Width(20).Background(BgSurface).Render(item[0])
		descStr := lipgloss.NewStyle().Foreground(FgText).Background(BgSurface).Render(item[1])
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, surfaceGap(2), keyStr, surfaceGap(1), descStr))
	}

	content = append(content, surfaceGap(1))
	content = append(content, SubTitleStyle.Render("Database Drivers & Tools:"))
	content = append(content, MutedStyle.Render("  • Postgres: psql, pgAdmin, DBeaver, Prisma, TypeORM, GORM"))
	content = append(content, MutedStyle.Render("  • SQL Server: sqlcmd, SSMS, Azure Data Studio, EF Core"))
	content = append(content, surfaceGap(1))
	content = append(content, MutedStyle.Render("Press [Esc], [?], or [Enter] to return."))

	return ActivePanelStyle.
		Width(modalWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
