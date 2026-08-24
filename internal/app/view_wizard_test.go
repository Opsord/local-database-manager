package app

import (
	"strings"
	"testing"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func cellsWithoutBG(view string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		inEsc := false
		var esc strings.Builder
		hasBG := false
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == 0x1b {
				inEsc = true
				esc.Reset()
				continue
			}
			if inEsc {
				esc.WriteByte(c)
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					inEsc = false
					seq := esc.String()
					if strings.Contains(seq, "48;2;") || strings.Contains(seq, "48;5;") {
						hasBG = true
					}
					if seq == "[0m" || seq == "[m" || seq == "[;m" {
						hasBG = false
					}
				}
				continue
			}
			if !hasBG {
				n++
			}
		}
	}
	return n
}

func reviewModel(width, height int) *AppModel {
	m := NewApp("/tmp")
	m.width = width
	m.height = height
	m.mode = ModeWizard
	m.instances = []*core.DatabaseInstance{{Name: "demo", Port: 5432}}
	m.wizard = newWizardModel(m.projectRoot, m.instancesDir, m.instances)
	m.wizard.step = StepReview
	m.wizard.inputs[0].SetValue("shop")
	m.wizard.inputs[1].SetValue("pg-this-is-a-very-long-container-name-that-will-overflow")
	m.wizard.inputs[2].SetValue("5433")
	m.wizard.inputs[3].SetValue("shop_db")
	m.wizard.inputs[4].SetValue("pgdata_shop")
	m.wizard.inputs[5].SetValue("s3cretPass")
	m.wizard.inputs[6].SetValue("512M")
	return m
}

func TestWizardReviewFitsTerminal(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, sz := range []struct{ w, h int }{{120, 32}, {80, 24}} {
		m := reviewModel(sz.w, sz.h)
		view := m.View()
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > sz.w {
				t.Fatalf("%dx%d line %d width=%d: %q", sz.w, sz.h, i, got, stripANSI(line))
			}
		}
		plain := stripANSI(view)
		if strings.Contains(plain, "\noverflow") || strings.Contains(plain, "│ overflow") {
			t.Fatalf("long container name wrapped out of its row:\n%s", plain)
		}
	}
}

func TestWizardReviewUsesDisplayLabels(t *testing.T) {
	t.Parallel()
	plain := stripANSI(reviewModel(120, 32).View())
	if strings.Contains(plain, "postgres") || strings.Contains(plain, "docker") {
		t.Fatalf("review still shows raw ids:\n%s", plain)
	}
	if !strings.Contains(plain, "Postgres") || !strings.Contains(plain, "Docker") {
		t.Fatalf("review missing display labels:\n%s", plain)
	}
}

func TestWizardReviewShowsPasswordAndHidesIdlePrompts(t *testing.T) {
	t.Parallel()
	plain := stripANSI(reviewModel(120, 32).View())
	if !strings.Contains(plain, "s3cretPass") {
		t.Fatalf("password missing from review:\n%s", plain)
	}
	if strings.Contains(plain, "•") {
		t.Fatalf("password still masked:\n%s", plain)
	}
	if strings.Count(plain, ">") > 0 {
		t.Fatalf("completed fields still show input prompt:\n%s", plain)
	}
}

func TestWizardReviewFillsSurface(t *testing.T) {
	// SetColorProfile is process-global; do not run this in parallel.
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := reviewModel(120, 32)
	got := cellsWithoutBG(m.View())
	const maxUnpainted = 80
	if got > maxUnpainted {
		t.Fatalf("wizard review unpainted cells=%d want <= %d", got, maxUnpainted)
	}
}
