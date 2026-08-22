package app

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logLineMsg string
type logClosedMsg struct{}

type logsModel struct {
	viewport viewport.Model
	cmd      *exec.Cmd
	lines    []string
	instName string
}

func (m *AppModel) startLogsCmd(inst *core.DatabaseInstance) tea.Cmd {
	m.logs.lines = []string{fmt.Sprintf("Starting logs stream for %s (%s)...", inst.Name, inst.ContainerName)}
	m.logs.instName = inst.Name
	m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
	m.logs.viewport.GotoBottom()

	cmd := m.runner.LogsCommand(inst)
	m.logs.cmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.logs.lines = append(m.logs.lines, fmt.Sprintf("Error opening stdout pipe: %v", err))
		m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
		return nil
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		m.logs.lines = append(m.logs.lines, fmt.Sprintf("Error starting logs command: %v", err))
		m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
		return nil
	}

	return tea.Batch(
		waitForLogLines(stdout),
		waitForLogDone(cmd),
	)
}

func waitForLogLines(r io.Reader) tea.Cmd {
	scanner := bufio.NewScanner(r)
	return func() tea.Msg {
		if scanner.Scan() {
			return logLineMsg(scanner.Text())
		}
		return logClosedMsg{}
	}
}

func waitForLogDone(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		_ = cmd.Wait()
		return logClosedMsg{}
	}
}

func (m *AppModel) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if m.logs.cmd != nil && m.logs.cmd.Process != nil {
				_ = m.logs.cmd.Process.Kill()
			}
			m.mode = ModeMain
			return m, nil
		}

	case logLineMsg:
		m.logs.lines = append(m.logs.lines, string(msg))
		if len(m.logs.lines) > 500 {
			m.logs.lines = m.logs.lines[len(m.logs.lines)-500:]
		}
		m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
		m.logs.viewport.GotoBottom()
		if m.logs.cmd != nil && m.logs.cmd.Stdout != nil {
			return m, waitForLogLines(m.logs.cmd.Stdout.(io.Reader))
		}
		return m, nil

	case logClosedMsg:
		m.logs.lines = append(m.logs.lines, "--- End of logs stream ---")
		m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
		return m, nil
	}

	var cmd tea.Cmd
	m.logs.viewport, cmd = m.logs.viewport.Update(msg)
	return m, cmd
}

func (m *AppModel) viewLogs() string {
	header := TitleStyle.Render(fmt.Sprintf(" Live Logs: %s ", m.logs.instName))
	footer := StatusBarStyle.Width(m.width - 4).Render("Press [Esc] or [q] to return to main menu. Use [↑/↓] or [PgUp/PgDn] to scroll.")

	return ActivePanelStyle.
		Width(m.width - 4).
		Height(m.height - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				header,
				"",
				m.logs.viewport.View(),
				"",
				footer,
			),
		)
}
