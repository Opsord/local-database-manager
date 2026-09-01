# Details Panel Double-Click Token Copy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Double-click a word (token) in the right-hand instance details panel to copy that token and show a short status notification, without changing `Ctrl+C`, `c`, or `E`/`x`.

**Architecture:** While building details, produce absolute screen hit rectangles (`copyHit`) for each value token. On `tea.MouseMsg` left press, detect double-click (500ms, +/-1 cell), hit-test, call `core.CopyToClipboard`, set `statusMsg`. Pure helpers are unit-tested; origin math reuses `screenInnerWidth` / `splitPanelWidths` / `panelInnerWidth` / `mainContentHeight` / `splitPanelHalfHeight`.

**Tech Stack:** Go 1.22+, Bubble Tea v0.26 (`tea.MouseMsg`, `MouseActionPress`, `MouseButtonLeft`), Lip Gloss, `core.CopyToClipboard`.

**Spec:** `docs/superpowers/specs/2026-09-01-details-double-click-copy-design.md`

## Global Constraints

- User-facing strings in English (AGENTS.md).
- Success: `Copied: <token>` (token = copied text).
- Failure: `Failed to copy: %v` (same style as existing clipboard failures in `view_main.go`).
- Clear status with `tea.Tick(3*time.Second, … clearStatusMsg{})` like `c` / `E`.
- Do **not** change `c`, `E`/`x`, `q`, or `ctrl+c`.
- No drag selection; no persistent highlight.
- Labels and `[c] copy` are not hits.
- Copy **visible** token text only (after truncate); full URI still via `c`.
- Hits only in details region; when wizard/action dock is open, clip with `maxYExclusive` so the dock is not hittable.
- Mouse already enabled: `tea.WithMouseCellMotion()` in `cmd/db-manager/main.go` — keep it.
- TDD: failing test first; run `go test` after each green step.
- Do not commit unless the user asks.

---

## File map

| File | Role |
|------|------|
| `internal/app/detail_copy.go` (new) | `copyHit`, tokenize, hit-test, click tracker, relative/absolute hit builders |
| `internal/app/detail_copy_test.go` (new) | Unit + wiring tests |
| `internal/app/helpers.go` | Extract plain field list shared by `renderDetailRows` and hit builder; keep `LabelStyle` width 14 |
| `internal/app/view_main.go` | Refresh hits from details layout; mouse path used by main view |
| `internal/app/tui.go` | `AppModel` fields (`detailHits`, `detailClick`); early `tea.MouseMsg` for modes that show details |
| `internal/app/view_modal.go` | One help line for double-click copy |

---

### Task 1: Tokenize + hit-test + double-click (pure)

**Files:**
- Create: `internal/app/detail_copy.go`
- Test: `internal/app/detail_copy_test.go`

**Interfaces:**
- Produces:
  - `type copyHit struct { X, Y, W, H int; Text string }`
  - `type valueToken struct { Start, End int; Text string }` // byte indices into plain value
  - `func tokenizeValue(s string) []valueToken`
  - `func hitTest(hits []copyHit, x, y int) (string, bool)` — first containing rect
  - `type clickTracker struct { x, y int; at time.Time; armed bool }`
  - `func (t *clickTracker) register(x, y int, now time.Time) bool` — **500ms** window, **+/-1** cell; true means double-click and disarm; else store as first click
  - Token runes: letters, digits, `_./:@%+-`. Everything else (space, `()`, etc.) separates tokens.

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"testing"
	"time"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run "TestTokenizeValue|TestHitTest|TestClickTrackerDoubleClick" -count=1`

Expected: FAIL (undefined: `tokenizeValue` / `copyHit` / `clickTracker`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/app/detail_copy.go`:

```go
package app

import (
	"time"
	"unicode"
	"unicode/utf8"
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run "TestTokenizeValue|TestHitTest|TestClickTrackerDoubleClick" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit only if the user asked**

---

### Task 2: Relative token hits for one plain value

**Files:**
- Modify: `internal/app/detail_copy.go`
- Test: `internal/app/detail_copy_test.go`

**Interfaces:**
- Consumes: `tokenizeValue`
- Produces:
  - `func displayWidth(s string) int` → `lipgloss.Width(s)`
  - `func appendValueTokenHits(dst []copyHit, originX, originY int, plainValue string) []copyHit`
  - `const labelColumnWidth = 15` // `LabelStyle` Width(14) + 1 gap in `detailField`
  - `func valueOriginX(fieldOriginX int) int` → `fieldOriginX + labelColumnWidth`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestAppendValueTokenHits -count=1`

Expected: FAIL (undefined `appendValueTokenHits`).

- [ ] **Step 3: Write minimal implementation**

```go
import "github.com/charmbracelet/lipgloss"

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestAppendValueTokenHits -count=1`

Expected: PASS.

- [ ] **Step 5: Commit only if the user asked**

---

### Task 3: Full details hit map + keep render in sync

**Files:**
- Modify: `internal/app/detail_copy.go`
- Modify: `internal/app/helpers.go` (`renderDetailRows` + shared plain fields)
- Test: `internal/app/detail_copy_test.go` (keep existing `TestRenderDetailRows*` green)

**Interfaces:**
- Produces:
  - `type plainDetailField struct { Label, Value string }` // Value = **plain** text
  - `func plainDetailFields(inst *core.DatabaseInstance, panelWidth int) []plainDetailField`
    - Same order as today’s details: Engine, Version (postgres only), Container, Status, Memory, Database, Port, User, Schema, Volume, Project
    - Engine: `fmt.Sprintf("%s (%s)", strings.ToUpper(inst.EngineType), strings.ToUpper(inst.Runtime))`
    - Memory: `fmt.Sprintf("%s (Limit: %s)", inst.MemoryUsage, inst.MemoryLimit)`
    - Status plain for hits: `"RUNNING"` / `"STARTING"` / `"UNKNOWN"` / `"STOPPED"` (match the word users double-click; bullets in the styled label are not tokens)
    - Container/Volume/Project: same `truncateEnd(..., panelWidth-20)` as render
  - `func buildDetailHits(inst *core.DatabaseInstance, panelWidth, rightInner, originX, originY, maxYExclusive int) []copyHit`
    - Single column if `panelWidth < 70`, else two columns with `colGap := 3` and same width math as `renderDetailRows`
    - Each field: `appendValueTokenHits(..., valueOriginX(fieldX), rowY, field.Value)`
    - After fields: one blank row, then URI plain = `truncateMiddle(inst.ConnectionURI(), codeBoxWidth)`, then CLI plain = `truncateMiddle(inst.CLICommand(), codeBoxWidth)` with `codeBoxWidth` matching `buildRightDetailsContent` (`rightInner-16`, min 20)
    - Drop hits where `Y < originY` or `Y >= maxYExclusive`
  - Refactor `renderDetailRows` to style values from `plainDetailFields` so layout cannot drift (Status still uses `statusLabel` for display; hits use plain status words)

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run "TestBuildDetailHits" -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement plain fields + `buildDetailHits`; refactor `renderDetailRows`**

Keep visual output identical. Existing tests such as `TestRenderDetailRowsIncludesPostgresVersion` / omit-version-for-sqlserver must still pass.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run "TestBuildDetailHits|TestRenderDetailRows" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit only if the user asked**

---

### Task 4: Absolute origin, mouse handler, status

**Files:**
- Modify: `internal/app/tui.go`
- Modify: `internal/app/view_main.go`
- Modify: `internal/app/detail_copy.go` (origin helper may live here)
- Test: `internal/app/detail_copy_test.go`

**Interfaces:**
- On `AppModel`:
  - `detailHits []copyHit`
  - `detailClick clickTracker`
  - `func (m *AppModel) detailsContentOrigin() (originX, originY, maxYExclusive int)`
    - Wrap pad X = 1; header = 1 row
    - Left outer width = `leftWidth + 2` (borders outside `Width`)
    - Then gap + right border 1 + panel pad 1 → content X of right panel
    - Content Y = header + top border 1 + title row 1
    - `maxYExclusive`: bottom of details content; when `ModeWizard` or `ModeActionMenu`, use top half from `splitPanelHalfHeight(contentHeight-1)` (same as view) so dock is excluded
  - `func (m *AppModel) refreshDetailHits()`
    - No selection → `detailHits = nil`
    - Else `buildDetailHits(inst, rightWidth, rightInner, ox, oy, maxY)`
  - `func (m *AppModel) handleDetailsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool)`
    - Delegates to `handleDetailsMouseAt(msg, time.Now())` for testability
  - `func (m *AppModel) handleDetailsMouseAt(msg tea.MouseMsg, now time.Time) (tea.Model, tea.Cmd, bool)`
    - Only `MouseButtonLeft` + `MouseActionPress`
    - `refreshDetailHits()` then `detailClick.register(msg.X, msg.Y, now)`
    - If not double → `(m, nil, false)`
    - If double + hit → `core.CopyToClipboard(text)`; set `statusMsg` / `statusIsErr`; return 3s clear tick; `handled=true`
    - If double + miss → `(m, nil, true)` (consume double, no status)

Dispatch in `AppModel.Update` **before** mode switch when mode is one of `{ModeMain, ModeWizard, ModeActionMenu, ModeEngineMenu}` (all show details via `viewMain`). Skip `ModeLogs` / `ModeHelp`.

```go
case tea.MouseMsg:
	switch m.mode {
	case ModeMain, ModeWizard, ModeActionMenu, ModeEngineMenu:
		model, cmd, handled := m.handleDetailsMouse(msg)
		if handled {
			return model, cmd
		}
	}
```

Success/failure:

```go
if err := core.CopyToClipboard(text); err != nil {
	m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
	m.statusIsErr = true
} else {
	m.statusMsg = fmt.Sprintf("Copied: %s", text)
	m.statusIsErr = false
}
return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }), true
```

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run "TestDetailsContentOrigin|TestHandleDetailsMouse" -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement origin, refresh, mouse handler, Update dispatch**

Do not edit `c` / `E` / quit key branches.

- [ ] **Step 4: Run full package tests**

Run: `go test ./internal/app/ -count=1`

Expected: PASS.

- [ ] **Step 5: Manual smoke (recommended)**

Run: `go run ./cmd/db-manager`

- Double-click user `postgres` → `Copied: postgres`
- Double-click a label → no copy status
- `c` still copies full URI
- `q` / `ctrl+c` still quit

- [ ] **Step 6: Commit only if the user asked**

---

### Task 5: Help text

**Files:**
- Modify: `internal/app/view_modal.go`

**Interfaces:**
- Add help row (English): `Double-click a value token in Details to copy it`

- [ ] **Step 1: Add the help entry near copy shortcuts**

- [ ] **Step 2: Run:** `go test ./internal/app/ -count=1` — expect PASS

- [ ] **Step 3: Commit only if the user asked**

---

## Spec coverage

| Spec requirement | Task |
|------------------|------|
| Double-click copies token + `Copied: …` | 1, 4 |
| Labels / `[c] copy` not copyable | 3 |
| Outside tokens / no selection → no-op | 4 |
| Clipboard error status | 4 |
| Details only; dock excluded | 3–4 (`maxYExclusive`) |
| 500ms / +/-1 double-click | 1 |
| Visible truncated text only | 3 |
| `c` / `E` / `Ctrl+C` unchanged | 4 |
| English copy | Global + 4–5 |
| Unit tests tokenize / hit / double-click | 1–3 |

## Self-review

- No TBD/placeholder steps.
- Names match codebase: `AppModel`, `DatabaseInstance`, `CopyToClipboard`, `ConnectionURI`, `CLICommand`, `clearStatusMsg`, `statusMsg` / `statusIsErr`, `NewApp`, `renderDetailRows`, `buildRightDetailsContent`, `screenInnerWidth`, `splitPanelWidths`, `panelInnerWidth`, `mainContentHeight`, `splitPanelHalfHeight`.
- `handleDetailsMouseAt` keeps double-click tests deterministic.
- Existing `TestRenderDetailRows*` remain the visual regression net after the plain-field refactor.
