package app

import (
	"fmt"
	"strings"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) wrapScreen(content string) string {
	return lipgloss.NewStyle().
		Width(m.width).
		Background(BgDark).
		Padding(0, 1).
		Render(content)
}

func (m *AppModel) renderOverlay(modal string) string {
	return lipgloss.Place(
		m.width-2,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceBackground(BgDark),
	)
}

func truncateMiddle(s string, max int) string {
	if max <= 3 || len(s) <= max {
		return s
	}
	keep := max - 3
	left := keep / 2
	right := keep - left
	return s[:left] + "..." + s[len(s)-right:]
}

func truncateEnd(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func statusIcon(status core.ContainerStatus) string {
	switch status {
	case core.StatusReady:
		return RunningStyle.Render("●")
	case core.StatusStarting:
		return StartingStyle.Render("◐")
	case core.StatusUnknown:
		return UnknownStyle.Render("?")
	default:
		return StoppedStyle.Render("○")
	}
}

func statusLabel(status core.ContainerStatus) string {
	switch status {
	case core.StatusReady:
		return RunningStyle.Render("● RUNNING")
	case core.StatusStarting:
		return StartingStyle.Render("◐ STARTING")
	case core.StatusUnknown:
		return UnknownStyle.Render("? UNKNOWN")
	default:
		return StoppedStyle.Render("○ STOPPED")
	}
}

func engineBadge(name string, health core.EngineHealth) string {
	switch health {
	case core.EngineOnline:
		return BadgeOnlineStyle.Render(fmt.Sprintf(" %s ONLINE ", name))
	case core.EngineNotInstalled:
		return BadgeUnknownStyle.Render(fmt.Sprintf(" %s N/A ", name))
	default:
		return BadgeOfflineStyle.Render(fmt.Sprintf(" %s OFFLINE ", name))
	}
}

func styleTextInput(ti textinput.Model) textinput.Model {
	surface := lipgloss.NewStyle().Foreground(FgText).Background(BgSurface)
	ti.Prompt = ""
	ti.PromptStyle = lipgloss.NewStyle().Foreground(AccentColor).Background(BgSurface)
	ti.TextStyle = surface
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(MutedColor).Background(BgSurface)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(PrimaryColor).Background(BgSurface)
	ti.Cursor.TextStyle = surface
	return ti
}

func wrapInputField(view string) string {
	return InputBoxStyle.Render(view)
}

func surfaceGap(n int) string {
	if n < 1 {
		n = 1
	}
	return lipgloss.NewStyle().Background(BgSurface).Render(strings.Repeat(" ", n))
}

func screenInnerWidth(termWidth int) int {
	w := termWidth - 2 // wrapScreen horizontal padding
	if w < 40 {
		return 40
	}
	return w
}

func splitPanelWidths(innerWidth int) (left, right, gap int) {
	gap = 1
	avail := innerWidth - gap - 4 // two 1-cell borders per panel
	if avail < 16 {
		avail = 16
	}
	left = avail / 3
	if left < 34 {
		left = 34
	}
	right = avail - left
	if right < 8 {
		right = 8
		left = avail - right
	}
	return left, right, gap
}

func panelBoxStyle(active bool) lipgloss.Style {
	if active {
		return ActivePanelStyle
	}
	return PanelStyle
}

func panelInnerWidth(contentWidth int) int {
	w := contentWidth - 2 // horizontal padding only; borders sit outside Width()
	if w < 8 {
		return 8
	}
	return w
}

func surfaceLine(width int, content string) string {
	return lipgloss.NewStyle().
		Width(width).
		Background(BgSurface).
		MaxHeight(1).
		Render(content)
}

func panelTitle(text string, width int) string {
	return TitleStyle.Width(width).Render(text)
}

func panelSeparator(width int) string {
	if width < 1 {
		width = 1
	}
	return SeparatorStyle.Width(width).Render(strings.Repeat("─", width))
}

func surfaceBlankLine(width int) string {
	return surfaceLine(width, " ")
}

func detailField(label, value string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render(label), surfaceGap(1), value)
}

type plainDetailField struct {
	Label, Value string
}

func statusPlainText(status core.ContainerStatus) string {
	switch status {
	case core.StatusReady:
		return "RUNNING"
	case core.StatusStarting:
		return "STARTING"
	case core.StatusUnknown:
		return "UNKNOWN"
	default:
		return "STOPPED"
	}
}

// statusDisplayPlain matches statusLabel layout (icon + space + word) without ANSI,
// so double-click hit boxes align with the visible status token.
func statusDisplayPlain(status core.ContainerStatus) string {
	return statusIconPlain(status) + " " + statusPlainText(status)
}

func plainDetailFields(inst *core.DatabaseInstance, panelWidth int) []plainDetailField {
	engineDesc := fmt.Sprintf("%s (%s)", strings.ToUpper(inst.EngineType), strings.ToUpper(inst.Runtime))
	memFormatted := fmt.Sprintf("%s (Limit: %s)", inst.MemoryUsage, inst.MemoryLimit)

	fields := []plainDetailField{
		{"Engine:", engineDesc},
	}
	if inst.EngineType == "postgres" {
		fields = append(fields, plainDetailField{"Version:", inst.Version})
	}
	fields = append(fields, []plainDetailField{
		{"Container:", truncateEnd(inst.ContainerName, panelWidth-20)},
		{"Status:", statusDisplayPlain(inst.Status)},
		{"Memory:", memFormatted},
		{"Database:", inst.Database},
		{"Port:", fmt.Sprintf("%d", inst.Port)},
		{"User:", inst.User},
		{"Schema:", inst.Schema},
		{"Volume:", truncateEnd(inst.Volume, panelWidth-20)},
		{"Project:", truncateEnd(inst.ProjectName, panelWidth-20)},
	}...)
	return fields
}

func styledDetailValue(label string, plainValue string, inst *core.DatabaseInstance) string {
	switch label {
	case "Engine:", "Port:":
		return ValueHighlightStyle.Render(plainValue)
	case "Status:":
		return statusLabel(inst.Status)
	default:
		return ValueStyle.Render(plainValue)
	}
}

func renderDetailRows(inst *core.DatabaseInstance, panelWidth int) []string {
	plain := plainDetailFields(inst, panelWidth)
	fields := make([]struct {
		label string
		value string
	}, len(plain))
	for i, f := range plain {
		fields[i].label = f.Label
		fields[i].value = styledDetailValue(f.Label, f.Value, inst)
	}

	if panelWidth < 70 {
		rows := make([]string, 0, len(fields))
		for _, f := range fields {
			rows = append(rows, detailField(f.label, f.value))
		}
		return rows
	}

	colGap := 3
	availW := panelWidth - 4
	col1W := (availW - colGap) / 2
	col2W := availW - col1W - colGap
	col1Style := lipgloss.NewStyle().Width(col1W).Background(BgSurface)
	col2Style := lipgloss.NewStyle().Width(col2W).Background(BgSurface)

	var rows []string
	for i := 0; i < len(fields); i += 2 {
		left := col1Style.Render(detailField(fields[i].label, fields[i].value))
		if i+1 < len(fields) {
			right := col2Style.Render(detailField(fields[i+1].label, fields[i+1].value))
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, surfaceGap(colGap), right))
		} else {
			rows = append(rows, left)
		}
	}
	return rows
}

func joinWithSurfaceGaps(parts []string, gapWidth int) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out = lipgloss.JoinHorizontal(lipgloss.Top, out, surfaceGap(gapWidth), parts[i])
	}
	return out
}

func formatShortcutBar(width int, shortcuts []string) string {
	line := joinWithSurfaceGaps(shortcuts, 2)
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}
	mid := (len(shortcuts) + 1) / 2
	row1 := joinWithSurfaceGaps(shortcuts[:mid], 2)
	row2 := joinWithSurfaceGaps(shortcuts[mid:], 2)
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}

func statusIconPlain(status core.ContainerStatus) string {
	switch status {
	case core.StatusReady:
		return "●"
	case core.StatusStarting:
		return "◐"
	case core.StatusUnknown:
		return "?"
	default:
		return "○"
	}
}

func listLineStyle(status core.ContainerStatus, selected bool) lipgloss.Style {
	if selected {
		return SelectedItemStyle
	}
	switch status {
	case core.StatusReady:
		return RunningStyle
	case core.StatusStarting:
		return StartingStyle
	case core.StatusUnknown:
		return UnknownStyle
	default:
		return StoppedStyle
	}
}

func renderListLine(status core.ContainerStatus, runtimeTag, engineLabel, name string, width int, selected bool) string {
	plain := fmt.Sprintf("%s  %-7s %-9s %s", statusIconPlain(status), runtimeTag, engineLabel, name)
	prefix := "  "
	if selected {
		prefix = "> "
	}
	return listLineStyle(status, selected).Width(width).Render(prefix + plain)
}

func mainContentHeight(termHeight int) int {
	// header 1 + panel borders 2 + footer 2
	const reserved = 5
	h := termHeight - reserved
	if h < 6 {
		return 6
	}
	return h
}

func splitPanelHalfHeight(contentHeight int) (top, bottom int) {
	top = contentHeight / 2
	bottom = contentHeight - top
	if top < 3 {
		top = 3
	}
	if bottom < 3 {
		bottom = 3
	}
	if top+bottom > contentHeight {
		top = contentHeight - bottom
		if top < 3 {
			top = 3
			bottom = contentHeight - top
		}
	}
	return top, bottom
}
