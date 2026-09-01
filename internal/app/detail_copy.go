package app

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type copyHit struct {
	X, Y, W, H int
	Text       string
}

type valueToken struct {
	Start int
	End   int
	Text  string
}

func isTokenRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '_', '.', '/', ':', '@', '%', '+', '-':
		return true
	}
	return false
}

func tokenizeValue(s string) []valueToken {
	var out []valueToken
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isTokenRune(r) {
			i += size
			continue
		}
		start := i
		i += size
		for i < len(s) {
			r2, size2 := utf8.DecodeRuneInString(s[i:])
			if !isTokenRune(r2) {
				break
			}
			i += size2
		}
		out = append(out, valueToken{Start: start, End: i, Text: s[start:i]})
	}
	return out
}

func displayWidth(s string) int { return lipgloss.Width(s) }

func appendValueTokenHits(dst []copyHit, originX, originY int, plainValue string) []copyHit {
	for _, tok := range tokenizeValue(plainValue) {
		dst = append(dst, copyHit{
			X:    originX + displayWidth(plainValue[:tok.Start]),
			Y:    originY,
			W:    displayWidth(tok.Text),
			H:    1,
			Text: tok.Text,
		})
	}
	return dst
}

const labelColumnWidth = 14 + 1

func valueOriginX(fieldOriginX int) int {
	return fieldOriginX + labelColumnWidth
}

func hitTest(hits []copyHit, x, y int) (string, bool) {
	for _, h := range hits {
		if y >= h.Y && y < h.Y+h.H && x >= h.X && x < h.X+h.W {
			return h.Text, true
		}
	}
	return "", false
}

type clickTracker struct {
	x, y  int
	at    time.Time
	armed bool
}

const doubleClickWindow = 500 * time.Millisecond

func (t *clickTracker) register(x, y int, now time.Time) bool {
	if t.armed && now.Sub(t.at) <= doubleClickWindow &&
		absInt(x-t.x) <= 1 && absInt(y-t.y) <= 1 {
		t.armed = false
		return true
	}
	t.x, t.y, t.at, t.armed = x, y, now, true
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func buildDetailHits(inst *core.DatabaseInstance, panelWidth, rightInner, originX, originY, maxYExclusive int) []copyHit {
	fields := plainDetailFields(inst, panelWidth)
	codeBoxWidth := rightInner - 16
	if codeBoxWidth < 20 {
		codeBoxWidth = 20
	}

	var hits []copyHit
	rowY := originY

	if panelWidth < 70 {
		for _, f := range fields {
			hits = appendValueTokenHits(hits, valueOriginX(originX), rowY, f.Value)
			rowY++
		}
	} else {
		colGap := 3
		availW := panelWidth - 4
		col1W := (availW - colGap) / 2
		for i := 0; i < len(fields); i += 2 {
			hits = appendValueTokenHits(hits, valueOriginX(originX), rowY, fields[i].Value)
			if i+1 < len(fields) {
				col2X := originX + col1W + colGap
				hits = appendValueTokenHits(hits, valueOriginX(col2X), rowY, fields[i+1].Value)
			}
			rowY++
		}
	}

	rowY++ // blank row before URI/CLI
	hits = appendValueTokenHits(hits, valueOriginX(originX), rowY, truncateMiddle(inst.ConnectionURI(), codeBoxWidth))
	rowY++
	hits = appendValueTokenHits(hits, valueOriginX(originX), rowY, truncateMiddle(inst.CLICommand(), codeBoxWidth))

	var filtered []copyHit
	for _, h := range hits {
		if h.Y >= originY && h.Y < maxYExclusive {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func (m *AppModel) detailsContentOrigin() (originX, originY, maxYExclusive int) {
	inner := screenInnerWidth(m.width)
	leftWidth, _, gapW := splitPanelWidths(inner)
	contentHeight := mainContentHeight(m.height)

	// wrapScreen Padding(0,1) + left outer + gap + right border + right padding
	originX = 1 + leftWidth + 2 + gapW + 1 + 1

	// header row + panel top border + title row
	originY = 1 + 1 + 1

	var detailsRegionHeight int
	if m.mode == ModeWizard || m.mode == ModeActionMenu {
		detailsRegionHeight, _ = splitPanelHalfHeight(contentHeight - 1)
	} else {
		detailsRegionHeight = contentHeight
	}
	panelInnerY := originY - 1
	maxYExclusive = panelInnerY + detailsRegionHeight
	return originX, originY, maxYExclusive
}

func (m *AppModel) refreshDetailHits() {
	inst := m.selectedInstance()
	if inst == nil {
		m.detailHits = nil
		return
	}
	inner := screenInnerWidth(m.width)
	_, rightWidth, _ := splitPanelWidths(inner)
	rightInner := panelInnerWidth(rightWidth)
	ox, oy, maxY := m.detailsContentOrigin()
	m.detailHits = buildDetailHits(inst, rightWidth, rightInner, ox, oy, maxY)
}

func (m *AppModel) handleDetailsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	return m.handleDetailsMouseAt(msg, time.Now())
}

func (m *AppModel) handleDetailsMouseAt(msg tea.MouseMsg, now time.Time) (tea.Model, tea.Cmd, bool) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil, false
	}
	m.refreshDetailHits()
	if !m.detailClick.register(msg.X, msg.Y, now) {
		return m, nil, false
	}
	text, ok := hitTest(m.detailHits, msg.X, msg.Y)
	if !ok {
		return m, nil, true
	}
	if err := core.CopyToClipboard(text); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
		m.statusIsErr = true
	} else {
		m.statusMsg = fmt.Sprintf("Copied: %s", text)
		m.statusIsErr = false
	}
	return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }), true
}
