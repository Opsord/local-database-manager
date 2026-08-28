package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestSplitPanelWidthsFitInner(t *testing.T) {
	t.Parallel()

	for _, inner := range []int{60, 80, 98, 118, 140} {
		inner := inner
		t.Run(strconv.Itoa(inner), func(t *testing.T) {
			t.Parallel()
			left, right, gap := splitPanelWidths(inner)
			got := (left + 2) + gap + (right + 2)
			if got != inner {
				t.Fatalf("inner=%d left=%d right=%d gap=%d outer=%d", inner, left, right, gap, got)
			}
		})
	}
}

func TestMainSplitDoesNotWrap(t *testing.T) {
	t.Parallel()

	inner := 118
	left, right, gap := splitPanelWidths(inner)

	leftBox := panelBoxStyle(true).Width(left).Height(8).Render("left")
	rightBox := panelBoxStyle(false).Width(right).Height(8).Render("right")
	gapCell := lipgloss.NewStyle().Background(BgDark).Width(gap).Render(" ")
	split := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gapCell, rightBox)

	if lipgloss.Width(split) != inner {
		t.Fatalf("split width=%d want %d", lipgloss.Width(split), inner)
	}

	first := strings.Split(split, "\n")[0]
	if !strings.Contains(first, "┌") || !strings.Contains(first, "┐") {
		t.Fatalf("top border missing square corners: %q", first)
	}
	if strings.Contains(first, "╭") || strings.Contains(first, "╮") {
		t.Fatalf("top border still uses rounded corners: %q", first)
	}
}

func TestSplitPanelHalfHeight(t *testing.T) {
	t.Parallel()
	top, bottom := splitPanelHalfHeight(20)
	if top != 10 || bottom != 10 {
		t.Fatalf("20 -> top=%d bottom=%d, want 10/10", top, bottom)
	}
	top, bottom = splitPanelHalfHeight(11)
	if top != 5 || bottom != 6 {
		t.Fatalf("11 -> top=%d bottom=%d, want 5/6", top, bottom)
	}
	top, bottom = splitPanelHalfHeight(6)
	if top < 3 || bottom < 3 {
		t.Fatalf("min heights: top=%d bottom=%d", top, bottom)
	}
}

func TestWizardDockedRightColumnMatchesLeftHeight(t *testing.T) {
	t.Parallel()
	inner := 118
	leftW, rightW, _ := splitPanelWidths(inner)
	contentH := 27
	leftBox := panelBoxStyle(true).Width(leftW).Height(contentH).Render("left")

	// Single bordered right panel (not two stacked bordered panels).
	rightInner := panelInnerWidth(rightW)
	top, bottom := splitPanelHalfHeight(contentH - 1)
	detailsBlock := lipgloss.NewStyle().Width(rightInner).Height(top).MaxHeight(top).Render("details")
	wizardBlock := lipgloss.NewStyle().Width(rightInner).Height(bottom).MaxHeight(bottom).Render("wizard")
	innerCol := lipgloss.JoinVertical(lipgloss.Left, detailsBlock, panelSeparator(rightInner), wizardBlock)
	rightCol := ActivePanelStyle.Width(rightW).Height(contentH).Render(innerCol)

	if lipgloss.Height(rightCol) != lipgloss.Height(leftBox) {
		t.Fatalf("right height=%d left height=%d", lipgloss.Height(rightCol), lipgloss.Height(leftBox))
	}
}

func TestPanelInnerWidthMatchesPadding(t *testing.T) {
	t.Parallel()

	contentWidth := 40
	inner := panelInnerWidth(contentWidth)
	line := surfaceLine(inner, "title")
	if lipgloss.Width(line) != inner {
		t.Fatalf("surface line width=%d want %d", lipgloss.Width(line), inner)
	}
}

func TestRenderOverlayPaintsWhitespace(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := &AppModel{width: 40, height: 10}
	out := m.renderOverlay("X")
	if !strings.Contains(out, "48;2;1;22;39") {
		t.Fatalf("overlay whitespace missing BgDark: %q", out)
	}
}

func TestStyleTextInputSetsSurfaceBackground(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	ti := styleTextInput(textinput.New())
	ti.SetValue("abc")
	ti.Focus()
	// Inspect the input's own view (not wrapInputField): the wrapper already
	// emits BgSurface, then inner SGR resets punch holes through it.
	view := ti.View()
	// BgSurface #0b253a → 11;37;58 (Lip Gloss may round to 11;36;58)
	if !strings.Contains(view, "48;2;11;37;58") && !strings.Contains(view, "48;2;11;36;58") {
		t.Fatalf("input view missing BgSurface: %q", view)
	}
}
