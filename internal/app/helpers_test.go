package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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

func TestPanelInnerWidthMatchesPadding(t *testing.T) {
	t.Parallel()

	contentWidth := 40
	inner := panelInnerWidth(contentWidth)
	line := surfaceLine(inner, "title")
	if lipgloss.Width(line) != inner {
		t.Fatalf("surface line width=%d want %d", lipgloss.Width(line), inner)
	}
}
