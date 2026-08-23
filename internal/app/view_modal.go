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
			label:       "Edit .env Configuration",
			description: "Open instance environment file in default editor",
			shortcut:    "e",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				if err := core.OpenInEditor(inst.EnvFilePath); err != nil {
					m.statusMsg = fmt.Sprintf("Failed to open editor: %v", err)
					m.statusIsErr = true
				} else {
					m.statusMsg = fmt.Sprintf("Opening %s in editor...", inst.Name)
					m.statusIsErr = false
				}
				return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
			},
		},
		{
			label:       "Purge Volume & Reset (Down -v)",
			description: "Completely delete container and all associated data volumes",
			shortcut:    "d",
			action: func(m *AppModel, inst *core.DatabaseInstance) (tea.Model, tea.Cmd) {
				m.mode = ModeMain
				m.confirmPurge = true
				m.statusMsg = fmt.Sprintf("Purge container and volume for '%s'? Press 'y' to confirm, 'n' to cancel", inst.Name)
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
		}
	}

	return m, nil
}

func (m *AppModel) viewActionMenu() string {
	inst := m.selectedInstance()
	if inst == nil {
		return ""
	}

	items := m.getActionMenuItems(inst)
	modalWidth := m.width - 12
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalWidth > 75 {
		modalWidth = 75
	}

	var content []string
	content = append(content, TitleStyle.Render(fmt.Sprintf("Actions: %s (%s)", inst.Name, inst.EngineType)))
	content = append(content, SeparatorStyle.Render(strings.Repeat("─", modalWidth-4)))
	content = append(content, surfaceGap(1))

	for i, item := range items {
		shortcutBadge := lipgloss.NewStyle().Foreground(AccentColor).Background(BgSurface).Render(fmt.Sprintf("[%s]", item.shortcut))
		labelStr := lipgloss.NewStyle().Bold(true).Foreground(FgText).Background(BgSurface).Render(item.label)
		descStr := MutedStyle.Render(" — " + item.description)
		itemText := fmt.Sprintf("%s  %s  %s", labelStr, shortcutBadge, descStr)

		var line string
		if i == m.actionMenuIndex {
			line = SelectedItemStyle.Width(modalWidth - 4).Render("> " + itemText)
		} else {
			line = NormalItemStyle.Render("  " + itemText)
		}
		content = append(content, line)
	}

	content = append(content, surfaceGap(1))
	content = append(content, MutedStyle.Render("Use [↑/↓] to navigate, [Enter] to execute, [Esc] to return."))

	return ActivePanelStyle.
		Width(modalWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
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
		{"[Enter]", "Open Action Menu (Command Palette) for selected instance"},
		{"[/]", "Live Search / Filter instances in real-time"},
		{"[Space]", "Start or Stop container instance"},
		{"[c]", "Copy connection URI to clipboard"},
		{"[E / x]", "Copy backend .env configuration block to clipboard"},
		{"[l]", "Open live log streamer for selected container"},
		{"[n]", "Create new database instance with step-by-step wizard"},
		{"[e]", "Edit .env file in default system editor (VS Code, Notepad, etc.)"},
		{"[d]", "Purge instance container and wipe persistent volume (down -v)"},
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
