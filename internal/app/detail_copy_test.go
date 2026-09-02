package app

import (
	"strings"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTokenizeValue(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"postgres", []string{"postgres"}},
		{"POSTGRES (DOCKER)", []string{"POSTGRES", "DOCKER"}},
		{"124.5MiB (Limit: 512MiB)", []string{"124.5MiB", "Limit:", "512MiB"}},
		{"postgresql://u:p@localhost:5432/db", []string{"postgresql://u:p@localhost:5432/db"}},
		{"", nil},
	}
	for _, tt := range tests {
		toks := tokenizeValue(tt.in)
		var got []string
		for _, tok := range toks {
			got = append(got, tok.Text)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("%q[%d]: got %q want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestHitTest(t *testing.T) {
	hits := []copyHit{
		{X: 10, Y: 5, W: 8, H: 1, Text: "postgres"},
		{X: 20, Y: 5, W: 4, H: 1, Text: "5432"},
	}
	if text, ok := hitTest(hits, 12, 5); !ok || text != "postgres" {
		t.Fatalf("got %q %v", text, ok)
	}
	if text, ok := hitTest(hits, 21, 5); !ok || text != "5432" {
		t.Fatalf("got %q %v", text, ok)
	}
	if _, ok := hitTest(hits, 0, 0); ok {
		t.Fatal("expected miss")
	}
}

func TestClickTrackerDoubleClick(t *testing.T) {
	var tr clickTracker
	t0 := time.Unix(0, 0)
	if tr.register(10, 5, t0) {
		t.Fatal("first click must not be double")
	}
	if !tr.register(10, 5, t0.Add(300*time.Millisecond)) {
		t.Fatal("expected double within window same cell")
	}
	if tr.register(10, 5, t0.Add(400*time.Millisecond)) {
		t.Fatal("after double, next click is a new first click")
	}
	if tr.register(40, 5, t0.Add(500*time.Millisecond)) {
		t.Fatal("far cell must reset, not double")
	}
	if tr.register(10, 5, t0.Add(2*time.Second)) {
		t.Fatal("outside 500ms must not double")
	}
}

func TestAppendValueTokenHits(t *testing.T) {
	hits := appendValueTokenHits(nil, 100, 7, "POSTGRES (DOCKER)")
	if len(hits) != 2 {
		t.Fatalf("got %d hits: %+v", len(hits), hits)
	}
	if hits[0].Text != "POSTGRES" || hits[0].X != 100 || hits[0].Y != 7 || hits[0].W != 8 {
		t.Fatalf("first hit %+v", hits[0])
	}
	// "POSTGRES"(8) + " "(1) + "("(1) => DOCKER at 110
	if hits[1].Text != "DOCKER" || hits[1].X != 110 || hits[1].W != 6 {
		t.Fatalf("second hit %+v", hits[1])
	}
}

func TestBuildDetailHitsIncludesUserToken(t *testing.T) {
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		Version: "16", ContainerName: "pg_shop", Status: core.StatusReady,
		MemoryUsage: "100MiB", MemoryLimit: "512MiB", Database: "shop",
		Port: 5432, User: "postgres", Schema: "public",
		Volume: "pgdata_shop_16", ProjectName: "pg_shop",
	}
	hits := buildDetailHits(inst, 120, 56, 0, 0, 100)
	found := false
	for _, h := range hits {
		if h.Text == "postgres" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected postgres token in hits: %+v", hits)
	}
	for _, h := range hits {
		switch h.Text {
		case "User:", "[c]", "copy":
			t.Fatalf("label/hint must not be a hit: %+v", h)
		}
	}
}

func TestBuildDetailHitsStatusTokenAlignedWithIcon(t *testing.T) {
	inst := &core.DatabaseInstance{
		EngineType: "postgres", Runtime: "docker", Status: core.StatusReady,
		MemoryUsage: "-", MemoryLimit: "-", User: "u", Port: 1,
	}
	originX := 50
	hits := buildDetailHits(inst, 40, 36, originX, 10, 100)
	var statusHit *copyHit
	for i := range hits {
		if hits[i].Text == "RUNNING" {
			statusHit = &hits[i]
			break
		}
	}
	if statusHit == nil {
		t.Fatalf("expected RUNNING token hit, got %+v", hits)
	}
	prefix := statusIconPlain(core.StatusReady) + " "
	wantX := valueOriginX(originX) + displayWidth(prefix)
	if statusHit.X != wantX {
		t.Fatalf("RUNNING hit X=%d want %d (icon prefix width %d)", statusHit.X, wantX, displayWidth(prefix))
	}
}

func TestBuildDetailHitsRespectsMaxY(t *testing.T) {
	inst := &core.DatabaseInstance{
		EngineType: "postgres", Runtime: "docker", User: "postgres", Port: 1,
		Status: core.StatusStopped, MemoryUsage: "-", MemoryLimit: "-",
	}
	hits := buildDetailHits(inst, 40, 36, 0, 0, 1)
	for _, h := range hits {
		if h.Y >= 1 {
			t.Fatalf("hit beyond maxY: %+v", h)
		}
	}
}

func TestDetailsContentOriginWizardHasSmallerMaxY(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	m.mode = ModeMain
	_, _, maxYMain := m.detailsContentOrigin()
	m.mode = ModeWizard
	_, _, maxYWizard := m.detailsContentOrigin()
	if maxYWizard >= maxYMain {
		t.Fatalf("wizard maxY=%d should be < main maxY=%d", maxYWizard, maxYMain)
	}
}

func TestDetailsContentOriginInRightPanel(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	ox, oy, maxY := m.detailsContentOrigin()
	inner := screenInnerWidth(m.width)
	left, _, gap := splitPanelWidths(inner)
	minX := 1 + left + 2 + gap // wrap + left outer + gap
	if ox < minX {
		t.Fatalf("originX=%d want >= %d", ox, minX)
	}
	if oy < 2 {
		t.Fatalf("originY=%d", oy)
	}
	if maxY <= oy {
		t.Fatalf("maxY=%d oy=%d", maxY, oy)
	}
}

func TestSuccessfulCopySetsCopiedHit(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	m.instances = []*core.DatabaseInstance{{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		User: "postgres", Port: 5432, Status: core.StatusReady,
		MemoryUsage: "-", MemoryLimit: "-", Database: "shop", Schema: "public",
	}}
	m.selectedIndex = 0
	m.refreshDetailHits()
	if len(m.detailHits) == 0 {
		t.Fatal("expected hits")
	}
	h := m.detailHits[0]
	t0 := time.Now()
	m.detailClick = clickTracker{x: h.X, y: h.Y, at: t0, armed: true}
	msg := tea.MouseMsg{X: h.X, Y: h.Y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	_, _, handled := m.handleDetailsMouseAt(msg, t0.Add(100*time.Millisecond))
	if !handled {
		t.Fatal("expected handled")
	}
	if m.copiedHit == nil || m.copiedHit.Text == "" {
		t.Fatalf("expected copiedHit set, got %#v", m.copiedHit)
	}
	if m.copiedHit.Text != h.Text {
		t.Fatalf("copiedHit.Text=%q want %q", m.copiedHit.Text, h.Text)
	}
}

func TestClearStatusMsgClearsCopiedHit(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	hit := copyHit{X: 1, Y: 2, W: 3, H: 1, Text: "postgres"}
	m.copiedHit = &hit
	m.statusMsg = "Copied: postgres"
	updated, _ := m.Update(clearStatusMsg{})
	am := updated.(*AppModel)
	if am.copiedHit != nil {
		t.Fatalf("expected copiedHit cleared, got %#v", am.copiedHit)
	}
	if am.statusMsg != "" {
		t.Fatalf("status=%q", am.statusMsg)
	}
}

func TestSelectionChangeClearsCopiedHit(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	m.instances = []*core.DatabaseInstance{
		{Name: "a", EngineType: "postgres", Runtime: "docker", User: "u1", Port: 1, Status: core.StatusReady, MemoryUsage: "-", MemoryLimit: "-", Database: "d", Schema: "public"},
		{Name: "b", EngineType: "postgres", Runtime: "docker", User: "u2", Port: 2, Status: core.StatusReady, MemoryUsage: "-", MemoryLimit: "-", Database: "d", Schema: "public"},
	}
	m.selectedIndex = 0
	hit := copyHit{Text: "u1"}
	m.copiedHit = &hit
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	am := updated.(*AppModel)
	if am.copiedHit != nil {
		t.Fatalf("expected clear on selection change, got %#v", am.copiedHit)
	}
}

func TestHandleDetailsMouseDoubleClickSetsStatus(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	m.instances = []*core.DatabaseInstance{{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		User: "postgres", Port: 5432, Status: core.StatusReady,
		MemoryUsage: "-", MemoryLimit: "-", Database: "shop", Schema: "public",
	}}
	m.selectedIndex = 0
	m.refreshDetailHits()
	if len(m.detailHits) == 0 {
		t.Fatal("expected hits")
	}
	h := m.detailHits[0]
	t0 := time.Now()
	m.detailClick = clickTracker{x: h.X, y: h.Y, at: t0, armed: true}
	msg := tea.MouseMsg{
		X: h.X, Y: h.Y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	_, cmd, handled := m.handleDetailsMouseAt(msg, t0.Add(100*time.Millisecond))
	if !handled {
		t.Fatal("expected handled double-click")
	}
	if !strings.HasPrefix(m.statusMsg, "Copied: ") && !strings.HasPrefix(m.statusMsg, "Failed to copy:") {
		t.Fatalf("status=%q", m.statusMsg)
	}
	_ = cmd
}
