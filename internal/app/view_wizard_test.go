package app

import (
	"os"
	"path/filepath"
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
	m.wizard.inputs[4].SetValue("s3cretPass")
	m.wizard.inputs[5].SetValue("512M")
	return m
}

func TestWizardNewModelStartsAtEngine(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	if w.kind != wizardCreate {
		t.Fatalf("kind = %v, want wizardCreate", w.kind)
	}
	if w.step != StepEngine {
		t.Fatalf("step = %v, want StepEngine", w.step)
	}
	if w.maxReached != StepEngine {
		t.Fatalf("maxReached = %v, want StepEngine", w.maxReached)
	}
}

func TestWizardDerivedVolumeIncludesVersion(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.selectedEngineIdx = 0 // postgres
	for i, v := range core.PostgresVersions {
		if v == "16" {
			w.selectedVersionIdx = i
			break
		}
	}
	w.inputs[0].SetValue("shop")
	if got := w.derivedVolume(); got != "pgdata_shop_16" {
		t.Fatalf("derivedVolume=%q", got)
	}
}

func TestWizardSaveWritesPostgresVersionAndVolume(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := newWizardModel("/tmp", dir, nil)
	w.selectedEngineIdx = 0
	for i, v := range core.PostgresVersions {
		if v == "15" {
			w.selectedVersionIdx = i
			break
		}
	}
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[3].SetValue("shop_db")
	w.inputs[4].SetValue("secret")
	w.inputs[5].SetValue("512M")
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "shop.env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "POSTGRES_VERSION=15") {
		t.Fatalf("missing version: %s", s)
	}
	if !strings.Contains(s, "POSTGRES_VOLUME=pgdata_shop_15") {
		t.Fatalf("missing volume: %s", s)
	}
}

func TestWizardSkipsVersionForSQLServer(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.step = StepRuntime
	w.selectedEngineIdx = 1 // sqlserver
	if !w.confirmAdvance() {
		t.Fatal("runtime confirm")
	}
	if w.step != StepName {
		t.Fatalf("sqlserver should skip Version, got step %v", w.step)
	}
}

func TestWizardPostgresRuntimeGoesToVersion(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.step = StepRuntime
	w.selectedEngineIdx = 0
	if !w.confirmAdvance() {
		t.Fatal("runtime confirm")
	}
	if w.step != StepVersion {
		t.Fatalf("want StepVersion, got %v", w.step)
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
	w.inputs[4].SetValue("postgres")
	w.inputs[5].SetValue("512M")
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
	if w.derivedVolume() != "sqlserver_shop" {
		t.Fatalf("volume = %q, want sqlserver_shop", w.derivedVolume())
	}
	if w.inputs[4].Value() != "SuperPassword123!" {
		t.Fatalf("password = %q", w.inputs[4].Value())
	}
	if w.inputs[5].Value() != "2G" {
		t.Fatalf("memory = %q, want 2G", w.inputs[5].Value())
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
	w.inputs[4].SetValue("my-secret")
	w.inputs[5].SetValue("1G")

	w.applyEngineDefaults("postgres", "sqlserver")

	if w.inputs[1].Value() != "custom-box" || w.inputs[2].Value() != "5555" ||
		w.inputs[4].Value() != "my-secret" || w.inputs[5].Value() != "1G" {
		t.Fatalf("edited fields were overwritten: container=%q port=%q pass=%q mem=%q",
			w.inputs[1].Value(), w.inputs[2].Value(), w.inputs[4].Value(), w.inputs[5].Value())
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
	if m.wizard.step != StepName {
		t.Fatalf("letter b must type into Name, got step %v", m.wizard.step)
	}
	if m.wizard.inputs[0].Value() != "b" {
		t.Fatalf("name=%q, want b", m.wizard.inputs[0].Value())
	}

	m.wizard.inputs[0].SetValue("")
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyUp})
	if m.wizard.step != StepVersion {
		t.Fatalf("up from Name -> Version, got %v", m.wizard.step)
	}

	m.wizard.maxReached = StepName
	m.wizard.setFocus(StepName)
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.wizard.step != StepVersion {
		t.Fatalf("empty backspace -> Version, got %v", m.wizard.step)
	}
}

func TestWizardCycleEngineTriggersRecalc(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp/instances", nil)
	w.inputs[0].SetValue("shop")
	w.inputs[1].SetValue("pg-shop")
	w.inputs[2].SetValue("5432")
	w.inputs[4].SetValue("postgres")
	w.inputs[5].SetValue("512M")
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
	if w.step != StepVersion || w.maxReached != StepVersion {
		t.Fatalf("step=%v max=%v, want Version/Version", w.step, w.maxReached)
	}
	if !w.confirmAdvance() {
		t.Fatal("Version confirm should succeed")
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
		!strings.Contains(plain, "[Esc] cancel") {
		t.Fatalf("footer missing nav hints:\n%s", plain)
	}
	if strings.Contains(plain, "[b] back") {
		t.Fatalf("letter b must not be a back shortcut while typing:\n%s", plain)
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

func TestWizardBodyLineForStepMatchesRenderedRows(t *testing.T) {
	t.Parallel()

	t.Run("postgres password is two lines after database", func(t *testing.T) {
		t.Parallel()
		w := newWizardModel("/tmp", "/tmp/instances", nil)
		w.maxReached = StepReview
		w.selectedEngineIdx = 0 // postgres

		dbLine := w.bodyLineForStep(StepDatabase)
		passLine := w.bodyLineForStep(StepPassword)
		if got := passLine - dbLine; got != 2 {
			t.Fatalf("password line %d - database line %d = %d, want 2 (database row + volume preview)", passLine, dbLine, got)
		}
	})

	t.Run("sqlserver skips version row", func(t *testing.T) {
		t.Parallel()
		w := newWizardModel("/tmp", "/tmp/instances", nil)
		w.maxReached = StepReview
		w.selectedEngineIdx = 1 // sqlserver

		runtimeLine := w.bodyLineForStep(StepRuntime)
		nameLine := w.bodyLineForStep(StepName)
		if got := nameLine - runtimeLine; got != 1 {
			t.Fatalf("name line %d - runtime line %d = %d, want 1 (version step not rendered)", nameLine, runtimeLine, got)
		}
	})
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

func TestEditWizardPrefillsFromInstance(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "podman",
		ContainerName: "pg-shop", Port: 5433, Database: "shop_db",
		Password: "s3cret", Volume: "pgdata_shop", MemoryLimit: "1G",
		EnvFilePath: "/tmp/instances/shop.env",
		Status:      core.StatusStopped,
	}
	w := newEditWizardModel("/tmp", "/tmp/instances", []*core.DatabaseInstance{inst}, inst)
	if w.kind != wizardEdit {
		t.Fatalf("kind=%v", w.kind)
	}
	if w.inputs[0].Value() != "shop" {
		t.Fatalf("name=%q", w.inputs[0].Value())
	}
	if w.engines[w.selectedEngineIdx] != "postgres" {
		t.Fatal("engine")
	}
	if w.runtimes[w.selectedRuntimeIdx] != "podman" {
		t.Fatal("runtime")
	}
	if w.inputs[2].Value() != "5433" || w.inputs[4].Value() != "s3cret" {
		t.Fatalf("port/pass not prefilled")
	}
	if w.sourceName != "shop" || w.sourceEnvPath != "/tmp/instances/shop.env" {
		t.Fatalf("source snapshot missing: name=%q path=%q", w.sourceName, w.sourceEnvPath)
	}
	if w.sourceRuntime != "podman" || w.sourceContainerName != "pg-shop" {
		t.Fatalf("source runtime/container snapshot missing")
	}

	ready := &core.DatabaseInstance{Name: "live", Status: core.StatusReady}
	wReady := newEditWizardModel("/tmp", "/tmp/instances", nil, ready)
	if !wReady.wasRunning {
		t.Fatal("wasRunning should be true for READY status")
	}
}

func TestEditWizardNameUniquenessAllowsSelf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := []*core.DatabaseInstance{{Name: "shop"}, {Name: "other"}}
	inst := &core.DatabaseInstance{Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db", Password: "p", Volume: "v", MemoryLimit: "512M",
		EnvFilePath: filepath.Join(dir, "shop.env")}
	_ = os.WriteFile(inst.EnvFilePath, []byte("ENGINE=postgres\n"), 0644)
	w := newEditWizardModel("/tmp", dir, existing, inst)
	if w.nameTaken("shop") {
		t.Fatal("self name must be allowed")
	}
	if !w.nameTaken("other") {
		t.Fatal("other name must be taken")
	}
}

func TestEditWizardRenameWritesNewDeletesOld(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	_ = os.WriteFile(oldPath, []byte("ENGINE=postgres\n"), 0644)
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db", Password: "p",
		Volume: "v", MemoryLimit: "512M", EnvFilePath: oldPath,
		Status: core.StatusStopped,
	}
	w := newEditWizardModel("/tmp", dir, []*core.DatabaseInstance{inst}, inst)
	w.inputs[0].SetValue("shop2")
	w.inputs[1].SetValue("pg-shop2")
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "shop2.env")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("expected new env")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expected old env removed when not running")
	}
}

func TestEditWizardRenameWhileRunningDefersOldDelete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	_ = os.WriteFile(oldPath, []byte("ENGINE=postgres\n"), 0644)
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db", Password: "p",
		Volume: "v", MemoryLimit: "512M", EnvFilePath: oldPath,
		Status: core.StatusReady,
	}
	w := newEditWizardModel("/tmp", dir, []*core.DatabaseInstance{inst}, inst)
	w.wasRunning = true
	w.inputs[0].SetValue("shop2")
	w.inputs[1].SetValue("pg-shop2")
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shop2.env")); err != nil {
		t.Fatal("expected new env")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("expected old env kept until restart completes or confirm cancelled")
	}
}

func TestEditWizardNameAdvanceDoesNotAutofill(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5433, Database: "shop_db",
		Password: "s3cret", Volume: "pgdata_shop", MemoryLimit: "512M",
		EnvFilePath: "/tmp/instances/shop.env",
	}
	w := newEditWizardModel("/tmp", "/tmp/instances", []*core.DatabaseInstance{inst}, inst)
	w.step = StepName
	if !w.confirmAdvance() {
		t.Fatal("expected advance")
	}
	if w.inputs[2].Value() != "5433" {
		t.Fatalf("port=%q, want 5433 unchanged", w.inputs[2].Value())
	}
	if w.inputs[1].Value() != "pg-shop" {
		t.Fatalf("container=%q, want pg-shop unchanged", w.inputs[1].Value())
	}
}

func TestActionEditOpensEditWizard(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 32
	m.mode = ModeActionMenu
	m.instances = []*core.DatabaseInstance{{
		Name: "demo", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-demo", Port: 5432, Database: "demo",
		Password: "x", Volume: "v", MemoryLimit: "512M",
		Status: core.StatusStopped,
	}}
	m.selectedIndex = 0
	items := m.getActionMenuItems(m.instances[0])
	editIdx := -1
	for i, it := range items {
		if it.label == "Edit Instance" {
			editIdx = i
			break
		}
	}
	if editIdx < 0 {
		t.Fatal("Edit Instance action missing")
	}
	m.actionMenuIndex = editIdx
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if am.mode != ModeWizard || am.wizard.kind != wizardEdit {
		t.Fatalf("mode=%v kind=%v", am.mode, am.wizard.kind)
	}
	plain := stripANSI(am.View())
	if !strings.Contains(plain, "Edit Instance") {
		t.Fatalf("expected edit title:\n%s", plain)
	}
}

func TestEditSaveWhenRunningArmsRestartConfirm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shop.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=v
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := core.ParseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inst.Status = core.StatusReady

	m := NewApp(filepath.Dir(dir), config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.instances = []*core.DatabaseInstance{inst}
	m.selectedIndex = 0
	m.mode = ModeWizard
	m.wizard = newEditWizardModel(m.projectRoot, dir, m.instances, inst)
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if !am.confirmRestartAfterEdit {
		t.Fatal("expected restart confirm armed")
	}
	if am.mode != ModeMain {
		t.Fatalf("expected main after save, got %v", am.mode)
	}
	if am.pendingRestartOld == nil || am.pendingRestartOld.Name != "shop" {
		t.Fatalf("pendingRestartOld=%v", am.pendingRestartOld)
	}
	if am.pendingRestartNewName != "shop" {
		t.Fatalf("pendingRestartNewName=%q", am.pendingRestartNewName)
	}
	if cmd == nil {
		t.Fatal("expected reload cmd")
	}
}

func TestEditRestartConfirmYesStopsThenStarts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	newPath := filepath.Join(dir, "new.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-new
COMPOSE_PROJECT_NAME=pg-new
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=v
`
	if err := os.WriteFile(newPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sr := &stubRunner{}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.runner = sr
	m.confirmRestartAfterEdit = true
	m.pendingRestartOld = &core.DatabaseInstance{
		Name: "old", Runtime: "docker", ProjectName: "pg-old",
		EnvFilePath: "/tmp/old.env", ContainerName: "pg-old",
	}
	m.pendingRestartNewName = "new"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am := updated.(*AppModel)
	if am.confirmRestartAfterEdit {
		t.Fatal("confirm should clear")
	}
	if cmd == nil {
		t.Fatal("expected restart cmd")
	}
	msg := cmd()
	if _, ok := msg.(restartAfterEditDoneMsg); !ok {
		t.Fatalf("got %T, want restartAfterEditDoneMsg", msg)
	}
	if sr.lastStopInst == nil || sr.lastStopInst.Name != "old" {
		t.Fatalf("lastStopInst=%v", sr.lastStopInst)
	}
	if sr.lastStartInst == nil || sr.lastStartInst.Name != "new" {
		t.Fatalf("lastStartInst=%v", sr.lastStartInst)
	}
}

func TestEditRestartConfirmNoClears(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	_ = os.WriteFile(oldPath, []byte("ENGINE=postgres\n"), 0644)

	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.confirmRestartAfterEdit = true
	m.pendingRestartOld = &core.DatabaseInstance{Name: "old"}
	m.pendingRestartNewName = "new"
	m.pendingDeleteEnvPath = oldPath
	m.statusMsg = "Saved. Restart container with new config? Press 'y' to confirm, 'n' to cancel"
	m.statusIsErr = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	am := updated.(*AppModel)
	if am.confirmRestartAfterEdit {
		t.Fatal("confirm should clear")
	}
	if am.pendingRestartOld != nil {
		t.Fatal("pendingRestartOld should be nil")
	}
	if am.pendingDeleteEnvPath != "" {
		t.Fatal("pendingDeleteEnvPath should be cleared")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expected deferred old env removed on cancel")
	}
	if am.mode == ModeWizard {
		t.Fatal("cancel should not open wizard")
	}
	if cmd == nil {
		t.Fatal("expected status clear tick")
	}
}

func TestEditRenameRunningRestartStopsOldEnvThenDeletes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	oldContent := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=v
`
	if err := os.WriteFile(oldPath, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := core.ParseEnvFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	inst.Status = core.StatusReady

	m := NewApp(filepath.Dir(dir), config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.instances = []*core.DatabaseInstance{inst}
	m.selectedIndex = 0
	m.mode = ModeWizard
	m.wizard = newEditWizardModel(m.projectRoot, dir, m.instances, inst)
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview
	m.wizard.inputs[0].SetValue("shop2")
	m.wizard.inputs[1].SetValue("pg-shop2")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(*AppModel)
	if !am.confirmRestartAfterEdit {
		t.Fatal("expected restart confirm armed")
	}
	if am.pendingDeleteEnvPath != oldPath {
		t.Fatalf("pendingDeleteEnvPath=%q, want %q", am.pendingDeleteEnvPath, oldPath)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("old env must still exist before restart Stop")
	}

	sr := &stubRunner{}
	am.runner = sr
	updated, cmd := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am = updated.(*AppModel)
	if cmd == nil {
		t.Fatal("expected restart cmd")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("old env must still exist at Stop time")
	}
	msg := cmd()
	done, ok := msg.(restartAfterEditDoneMsg)
	if !ok {
		t.Fatalf("got %T, want restartAfterEditDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected restart error: %v", done.err)
	}
	if sr.lastStopInst == nil || sr.lastStopInst.EnvFilePath != oldPath {
		t.Fatalf("Stop env path=%q, want %q", sr.lastStopInst.EnvFilePath, oldPath)
	}
	if sr.lastStopInst.ProjectName != "pg-shop" {
		t.Fatalf("Stop project=%q, want pg-shop", sr.lastStopInst.ProjectName)
	}
	if sr.lastStartInst == nil || sr.lastStartInst.Name != "shop2" {
		t.Fatalf("Start inst=%v", sr.lastStartInst)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("old env should remain until restart success is handled")
	}

	updated, reloadCmd := am.Update(done)
	am = updated.(*AppModel)
	if am.pendingDeleteEnvPath != "" {
		t.Fatal("pendingDeleteEnvPath should be cleared after success")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expected old env removed after successful restart")
	}
	if reloadCmd == nil {
		t.Fatal("expected reload cmd after restart success")
	}
}

func TestEditRestartFailureReloadsInstances(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	newPath := filepath.Join(dir, "new.env")
	_ = os.WriteFile(newPath, []byte(`ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-new
COMPOSE_PROJECT_NAME=pg-new
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_VOLUME=v
`), 0644)

	sr := &stubRunner{startErr: os.ErrInvalid}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.runner = sr
	m.confirmRestartAfterEdit = true
	m.pendingRestartOld = &core.DatabaseInstance{
		Name: "old", Runtime: "docker", ProjectName: "pg-old",
		EnvFilePath: "/tmp/old.env", ContainerName: "pg-old",
	}
	m.pendingRestartNewName = "new"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am := updated.(*AppModel)
	msg := cmd().(restartAfterEditDoneMsg)
	if msg.err == nil {
		t.Fatal("expected restart error")
	}
	updated, reloadCmd := am.Update(msg)
	if reloadCmd == nil {
		t.Fatal("expected reload cmd on restart failure")
	}
	if updated.(*AppModel).statusIsErr != true {
		t.Fatal("expected error status")
	}
}

func TestWizardSaveSanitizesSpacesInNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := newWizardModel("/tmp", dir, nil)
	w.selectedEngineIdx = 0
	w.inputs[0].SetValue("My Shop")
	w.inputs[1].SetValue("pg-My Shop")
	w.inputs[2].SetValue("5432")
	w.inputs[3].SetValue("My Shop_db")
	w.inputs[4].SetValue("secret")
	w.inputs[5].SetValue("512M")
	if err := w.saveInstance(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my_shop.env")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "CONTAINER_NAME=pg_my_shop") {
		t.Fatalf("container not sanitized: %s", s)
	}
	if !strings.Contains(s, "POSTGRES_DB=my_shop_db") {
		t.Fatalf("db not sanitized: %s", s)
	}
	if !strings.Contains(s, "POSTGRES_VOLUME=pgdata_my_shop_18") {
		t.Fatalf("volume not sanitized: %s", s)
	}
}

func TestWizardVimKeysDoNotStealTextInput(t *testing.T) {
	t.Parallel()
	w := newWizardModel("/tmp", "/tmp", nil)
	w.maxReached = StepName
	w.setFocus(StepName)
	m := &AppModel{mode: ModeWizard, wizard: w}
	for _, letter := range []string{"j", "k", "h", "l", "b", "o"} {
		before := m.wizard.inputs[0].Value()
		_, _ = m.updateWizard(key(letter))
		if m.wizard.step != StepName {
			t.Fatalf("%q changed step to %v", letter, m.wizard.step)
		}
		if m.wizard.inputs[0].Value() != before+letter {
			t.Fatalf("%q not typed: got %q", letter, m.wizard.inputs[0].Value())
		}
	}
}

func TestEditWizardPreloadsPostgresVersion(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db",
		Password: "p", Volume: "pgdata_shop_16", MemoryLimit: "512M",
		Version: "16", EnvFilePath: "/tmp/shop.env",
	}
	w := newEditWizardModel("/tmp", "/tmp", nil, inst)
	if w.selectedVersion() != "16" {
		t.Fatalf("version=%q", w.selectedVersion())
	}
}

func TestEditSaveWarnsWhenVolumeChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	old := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_VERSION=16
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_shop_16
`
	if err := os.WriteFile(oldPath, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := core.ParseEnvFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.wizard = newEditWizardModel(dir, dir, []*core.DatabaseInstance{inst}, inst)
	m.mode = ModeWizard
	for i, v := range core.PostgresVersions {
		if v == "18" {
			m.wizard.selectedVersionIdx = i
			break
		}
	}
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview
	_, cmd := m.updateWizard(tea.KeyMsg{Type: tea.KeyEnter})
	_ = cmd
	if !strings.Contains(m.statusMsg, "pgdata_shop_18") || !strings.Contains(m.statusMsg, "pgdata_shop_16") {
		t.Fatalf("expected volume change warning, got %q", m.statusMsg)
	}
}

func TestEditSaveVolumeChangeWhileRunningCombinedRestartStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "shop.env")
	old := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_VERSION=16
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=p
POSTGRES_DB=db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_shop_16
`
	if err := os.WriteFile(oldPath, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := core.ParseEnvFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	inst.Status = core.StatusReady

	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.wizard = newEditWizardModel(dir, dir, []*core.DatabaseInstance{inst}, inst)
	m.mode = ModeWizard
	for i, v := range core.PostgresVersions {
		if v == "18" {
			m.wizard.selectedVersionIdx = i
			break
		}
	}
	m.wizard.step = StepReview
	m.wizard.maxReached = StepReview
	_, _ = m.updateWizard(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.confirmRestartAfterEdit {
		t.Fatal("expected restart confirm armed")
	}
	if !strings.Contains(m.statusMsg, "pgdata_shop_18") || !strings.Contains(m.statusMsg, "old volume kept") {
		t.Fatalf("expected combined volume restart status, got %q", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "Restart container with new config?") {
		t.Fatalf("expected restart prompt in status, got %q", m.statusMsg)
	}
}

func TestEditWizardRuntimeIsLocked(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		ContainerName: "pg-shop", Port: 5432, Database: "db",
		Password: "p", Volume: "pgdata_shop_18", MemoryLimit: "512M",
		Version: "18", EnvFilePath: "/tmp/shop.env",
	}
	w := newEditWizardModel("/tmp", "/tmp", nil, inst)
	if w.selectedRuntimeIdx != 0 {
		t.Fatalf("runtime idx=%d, want docker(0)", w.selectedRuntimeIdx)
	}

	w.step = StepEngine
	w.maxReached = StepReview
	if !w.confirmAdvance() {
		t.Fatal("engine confirm")
	}
	if w.step != StepVersion {
		t.Fatalf("edit should skip Runtime focus, got step %v", w.step)
	}

	w.step = StepRuntime
	w.cycleOption(1)
	if w.selectedRuntimeIdx != 0 {
		t.Fatalf("cycle must not change locked runtime, got %d", w.selectedRuntimeIdx)
	}

	w.step = StepEngine
	w.moveFocus(1)
	if w.step != StepVersion {
		t.Fatalf("moveFocus should skip Runtime, got %v", w.step)
	}
}

func TestEditWizardShowsRuntimeLockedPreview(t *testing.T) {
	t.Parallel()
	inst := &core.DatabaseInstance{
		Name: "shop", EngineType: "postgres", Runtime: "podman",
		ContainerName: "pg-shop", Port: 5432, Database: "db",
		Password: "p", Volume: "pgdata_shop_18", MemoryLimit: "512M",
		Version: "18", EnvFilePath: "/tmp/shop.env", Status: core.StatusStopped,
	}
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.width, m.height = 120, 36
	m.wizard = newEditWizardModel("/tmp", "/tmp", nil, inst)
	m.mode = ModeWizard
	plain := stripANSI(m.viewWizardDock(60, 20))
	if !strings.Contains(plain, "locked") {
		t.Fatalf("expected locked runtime preview, got %q", plain)
	}
	if !strings.Contains(plain, "Podman") && !strings.Contains(plain, "podman") {
		t.Fatalf("expected Podman label, got %q", plain)
	}
}
