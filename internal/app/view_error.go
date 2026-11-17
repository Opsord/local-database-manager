package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type clearErrorModalStatusMsg struct{}

type wrappedErrorLine struct {
	text       string
	hardBreak  bool // true if this line ended with a real '\n' in the source
}

type errorModalModel struct {
	rawText     string
	lines       []wrappedErrorLine
	wrapWidth   int
	cursor      int
	anchor      int // -1 when no keyboard line selection is active
	scrollTop   int
	statusMsg   string
	statusIsErr bool
}

func newErrorModalModel(rawText string, termWidth, termHeight int) errorModalModel {
	m := errorModalModel{
		rawText: rawText,
		anchor:  -1,
	}
	m.rewrap(errorModalInnerWidth(termWidth))
	m.ensureCursorVisible(errorModalBodyHeight(termHeight, len(m.lines)))
	return m
}

func errorModalWidth(termWidth int) int {
	w := termWidth - 8
	if w > 96 {
		w = 96
	}
	if w < 50 {
		w = termWidth - 4
	}
	if w < 30 {
		w = 30
	}
	return w
}

func errorModalInnerWidth(termWidth int) int {
	// ActivePanelStyle has 2 border cells + Padding(0, 1) (2 cells).
	inner := errorModalWidth(termWidth) - 4
	if inner < 20 {
		inner = 20
	}
	return inner
}

func errorModalBodyHeight(termHeight, lineCount int) int {
	maxBody := termHeight - 12
	if maxBody < 4 {
		maxBody = 4
	}
	if lineCount < 4 {
		return 4
	}
	if lineCount < maxBody {
		return lineCount
	}
	return maxBody
}

func (e *errorModalModel) rewrap(innerWidth int) {
	// Reserve 2 cells for line prefix ("> " / "▌ " / "  ").
	textW := innerWidth - 2
	if textW < 16 {
		textW = 16
	}
	if e.wrapWidth == textW && len(e.lines) > 0 {
		return
	}
	e.wrapWidth = textW
	e.lines = wrapErrorLines(e.rawText, textW)
	if len(e.lines) == 0 {
		e.lines = []wrappedErrorLine{{text: "", hardBreak: true}}
	}
	if e.cursor >= len(e.lines) {
		e.cursor = len(e.lines) - 1
	}
	if e.cursor < 0 {
		e.cursor = 0
	}
	if e.anchor >= len(e.lines) {
		e.anchor = len(e.lines) - 1
	}
}

func wrapErrorLines(raw string, maxWidth int) []wrappedErrorLine {
	if maxWidth < 1 {
		maxWidth = 1
	}
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	paragraphs := strings.Split(normalized, "\n")

	var out []wrappedErrorLine
	for _, para := range paragraphs {
		if para == "" {
			out = append(out, wrappedErrorLine{text: "", hardBreak: true})
			continue
		}
		wrapped := wrapParagraph(para, maxWidth)
		for i, seg := range wrapped {
			out = append(out, wrappedErrorLine{
				text:      seg,
				hardBreak: i == len(wrapped)-1,
			})
		}
	}
	return out
}

func wrapParagraph(para string, maxWidth int) []string {
	var lines []string
	rem := para
	for len(rem) > 0 {
		if displayWidth(rem) <= maxWidth {
			lines = append(lines, rem)
			break
		}
		cut := findWrapCut(rem, maxWidth)
		lines = append(lines, strings.TrimRight(rem[:cut], " \t"))
		rem = strings.TrimLeft(rem[cut:], " \t")
		if len(rem) == 0 {
			break
		}
	}
	return lines
}

func findWrapCut(s string, maxWidth int) int {
	width := 0
	lastSpace := -1
	byteIdx := 0
	for byteIdx < len(s) {
		r, size := utf8.DecodeRuneInString(s[byteIdx:])
		rw := displayWidth(string(r))
		if width+rw > maxWidth {
			break
		}
		if r == ' ' || r == '\t' {
			lastSpace = byteIdx
		}
		width += rw
		byteIdx += size
	}
	if byteIdx == 0 {
		_, size := utf8.DecodeRuneInString(s)
		return size
	}
	if lastSpace > 0 {
		return lastSpace
	}
	return byteIdx
}

func (e *errorModalModel) selectionRange() (start, end int, active bool) {
	if e.anchor < 0 || len(e.lines) == 0 {
		return 0, 0, false
	}
	a, b := e.anchor, e.cursor
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if b >= len(e.lines) {
		b = len(e.lines) - 1
	}
	return a, b, true
}

func (e *errorModalModel) selectedText() string {
	start, end, active := e.selectionRange()
	if !active {
		return e.rawText
	}
	if start == 0 && end == len(e.lines)-1 {
		return e.rawText
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		if i > start {
			if e.lines[i-1].hardBreak {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(e.lines[i].text)
	}
	return b.String()
}

func (e *errorModalModel) ensureCursorVisible(bodyH int) {
	if bodyH < 1 {
		bodyH = 1
	}
	if e.cursor < e.scrollTop {
		e.scrollTop = e.cursor
	}
	if e.cursor >= e.scrollTop+bodyH {
		e.scrollTop = e.cursor - bodyH + 1
	}
	maxScroll := len(e.lines) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.scrollTop > maxScroll {
		e.scrollTop = maxScroll
	}
	if e.scrollTop < 0 {
		e.scrollTop = 0
	}
}

func (m *AppModel) openErrorModal() tea.Cmd {
	if m.lastError == "" {
		return nil
	}
	if m.mode != ModeError {
		m.errorReturnMode = m.mode
	}
	m.errorModal = newErrorModalModel(m.lastError, m.width, m.height)
	m.mode = ModeError
	return tea.DisableMouse
}

func (m *AppModel) closeErrorModal() tea.Cmd {
	target := m.errorReturnMode
	if target == ModeError {
		target = ModeMain
	}
	m.mode = target
	return tea.EnableMouseCellMotion
}

func (m *AppModel) copyLastError() tea.Cmd {
	text := m.lastError
	if text == "" && m.statusIsErr && !m.hasActiveConfirm() {
		text = m.statusMsg
	}
	if text == "" {
		return nil
	}
	if err := copyToClipboard(text); err != nil {
		m.setError(fmt.Sprintf("Failed to copy: %v", err))
		return nil
	}
	m.statusMsg = "Error message copied to clipboard!"
	m.statusIsErr = false
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
}

func (m *AppModel) updateErrorModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	e := &m.errorModal
	e.rewrap(errorModalInnerWidth(m.width))
	bodyH := errorModalBodyHeight(m.height, len(e.lines))

	switch msg := msg.(type) {
	case clearErrorModalStatusMsg:
		e.statusMsg = ""
		e.statusIsErr = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if e.anchor >= 0 {
				e.anchor = -1
				return m, nil
			}
			return m, m.closeErrorModal()

		case "q", "v", "!":
			return m, m.closeErrorModal()

		case "ctrl+c":
			// Allow Ctrl+C inside Error modal to copy selection/full error without quitting.
			return m, m.copyErrorModalSelection(true)

		case "up", "k":
			if e.cursor > 0 {
				e.cursor--
				e.ensureCursorVisible(bodyH)
			}
			return m, nil

		case "down", "j":
			if e.cursor < len(e.lines)-1 {
				e.cursor++
				e.ensureCursorVisible(bodyH)
			}
			return m, nil

		case "pgup":
			e.cursor -= bodyH
			if e.cursor < 0 {
				e.cursor = 0
			}
			e.ensureCursorVisible(bodyH)
			return m, nil

		case "pgdown":
			e.cursor += bodyH
			if e.cursor >= len(e.lines) {
				e.cursor = len(e.lines) - 1
			}
			e.ensureCursorVisible(bodyH)
			return m, nil

		case "home", "g":
			e.cursor = 0
			e.ensureCursorVisible(bodyH)
			return m, nil

		case "end", "G":
			e.cursor = len(e.lines) - 1
			e.ensureCursorVisible(bodyH)
			return m, nil

		case " ", "s":
			if e.anchor < 0 {
				e.anchor = e.cursor
			} else {
				e.anchor = -1
			}
			return m, nil

		case "a":
			if len(e.lines) > 0 {
				e.anchor = 0
				e.cursor = len(e.lines) - 1
				e.ensureCursorVisible(bodyH)
			}
			return m, nil

		case "c", "y":
			return m, m.copyErrorModalSelection(true)

		case "enter":
			return m, m.copyErrorModalSelection(false)
		}
	}

	return m, nil
}

func (m *AppModel) copyErrorModalSelection(fullWhenUnmarked bool) tea.Cmd {
	e := &m.errorModal
	var text string
	var label string
	if e.anchor >= 0 {
		text = e.selectedText()
		start, end, _ := e.selectionRange()
		if start == 0 && end == len(e.lines)-1 {
			label = "Copied full error to clipboard!"
		} else {
			label = fmt.Sprintf("Copied %d selected line(s) to clipboard!", end-start+1)
		}
	} else if fullWhenUnmarked || len(e.lines) <= 1 {
		text = e.rawText
		label = "Copied full error to clipboard!"
	} else {
		text = e.lines[e.cursor].text
		label = "Copied current line to clipboard!"
	}

	if err := copyToClipboard(text); err != nil {
		e.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
		e.statusIsErr = true
	} else {
		e.statusMsg = label
		e.statusIsErr = false
	}
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearErrorModalStatusMsg{} })
}

func (m *AppModel) viewErrorModal() string {
	e := &m.errorModal
	modalWidth := errorModalWidth(m.width)
	innerW := errorModalInnerWidth(m.width)
	e.rewrap(innerW)
	bodyH := errorModalBodyHeight(m.height, len(e.lines))
	e.ensureCursorVisible(bodyH)

	titleLeft := TitleStyle.Render("Error Details")
	mouseBadge := lipgloss.NewStyle().
		Foreground(TealColor).
		Background(SelectedBg).
		Padding(0, 1).
		Render("Mouse selection active")
	titleGapW := innerW - lipgloss.Width(titleLeft) - lipgloss.Width(mouseBadge)
	if titleGapW < 1 {
		titleGapW = 1
	}
	headerRow := surfaceLine(innerW, lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleLeft,
		surfaceGap(titleGapW),
		mouseBadge,
	))

	subHint := surfaceLine(innerW, MutedStyle.Render("Drag mouse to select any text, or use [↑/↓] + [Space] to select lines."))
	sep := panelSeparator(innerW)

	selStart, selEnd, selActive := e.selectionRange()
	endIdx := e.scrollTop + bodyH
	if endIdx > len(e.lines) {
		endIdx = len(e.lines)
	}

	var rows []string
	for i := e.scrollTop; i < endIdx; i++ {
		inSel := selActive && i >= selStart && i <= selEnd
		isCursor := i == e.cursor

		bg := BgSurface
		fg := ErrorColor
		prefix := "  "
		prefixFg := MutedColor
		bold := false

		if inSel {
			bg = SelectedBg
			fg = FgText
			prefix = "▌ "
			prefixFg = PrimaryColor
			bold = true
		} else if isCursor {
			prefix = "> "
			prefixFg = PrimaryColor
			bold = true
		}

		prefixStyled := lipgloss.NewStyle().Foreground(prefixFg).Background(bg).Bold(true).Render(prefix)
		lineStyled := lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(bold).Render(e.lines[i].text)
		rowContent := lipgloss.JoinHorizontal(lipgloss.Top, prefixStyled, lineStyled)
		rows = append(rows, lipgloss.NewStyle().
			Width(innerW).
			MaxWidth(innerW).
			Background(bg).
			Render(rowContent))
	}
	for len(rows) < bodyH {
		rows = append(rows, surfaceBlankLine(innerW))
	}

	body := lipgloss.NewStyle().
		Width(innerW).
		Height(bodyH).
		MaxHeight(bodyH).
		Background(BgSurface).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	var footerLine string
	if e.statusMsg != "" {
		stColor := SecondaryColor
		if e.statusIsErr {
			stColor = ErrorColor
		}
		footerLine = surfaceLine(innerW, lipgloss.NewStyle().Foreground(stColor).Bold(true).Background(BgSurface).Render(e.statusMsg))
	} else {
		shortcut := func(key, desc string) string {
			return lipgloss.JoinHorizontal(lipgloss.Top, KeyStyle.Render(key), surfaceGap(1), KeyDescStyle.Render(desc))
		}
		entries := []string{
			shortcut("[↑/↓]", "Line"),
			shortcut("[Space]", "Select"),
			shortcut("[a]", "All"),
			shortcut("[c]", "Copy"),
			shortcut("[Enter]", "Copy Line"),
			shortcut("[Esc]", "Close"),
		}
		if selActive {
			entries[1] = shortcut("[Space]", "Clear Sel")
			entries[4] = shortcut("[Enter]", "Copy Sel")
		}
		bar := formatShortcutBar(innerW, entries)
		if len(e.lines) > bodyH {
			pos := fmt.Sprintf(" %d-%d/%d", e.scrollTop+1, endIdx, len(e.lines))
			if lipgloss.Width(bar)+len(pos)+1 <= innerW {
				gap := innerW - lipgloss.Width(bar) - len(pos)
				bar = lipgloss.JoinHorizontal(lipgloss.Top, bar, surfaceGap(gap), MutedStyle.Render(pos))
			}
		}
		footerLine = lipgloss.NewStyle().Width(innerW).Background(BgSurface).Render(bar)
	}

	return ActivePanelStyle.
		Width(modalWidth).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			headerRow,
			subHint,
			sep,
			body,
			sep,
			footerLine,
		))
}
