package app

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Opsord/local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logLineMsg struct {
	streamID uint64
	line     string
}
type logClosedMsg struct {
	streamID uint64
	err      error
}
type clearLogStatusMsg struct{}

type logsModel struct {
	viewport    viewport.Model
	cmd         *exec.Cmd
	scanner     *bufio.Scanner
	streamID    uint64
	lines       []string
	instName    string
	autoScroll  bool
	isFiltering bool
	filterInput string
	statusMsg   string
}

func (m *logsModel) visibleLines() []string {
	query := strings.ToLower(strings.TrimSpace(m.filterInput))
	if query == "" {
		return m.lines
	}
	var out []string
	for _, l := range m.lines {
		if strings.Contains(strings.ToLower(l), query) {
			out = append(out, l)
		}
	}
	return out
}

func (m *logsModel) refreshViewportContent() {
	vis := m.visibleLines()
	if len(vis) == 0 && strings.TrimSpace(m.filterInput) != "" {
		m.viewport.SetContent("(No matching log lines)")
	} else {
		m.viewport.SetContent(strings.Join(vis, "\n"))
	}
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

func (m *AppModel) startLogsCmd(inst *core.DatabaseInstance) tea.Cmd {
	if m.logs.cmd != nil && m.logs.cmd.Process != nil {
		_ = m.logs.cmd.Process.Kill()
	}
	m.logs.streamID++
	m.logs.cmd = nil
	m.logs.scanner = nil
	m.logs.lines = []string{fmt.Sprintf("Starting logs stream for %s (%s)...", inst.Name, inst.ContainerName)}
	m.logs.instName = inst.Name
	m.logs.autoScroll = true
	m.logs.isFiltering = false
	m.logs.filterInput = ""
	m.logs.statusMsg = ""
	m.logs.refreshViewportContent()

	cmd := m.runner.LogsCommand(inst)
	if cmd == nil {
		m.logs.lines = append(m.logs.lines, "Error: logs command is not available")
		m.logs.refreshViewportContent()
		return nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.logs.lines = append(m.logs.lines, fmt.Sprintf("Error opening stdout pipe: %v", err))
		m.logs.refreshViewportContent()
		return nil
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		m.logs.lines = append(m.logs.lines, fmt.Sprintf("Error starting logs command: %v", err))
		m.logs.refreshViewportContent()
		return nil
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	m.logs.cmd = cmd
	m.logs.scanner = scanner
	return waitForLogLines(m.logs.streamID, m.logs.scanner, m.logs.cmd)
}

func waitForLogLines(streamID uint64, scanner *bufio.Scanner, cmd *exec.Cmd) tea.Cmd {
	if scanner == nil {
		return nil
	}
	return func() tea.Msg {
		if scanner.Scan() {
			return logLineMsg{streamID: streamID, line: scanner.Text()}
		}
		err := scanner.Err()
		if cmd != nil {
			_ = cmd.Wait()
		}
		return logClosedMsg{streamID: streamID, err: err}
	}
}

func (m *AppModel) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearLogStatusMsg:
		m.logs.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		if m.logs.isFiltering {
			switch msg.String() {
			case "esc":
				m.logs.isFiltering = false
				m.logs.filterInput = ""
				m.logs.refreshViewportContent()
				return m, nil
			case "enter":
				m.logs.isFiltering = false
				m.logs.refreshViewportContent()
				return m, nil
			case "backspace":
				if len(m.logs.filterInput) > 0 {
					m.logs.filterInput = m.logs.filterInput[:len(m.logs.filterInput)-1]
					m.logs.refreshViewportContent()
				}
				return m, nil
			default:
				if len(msg.String()) == 1 && msg.Runes != nil && len(msg.Runes) > 0 {
					m.logs.filterInput += msg.String()
					m.logs.refreshViewportContent()
					return m, nil
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if m.logs.filterInput != "" {
				m.logs.filterInput = ""
				m.logs.refreshViewportContent()
				return m, nil
			}
			if m.logs.cmd != nil && m.logs.cmd.Process != nil {
				_ = m.logs.cmd.Process.Kill()
			}
			m.logs.streamID++
			m.logs.cmd = nil
			m.logs.scanner = nil
			m.mode = ModeMain
			return m, nil

		case "/":
			m.logs.isFiltering = true
			m.logs.filterInput = ""
			m.logs.refreshViewportContent()
			return m, nil

		case "f", " ":
			m.logs.autoScroll = !m.logs.autoScroll
			if m.logs.autoScroll {
				m.logs.viewport.GotoBottom()
			}
			return m, nil

		case "c", "y":
			vis := m.logs.visibleLines()
			if len(vis) == 0 {
				m.logs.statusMsg = "No lines to copy"
			} else {
				content := strings.Join(vis, "\n")
				if err := core.CopyToClipboard(content); err != nil {
					m.logs.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
				} else {
					m.logs.statusMsg = fmt.Sprintf("Copied %d lines to clipboard!", len(vis))
				}
			}
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearLogStatusMsg{} })

		case "up", "pgup":
			m.logs.autoScroll = false
		}

	case logLineMsg:
		if msg.streamID != m.logs.streamID {
			return m, nil
		}
		m.logs.lines = append(m.logs.lines, msg.line)
		if len(m.logs.lines) > 1000 {
			m.logs.lines = m.logs.lines[len(m.logs.lines)-1000:]
		}
		m.logs.refreshViewportContent()
		if m.logs.scanner != nil {
			return m, waitForLogLines(m.logs.streamID, m.logs.scanner, m.logs.cmd)
		}
		return m, nil

	case logClosedMsg:
		if msg.streamID != m.logs.streamID {
			return m, nil
		}
		m.logs.scanner = nil
		m.logs.cmd = nil
		if msg.err != nil {
			m.logs.lines = append(m.logs.lines, fmt.Sprintf("Log stream error: %v", msg.err))
		}
		m.logs.lines = append(m.logs.lines, "--- End of logs stream ---")
		m.logs.refreshViewportContent()
		return m, nil
	}

	var cmd tea.Cmd
	m.logs.viewport, cmd = m.logs.viewport.Update(msg)
	return m, cmd
}

func (m *AppModel) viewLogs() string {
	panelWidth := m.width - 4
	if panelWidth < 40 {
		panelWidth = 40
	}
	logHeight := m.height - 8
	if logHeight < 10 {
		logHeight = 10
	}

	title := TitleStyle.Render(fmt.Sprintf("Logs: %s", m.logs.instName))

	var followBadge string
	if m.logs.autoScroll {
		followBadge = lipgloss.NewStyle().Foreground(SecondaryColor).Background(BgSurface).Bold(true).Render("[â— FOLLOWING]")
	} else {
		followBadge = lipgloss.NewStyle().Foreground(WarningColor).Background(BgSurface).Bold(true).Render("[â¸ PAUSED]")
	}

	headerElements := []string{title, surfaceGap(2), followBadge}

	if m.logs.isFiltering {
		filterBox := FilterBoxStyle.Render(fmt.Sprintf("  / %s|", m.logs.filterInput))
		headerElements = append(headerElements, filterBox)
	} else if strings.TrimSpace(m.logs.filterInput) != "" {
		vis := m.logs.visibleLines()
		filterInfo := FilterBoxStyle.Render(fmt.Sprintf("  [Filter: \"%s\" (%d/%d)]", m.logs.filterInput, len(vis), len(m.logs.lines)))
		headerElements = append(headerElements, filterInfo)
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top, headerElements...)
	separator := SeparatorStyle.Render(strings.Repeat("â”€", panelWidth-4))

	logContent := LogAreaStyle.
		Width(panelWidth - 4).
		Height(logHeight).
		Render(m.logs.viewport.View())

	var footerContent string
	if m.logs.statusMsg != "" {
		footerContent = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Background(BgSurface).Render(m.logs.statusMsg)
	} else if m.logs.isFiltering {
		footerContent = fmt.Sprintf(" %s %s",
			lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Background(BgSurface).Render("[Enter] Apply"),
			lipgloss.NewStyle().Foreground(FgText).Background(BgSurface).Render("  |  [Esc] Clear filter"),
		)
	} else {
		footerContent = fmt.Sprintf(" %s %s %s %s %s",
			lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Background(BgSurface).Render("[Esc/q] Return"),
			lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Background(BgSurface).Render("  |  [f/Space] Follow"),
			lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Background(BgSurface).Render("  |  [/] Filter"),
			lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Background(BgSurface).Render("  |  [c] Copy"),
			lipgloss.NewStyle().Foreground(FgText).Background(BgSurface).Render("  |  [â†‘/â†“/PgUp/PgDn] Scroll"),
		)
	}

	footer := StatusBarStyle.Width(panelWidth - 2).Render(footerContent)

	return ActivePanelStyle.
		Width(panelWidth).
		Height(m.height - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				header,
				separator,
				logContent,
				footer,
			),
		)
}
