# Details Copy Token Highlight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a successful details-panel double-click copy, highlight that token in the Details view for ~3 seconds (same window as `Copied: …`), then clear it with status.

**Architecture:** Store the successful `copyHit` on `AppModel`. On render, paint value strings token-by-token and apply `CopiedTokenStyle` to the segment whose absolute cell rect matches `copiedHit`. Clear highlight with `clearStatusMsg`, on selection change, and on clipboard failure (do not set / clear previous).

**Tech Stack:** Go 1.22+, Bubble Tea, Lip Gloss (`SelectedBg`), existing `detail_copy.go` hit map.

**Spec:** `docs/superpowers/specs/2026-09-01-details-copy-highlight-design.md`

## Global Constraints

- English UI only (AGENTS.md).
- Highlight only after **successful** double-click token copy (not `c` / `E` / failed clipboard).
- Lifetime ~3s, same clear path as copy status (`clearStatusMsg` after `tea.Tick(3*time.Second, …)`).
- Match hit identity (rect + text), not first substring match.
- Do not change `c`, `E`/`x`, `q`, or `ctrl+c`.
- TDD: failing test first; `go test` after each green step.
- Do not commit unless the user asks (or the execution session explicitly allows feature commits).
- Leave unrelated WIP (wizard/runner/podman/etc.) unstaged.

---

## File map

| File | Role |
|------|------|
| `internal/app/tui.go` | `copiedHit *copyHit`; clear on `clearStatusMsg` |
| `internal/app/detail_copy.go` | `findCopyHit`; set/clear helpers; set on successful copy |
| `internal/app/view_main.go` | Clear highlight on selection change; pass highlight into details render |
| `internal/app/helpers.go` | Token-aware value rendering with optional highlight |
| `internal/app/styles.go` | `CopiedTokenStyle` |
| `internal/app/detail_copy_test.go` | State + clear + selection tests |
| `internal/app/helpers_test.go` or `detail_copy_test.go` | Render includes highlight styling for copied span |

---

### Task 1: Store and clear `copiedHit`

**Files:**
- Modify: `internal/app/tui.go`
- Modify: `internal/app/detail_copy.go`
- Modify: `internal/app/view_main.go` (selection change clear)
- Test: `internal/app/detail_copy_test.go`

**Interfaces:**
- Consumes: existing `copyHit`, `hitTest` / `detailHits`, `handleDetailsMouseAt`, `clearStatusMsg`
- Produces:
  - `AppModel.copiedHit *copyHit`
  - `func findCopyHit(hits []copyHit, x, y int) (copyHit, bool)` — first containing rect (same geometry as `hitTest`)
  - `func (m *AppModel) clearCopiedHit()` — sets `copiedHit = nil`
  - On successful clipboard write in `handleDetailsMouseAt`: `h, ok := findCopyHit(...); if ok { cp := h; m.copiedHit = &cp }`
  - On clipboard error: `m.clearCopiedHit()` (no highlight) then set error status as today
  - `case clearStatusMsg:` also call `m.clearCopiedHit()`
  - When `selectedIndex` changes in `updateMain` (up/down/j/k and filter nav): `m.clearCopiedHit()`

- [ ] **Step 1: Write the failing tests**

```go
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
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // or KeyDown — use the same KeyMsg shape updateMain expects for "down"/"j"
	// Prefer: send tea.KeyMsg with String "j" if that is how Bubble Tea delivers letters in tests elsewhere in this package.
	am := updated.(*AppModel)
	if am.copiedHit != nil {
		t.Fatalf("expected clear on selection change, got %#v", am.copiedHit)
	}
}
```

Mirror existing key-msg construction from `view_delete_test.go` / other tests (use whatever pattern already drives `updateMain` with `"j"` / `"down"`).

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/app/ -run "TestSuccessfulCopySetsCopiedHit|TestClearStatusMsgClearsCopiedHit|TestSelectionChangeClearsCopiedHit" -count=1`

Expected: FAIL (undefined `copiedHit` / behavior missing).

- [ ] **Step 3: Minimal implementation**

In `tui.go` on `AppModel`:

```go
copiedHit *copyHit
```

In `detail_copy.go`:

```go
func findCopyHit(hits []copyHit, x, y int) (copyHit, bool) {
	for _, h := range hits {
		if y >= h.Y && y < h.Y+h.H && x >= h.X && x < h.X+h.W {
			return h, true
		}
	}
	return copyHit{}, false
}

func (m *AppModel) clearCopiedHit() {
	m.copiedHit = nil
}
```

In `handleDetailsMouseAt` success path:

```go
if err := core.CopyToClipboard(text); err != nil {
	m.clearCopiedHit()
	m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
	m.statusIsErr = true
} else {
	if h, ok := findCopyHit(m.detailHits, msg.X, msg.Y); ok {
		cp := h
		m.copiedHit = &cp
	}
	m.statusMsg = fmt.Sprintf("Copied: %s", text)
	m.statusIsErr = false
}
```

In `clearStatusMsg` case:

```go
case clearStatusMsg:
	m.statusMsg = ""
	m.clearCopiedHit()
	return m, nil
```

In `updateMain`, whenever `selectedIndex` changes (up/down/j/k and filter list nav), call `m.clearCopiedHit()` before returning.

Optionally refactor `hitTest` to call `findCopyHit` to avoid duplication.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/app/ -run "TestSuccessfulCopySetsCopiedHit|TestClearStatusMsgClearsCopiedHit|TestSelectionChangeClearsCopiedHit|TestHandleDetailsMouse" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit only if user asked**

```bash
git add internal/app/tui.go internal/app/detail_copy.go internal/app/detail_copy_test.go internal/app/view_main.go
git commit -m "feat(tui): track copied details token for highlight"
```

---

### Task 2: Render highlight on the copied token

**Files:**
- Modify: `internal/app/styles.go`
- Modify: `internal/app/helpers.go`
- Modify: `internal/app/view_main.go` (`buildRightDetailsContent`)
- Modify: `internal/app/detail_copy.go` (shared match helper if useful)
- Test: `internal/app/detail_copy_test.go` and/or `helpers_test.go`

**Interfaces:**
- Consumes: `copiedHit`, `tokenizeValue`, `displayWidth`, `valueOriginX`, `labelColumnWidth`, `plainDetailFields`, `detailsContentOrigin`, layout math from `buildDetailHits` / `renderDetailRows`
- Produces:
  - `CopiedTokenStyle` in `styles.go`: `Foreground(FgText).Background(SelectedBg).Bold(true)` (keep `Background` on every fragment per existing surface rules)
  - `func hitsEqual(a, b copyHit) bool` — same `X,Y,W,H,Text`
  - `func styleValueWithCopiedHit(plain string, fieldOriginX, rowY int, base lipgloss.Style, copied *copyHit) string`
    - Walk plain with `tokenizeValue`; for separators use `base` (or surface bg); for each token build provisional `copyHit{X: fieldOriginX+displayWidth(prefix), Y: rowY, W: displayWidth(tok.Text), H: 1, Text: tok.Text}`; if `copied != nil && hitsEqual(*copied, provisional)` render with `CopiedTokenStyle`, else `base`
  - `func renderDetailRowsWithCopiedHit(inst *core.DatabaseInstance, panelWidth int, originX, originY int, copied *copyHit) []string`
    - Same layout as `renderDetailRows` (single vs two-column), but values via `styleValueWithCopiedHit` / Status: build from `statusDisplayPlain` segments with Status’s normal `statusLabel` when not highlighting a token — when highlighting, prefer painting `statusDisplayPlain` token-by-token so the RUNNING word can use `CopiedTokenStyle` while keeping icon unstyled or base-styled
  - URI/CLI rows in `buildRightDetailsContent`: also use `styleValueWithCopiedHit` on truncated plain strings at the correct `valueOriginX(originX)` and row Y (after field rows + blank)
  - `buildRightDetailsContent`: `ox, oy, _ := m.detailsContentOrigin()`; render with `m.copiedHit`

Keep `renderDetailRows` as a thin wrapper calling `renderDetailRowsWithCopiedHit(..., nil)` so existing tests stay green without highlight.

- [ ] **Step 1: Write the failing test**

```go
func TestStyleValueWithCopiedHitHighlightsMatchingToken(t *testing.T) {
	plain := "POSTGRES (DOCKER)"
	originX, rowY := 100, 7
	hits := appendValueTokenHits(nil, originX, rowY, plain)
	if len(hits) < 2 {
		t.Fatal("expected tokens")
	}
	copied := hits[1] // DOCKER
	base := ValueStyle
	out := styleValueWithCopiedHit(plain, originX, rowY, base, &copied)
	// Highlighted output must differ from unstyled rendering of the same plain value
	plainOut := styleValueWithCopiedHit(plain, originX, rowY, base, nil)
	if out == plainOut {
		t.Fatal("expected highlighted render to differ when copiedHit matches")
	}
	if !strings.Contains(out, "DOCKER") {
		t.Fatalf("missing token text: %q", out)
	}
}

func TestBuildRightDetailsContentIncludesHighlightWhenCopiedHitSet(t *testing.T) {
	m := NewApp(t.TempDir(), config.Config{})
	m.width, m.height = 120, 40
	m.instances = []*core.DatabaseInstance{{
		Name: "shop", EngineType: "postgres", Runtime: "docker",
		User: "postgres", Port: 5432, Status: core.StatusReady,
		MemoryUsage: "-", MemoryLimit: "-", Database: "shop", Schema: "public", Version: "16",
		ContainerName: "c", Volume: "v", ProjectName: "p",
	}}
	m.selectedIndex = 0
	m.refreshDetailHits()
	var userHit *copyHit
	for i := range m.detailHits {
		if m.detailHits[i].Text == "postgres" {
			h := m.detailHits[i]
			userHit = &h
			break
		}
	}
	if userHit == nil {
		t.Fatal("expected postgres hit")
	}
	m.copiedHit = userHit
	inner := screenInnerWidth(m.width)
	_, rightWidth, _ := splitPanelWidths(inner)
	rightInner := panelInnerWidth(rightWidth)
	withHL := m.buildRightDetailsContent(rightInner, rightWidth)
	m.copiedHit = nil
	without := m.buildRightDetailsContent(rightInner, rightWidth)
	if withHL == without {
		t.Fatal("expected details content to change when copiedHit is set")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/app/ -run "TestStyleValueWithCopiedHit|TestBuildRightDetailsContentIncludesHighlight" -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement styles + token render + wire `buildRightDetailsContent`**

```go
// styles.go
CopiedTokenStyle = lipgloss.NewStyle().
	Foreground(FgText).
	Background(SelectedBg).
	Bold(true)
```

Implement `styleValueWithCopiedHit` and `renderDetailRowsWithCopiedHit` carefully so column widths / `surfaceLine` wrapping stay identical to today when `copied == nil`.

For Status when `copied != nil`: render from `statusDisplayPlain` with per-token styles (icon chars are separators / non-tokens) so the word can highlight; when `copied == nil`, keep using `statusLabel(inst.Status)` for visual parity with today.

- [ ] **Step 4: Run package tests**

Run: `go test ./internal/app/ -count=1`

Expected: PASS (including `TestRenderDetailRows*`).

- [ ] **Step 5: Manual smoke (recommended)**

Run: `go run ./cmd/db-manager`

- Double-click `postgres` under User → status + token highlight ~3s
- Highlight clears with status
- Failed clipboard path (if simulable) → no highlight
- Change instance while highlighted → highlight clears
- `c` still copies URI without highlight

- [ ] **Step 6: Commit only if user asked**

```bash
git add internal/app/styles.go internal/app/helpers.go internal/app/view_main.go internal/app/detail_copy.go internal/app/detail_copy_test.go internal/app/helpers_test.go
git commit -m "feat(tui): highlight copied details token for 3s"
```

---

## Spec coverage

| Spec requirement | Task |
|------------------|------|
| Highlight after successful double-click copy | 1–2 |
| ~3s same clear as status | 1 (`clearStatusMsg`) |
| New copy moves highlight / restarts timer | 1 (overwrite `copiedHit` + new tick) |
| Clipboard failure → no highlight | 1 |
| Instance / selection change clears | 1 |
| Match hit identity not first substring | 2 (`hitsEqual` on absolute rect) |
| Style uses selection chrome | 2 (`CopiedTokenStyle` / `SelectedBg`) |
| `c` / `E` / quit unchanged | Global |
| Unit tests for set/clear/selection | 1 |
| Optional render differs with highlight | 2 |

## Self-review

- No TBD/placeholder steps.
- Names match codebase: `AppModel`, `copyHit`, `clearStatusMsg`, `handleDetailsMouseAt`, `buildRightDetailsContent`, `SelectedBg`, `statusDisplayPlain`.
- `findCopyHit` / `hitsEqual` / `styleValueWithCopiedHit` / `copiedHit` naming consistent across tasks.
- `clearStatusMsg` clearing highlight is intentional for all status clears (safe; highlight only set on token copy).
