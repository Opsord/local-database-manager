package app

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = width
	m.height = height
	m.mode = ModeWizard
	m.instances = []*core.DatabaseInstance{{Name: "demo", Port: 5432}}
	m.wizard = newWizardModel(m.projectRoot, m.instancesDir, m.instances)
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview
	m.wizard.inputs[0].SetValue("shop")
	m.wizard.inputs[1].SetValue("pg-this-is-a-very-long-container-name-that-will-overflow")
	m.wizard.inputs[2].SetValue("5433")
	m.wizard.inputs[3].SetValue("shop_db")
	m.wizard.inputs[4].SetValue("pgdata_shop")
	m.wizard.inputs[5].SetValue("s3cretPass")
	m.wizard.inputs[6].SetValue("512M")
	return m
}

func TestWizardNewModelStartsAtEngine(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if w.step != StepEngine {
		t.Fatalf("step = %v, want StepEngine", w.step)
	}
	if w.maxReached != StepEngine {
		t.Fatalf("maxReached = %v, want StepEngine", w.maxReached)
	}
}

func TestWizardMoveFocusRespectsUnlock(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.maxReached = StepRuntime
	w.step = StepEngine

	w.moveFocus(1)
	if w.step != StepRuntime {
		t.Fatalf("after down: step = %v, want StepRuntime", w.step)
	}
	w.moveFocus(1) // locked Name
	if w.step != StepRuntime {
		t.Fatalf("should not enter locked Name, got %v", w.step)
	}
	w.moveFocus(-1)
	if w.step != StepEngine {
		t.Fatalf("after up: step = %v, want StepEngine", w.step)
	}
}

func TestWizardCycleOptionIsLateralOnly(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if w.selectedEngineIdx != 0 {
		t.Fatalf("engine idx = %d", w.selectedEngineIdx)
	}
	w.cycleOption(1)
	if w.selectedEngineIdx != 1 {
		t.Fatalf("after right: engine idx = %d, want 1", w.selectedEngineIdx)
	}
	w.cycleOption(1)
	if w.selectedEngineIdx != 1 {
		t.Fatalf("should clamp at last engine, got %d", w.selectedEngineIdx)
	}
	w.cycleOption(-1)
	if w.selectedEngineIdx != 0 {
		t.Fatalf("after left: engine idx = %d, want 0", w.selectedEngineIdx)
	}

	w.step = StepRuntime
	w.maxReached = StepRuntime
	w.cycleOption(1)
	if w.selectedRuntimeIdx != 1 {
		t.Fatalf("runtime idx = %d, want 1", w.selectedRuntimeIdx)
	}

	w.step = StepName
	w.maxReached = StepName
	before := w.selectedEngineIdx
	w.cycleOption(1)
	if w.selectedEngineIdx != before {
		t.Fatal("cycleOption on text row must not change engine")
	}
}

func TestWizardApplyEngineDefaultsOnlyWhenStillDefault(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[3].SetValue("shop_db")
	w.inputs[4].SetValue("pgdata_shop")
	w.inputs[5].SetValue("postgres")
	w.inputs[6].SetValue("512M")
	w.selectedEngineIdx = 0

	w.applyEngineDefaults("postgres", "sqlserver")
	w.selectedEngineIdx = 1

	if w.inputs[1].Value() != "sql-shop" {
		t.Fatalf("container = %q, want sql-shop", w.inputs[1].Value())
	}
	wantPort := strconv.Itoa(core.FindNextFreePort(1433, nil))
	if w.inputs[2].Value() != wantPort {
		t.Fatalf("port = %q, want %s", w.inputs[2].Value(), wantPort)
	}
	if w.inputs[4].Value() != "sqlserver_shop" {
		t.Fatalf("volume = %q, want sqlserver_shop", w.inputs[4].Value())
	}
	if w.inputs[5].Value() != "SuperPassword123!" {
		t.Fatalf("password = %q", w.inputs[5].Value())
	}
	if w.inputs[6].Value() != "2G" {
		t.Fatalf("memory = %q, want 2G", w.inputs[6].Value())
	}
	if w.inputs[3].Value() != "shop_db" {
		t.Fatalf("database should stay %q", w.inputs[3].Value())
	}
}

func TestWizardApplyEngineDefaultsSkipsEditedFields(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("custom-box")
	w.inputs[2].SetValue("5555")
	w.inputs[4].SetValue("custom_vol")
	w.inputs[5].SetValue("my-secret")
	w.inputs[6].SetValue("1G")

	w.applyEngineDefaults("postgres", "sqlserver")

	if w.inputs[1].Value() != "custom-box" || w.inputs[2].Value() != "5555" ||
		w.inputs[4].Value() != "custom_vol" || w.inputs[5].Value() != "my-secret" ||
		w.inputs[6].Value() != "1G" {
		t.Fatalf("edited fields were overwritten: container=%q port=%q vol=%q pass=%q mem=%q",
			w.inputs[1].Value(), w.inputs[2].Value(), w.inputs[4].Value(),
			w.inputs[5].Value(), w.inputs[6].Value())
	}
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestWizardKeysLateralAndVertical(t *testing.T) {
	t.Parallel()
	m := &AppModel{mode: ModeWizard, wizard: newWizardModel("/tmp", "/tmp/instances", nil)}

	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyRight})
	if m.wizard.selectedEngineIdx != 1 {
		t.Fatalf("right should select sqlserver, idx=%d", m.wizard.selectedEngineIdx)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyDown})
	if m.wizard.step != StepEngine {
		t.Fatalf("down must not leave Engine before unlock, got %v", m.wizard.step)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyEnter})
	if m.wizard.step != StepRuntime {
		t.Fatalf("enter -> Runtime, got %v", m.wizard.step)
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyDown})
	if m.wizard.step != StepRuntime {
		t.Fatalf("down must not enter locked Name")
	}
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyUp})
	if m.wizard.step != StepEngine {
		t.Fatalf("up -> Engine, got %v", m.wizard.step)
	}
}

func TestWizardBackAndEmptyBackspace(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.maxReached = StepName
	w.setFocus(StepName)
	m := &AppModel{mode: ModeWizard, wizard: w}

	_, _ = m.updateWizard(key("b"))
	if m.wizard.step != StepRuntime {
		t.Fatalf("b from Name -> Runtime, got %v", m.wizard.step)
	}

	m.wizard.setFocus(StepName)
	m.wizard.inputs[0].SetValue("")
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.wizard.step != StepRuntime {
		t.Fatalf("empty backspace -> Runtime, got %v", m.wizard.step)
	}
}

func TestWizardCycleEngineTriggersRecalc(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[5].SetValue("postgres")
	w.inputs[6].SetValue("512M")
	w.maxReached = StepEngine
	w.setFocus(StepEngine)
	m := &AppModel{mode: ModeWizard, wizard: w}

	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyRight})
	if m.wizard.inputs[1].Value() != "sql-shop" {
		t.Fatalf("container after engine change = %q", m.wizard.inputs[1].Value())
	}
}

func TestWizardConfirmAdvanceUnlocksNext(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if !w.confirmAdvance() {
		t.Fatal("Engine confirm should succeed")
	}
	if w.step != StepRuntime || w.maxReached != StepRuntime {
		t.Fatalf("step=%v max=%v, want Runtime/Runtime", w.step, w.maxReached)
	}
	if !w.confirmAdvance() {
		t.Fatal("Runtime confirm should succeed")
	}
	if w.step != StepName || w.maxReached != StepName {
		t.Fatalf("step=%v max=%v, want Name/Name", w.step, w.maxReached)
	}
	if w.confirmAdvance() {
		t.Fatal("empty Name must not advance")
	}
	w.inputs[0].SetValue("shop")
	if !w.confirmAdvance() {
		t.Fatal("Name with value should advance")
	}
	if w.step != StepContainerName {
		t.Fatalf("step = %v, want ContainerName", w.step)
	}
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
	// One ">" comes from the selected row in the instance list, not wizard inputs.
	if strings.Count(plain, ">") > 1 {
		t.Fatalf("completed wizard fields still show input prompt:\n%s", plain)
	}
}

func TestWizardFooterShowsNavHints(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if !strings.Contains(plain, "[↑↓] rows") || !strings.Contains(plain, "[←→] options") ||
		!strings.Contains(plain, "[b] back") {
		t.Fatalf("footer missing nav hints:\n%s", plain)
	}
}

func TestWizardShowsRuntimeAfterUnlock(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if strings.Contains(plain, "2. Runtime:") {
		t.Fatal("Runtime must stay hidden until unlocked")
	}
	m.wizard.maxReached = StepRuntime
	m.wizard.setFocus(StepRuntime)
	plain = stripANSI(m.viewWizard())
	if !strings.Contains(plain, "2. Runtime:") {
		t.Fatalf("Runtime missing after unlock:\n%s", plain)
	}
}

func TestWizardActiveEngineShowsChips(t *testing.T) {
	t.Parallel()
	m := &AppModel{width: 120, height: 32, mode: ModeWizard}
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	plain := stripANSI(m.viewWizard())
	if !strings.Contains(plain, "[Postgres]") {
		t.Fatalf("active engine should show chip brackets:\n%s", plain)
	}
}

func TestWizardDockedShowsDetailsAndWizard(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.instances = []*core.DatabaseInstance{
		{Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped},
	}
	m.selectedIndex = 0

	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "Details & Config") {
		t.Fatal("expected details panel title")
	}
	if !strings.Contains(plain, "demo") {
		t.Fatal("expected selected instance name in details area")
	}
	if !strings.Contains(plain, "New Database Instance") {
		t.Fatal("expected wizard in dock")
	}
}

func TestWizardDockedFooterShowsWizardHints(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)

	plain := stripANSI(m.viewMain())
	if strings.Contains(plain, "[n]") && strings.Contains(plain, "New") {
		t.Fatal("main [n] New shortcut should not appear while wizard is open")
	}
	if !strings.Contains(plain, "[Esc]") {
		t.Fatal("expected wizard cancel hint in footer area")
	}
}

func TestWizardScrollFollowsFocus(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.scrollViewport = viewport.New(60, 4)
	w.maxReached = StepReview
	w.setFocus(StepMemoryLimit)
	if w.scrollViewport.YOffset == 0 {
		t.Fatal("expected scroll offset > 0 when focusing late step in small viewport")
	}
}

func TestWizardDockFixedHintsOutsideViewport(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 80
	m.height = 20
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.wizard.maxReached = StepReview
	m.wizard.step = StepReview

	plain := stripANSI(m.viewMain())
	if !strings.Contains(plain, "[Esc]") || !strings.Contains(plain, "All set!") {
		t.Fatal("review hints should appear in dock/footer")
	}
}

func TestViewWizardUsesMainLayout(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width = 120
	m.height = 32
	m.mode = ModeWizard
	m.wizard = newWizardModel("/tmp", "/tmp/instances", nil)
	m.instances = []*core.DatabaseInstance{
		{Name: "demo", EngineType: "postgres", Runtime: "docker"},
	}
	m.selectedIndex = 0

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "LOCAL DATABASE MANAGER") {
		t.Fatal("expected main header visible in wizard mode")
	}
	if !strings.Contains(plain, "New Database Instance") {
		t.Fatal("expected wizard title in wizard mode")
	}
	if !strings.Contains(plain, "DB Instances") {
		t.Fatal("expected left panel in wizard mode")
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
