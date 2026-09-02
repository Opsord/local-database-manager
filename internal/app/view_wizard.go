package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wizardKind int

const (
	wizardCreate wizardKind = iota
	wizardEdit
)

type wizardStep int

const (
	StepEngine wizardStep = iota
	StepRuntime
	StepVersion
	StepName
	StepContainerName
	StepPort
	StepDatabase
	StepPassword
	StepMemoryLimit
	StepReview
)

type wizardModel struct {
	projectRoot  string
	instancesDir string
	instances    []*core.DatabaseInstance

	kind wizardKind

	sourceName          string
	sourceEnvPath       string
	sourceRuntime       string
	sourceProjectName   string
	sourceContainerName string
	sourceEngine        string
	sourceVolume        string
	wasRunning          bool

	step wizardStep

	maxReached wizardStep

	selectedEngineIdx  int
	selectedRuntimeIdx int
	selectedVersionIdx int

	engines  []string
	runtimes []string

	inputs []textinput.Model

	scrollViewport viewport.Model
}

func engineDisplay(id string) string {
	if id == "sqlserver" {
		return "SQL Server"
	}
	return "Postgres"
}

func runtimeDisplay(id string) string {
	if id == "podman" {
		return "Podman"
	}
	return "Docker"
}

func defaultPostgresVersionIdx() int {
	for i, v := range core.PostgresVersions {
		if v == core.DefaultPostgresVersion {
			return i
		}
	}
	return len(core.PostgresVersions) - 1
}

func (w *wizardModel) derivedVolume() string {
	name := core.SanitizeIdent(w.inputs[0].Value())
	engine := w.engines[w.selectedEngineIdx]
	ver := ""
	if engine == "postgres" {
		ver = w.selectedVersion()
	}
	return core.DeriveVolumeName(engine, name, ver)
}

func (w *wizardModel) selectedVersion() string {
	if w.selectedVersionIdx < 0 || w.selectedVersionIdx >= len(core.PostgresVersions) {
		return core.DefaultPostgresVersion
	}
	return core.PostgresVersions[w.selectedVersionIdx]
}

func (w *wizardModel) isPostgres() bool {
	return w.engines[w.selectedEngineIdx] == "postgres"
}

func (w *wizardModel) adjustStepForEngine(step wizardStep) wizardStep {
	if w.kind == wizardEdit && step == StepRuntime {
		if w.isPostgres() {
			return StepVersion
		}
		return StepName
	}
	if !w.isPostgres() && step == StepVersion {
		return StepName
	}
	return step
}

func (w *wizardModel) runtimeLocked() bool {
	return w.kind == wizardEdit
}

func (w *wizardModel) blurAll() {
	for i := range w.inputs {
		w.inputs[i].Blur()
	}
}

func (w *wizardModel) focusInput(i int) {
	w.blurAll()
	if i >= 0 && i < len(w.inputs) {
		w.inputs[i].Focus()
	}
}

func newWizardModel(projectRoot, instancesDir string, existing []*core.DatabaseInstance) wizardModel {
	engines := []string{"postgres", "sqlserver"}
	runtimes := []string{"docker", "podman"}

	inputs := make([]textinput.Model, 6)

	inputs[0] = styleTextInput(textinput.New())
	inputs[0].Placeholder = "my_new_instance"

	inputs[1] = styleTextInput(textinput.New())
	inputs[1].Placeholder = "pg-my-new-instance"

	inputs[2] = styleTextInput(textinput.New())
	freePort := core.FindNextFreePort(5432, existing)
	inputs[2].SetValue(strconv.Itoa(freePort))

	inputs[3] = styleTextInput(textinput.New())
	inputs[3].Placeholder = "my_new_db"

	inputs[4] = styleTextInput(textinput.New())
	inputs[4].SetValue("postgres")

	inputs[5] = styleTextInput(textinput.New())
	inputs[5].SetValue("512M")

	for i := range inputs {
		inputs[i].Prompt = ""
		inputs[i].Width = 32
	}

	w := wizardModel{
		projectRoot:        projectRoot,
		instancesDir:       instancesDir,
		instances:          existing,
		kind:               wizardCreate,
		step:               StepEngine,
		maxReached:         StepEngine,
		selectedEngineIdx:  0,
		selectedRuntimeIdx: 0,
		selectedVersionIdx: defaultPostgresVersionIdx(),
		engines:            engines,
		runtimes:           runtimes,
		inputs:             inputs,
		scrollViewport:     viewport.New(60, 8),
	}
	w.blurAll()
	return w
}

func newEditWizardModel(projectRoot, instancesDir string, existing []*core.DatabaseInstance, inst *core.DatabaseInstance) wizardModel {
	w := newWizardModel(projectRoot, instancesDir, existing)
	w.kind = wizardEdit
	w.sourceName = inst.Name
	w.sourceEnvPath = inst.EnvFilePath
	w.sourceRuntime = inst.Runtime
	w.sourceProjectName = inst.ProjectName
	w.sourceContainerName = inst.ContainerName
	w.sourceEngine = inst.EngineType
	w.sourceVolume = inst.Volume
	w.wasRunning = inst.Status == core.StatusReady || inst.Status == core.StatusStarting

	for i, e := range w.engines {
		if e == inst.EngineType {
			w.selectedEngineIdx = i
			break
		}
	}
	for i, r := range w.runtimes {
		if r == inst.Runtime {
			w.selectedRuntimeIdx = i
			break
		}
	}

	w.inputs[0].SetValue(inst.Name)
	w.inputs[1].SetValue(inst.ContainerName)
	w.inputs[2].SetValue(strconv.Itoa(inst.Port))
	w.inputs[3].SetValue(inst.Database)
	w.inputs[4].SetValue(inst.Password)
	if inst.MemoryLimit != "" {
		w.inputs[5].SetValue(inst.MemoryLimit)
	}
	if inst.EngineType == "postgres" && inst.Version != "" {
		normalized := core.NormalizePostgresVersion(inst.Version)
		for i, v := range core.PostgresVersions {
			if v == normalized {
				w.selectedVersionIdx = i
				break
			}
		}
	}

	w.maxReached = StepReview
	w.step = StepReview
	w.blurAll()
	return w
}

func (w *wizardModel) setFocus(step wizardStep) {
	if step < StepEngine {
		step = StepEngine
	}
	if step > w.maxReached {
		step = w.maxReached
	}
	step = w.adjustStepForEngine(step)
	w.step = step
	w.blurAll()
	if step >= StepName && step <= StepMemoryLimit {
		w.focusInput(int(step) - int(StepName))
	}
	w.syncScrollToFocus()
}

func (w *wizardModel) moveFocus(delta int) {
	next := w.step + wizardStep(delta)
	if w.runtimeLocked() {
		if delta > 0 && next == StepRuntime {
			if w.isPostgres() {
				next = StepVersion
			} else {
				next = StepName
			}
		}
		if delta < 0 && next == StepRuntime {
			next = StepEngine
		}
	}
	if !w.isPostgres() {
		if delta > 0 && next == StepVersion {
			next = StepName
		}
		if delta < 0 && next == StepVersion {
			if w.runtimeLocked() {
				next = StepEngine
			} else {
				next = StepRuntime
			}
		}
	}
	w.setFocus(next)
}

func (w *wizardModel) cycleOption(delta int) {
	switch w.step {
	case StepEngine:
		n := w.selectedEngineIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(w.engines) {
			n = len(w.engines) - 1
		}
		w.selectedEngineIdx = n
	case StepRuntime:
		if w.runtimeLocked() {
			return
		}
		n := w.selectedRuntimeIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(w.runtimes) {
			n = len(w.runtimes) - 1
		}
		w.selectedRuntimeIdx = n
	case StepVersion:
		n := w.selectedVersionIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(core.PostgresVersions) {
			n = len(core.PostgresVersions) - 1
		}
		w.selectedVersionIdx = n
	}
}

func (w *wizardModel) confirmAdvance() bool {
	switch w.step {
	case StepEngine:
		w.maxReached = maxStep(w.maxReached, StepRuntime)
		if w.runtimeLocked() {
			if w.isPostgres() {
				w.maxReached = maxStep(w.maxReached, StepVersion)
				w.setFocus(StepVersion)
			} else {
				w.maxReached = maxStep(w.maxReached, StepName)
				w.setFocus(StepName)
			}
			return true
		}
		w.setFocus(StepRuntime)
		return true
	case StepRuntime:
		if w.isPostgres() {
			w.maxReached = maxStep(w.maxReached, StepVersion)
			w.setFocus(StepVersion)
		} else {
			w.maxReached = maxStep(w.maxReached, StepName)
			w.setFocus(StepName)
		}
		return true
	case StepVersion:
		w.maxReached = maxStep(w.maxReached, StepName)
		w.setFocus(StepName)
		return true
	case StepName:
		name := core.SanitizeIdent(w.inputs[0].Value())
		if name == "" {
			return false
		}
		if w.nameTaken(name) {
			return false
		}
		w.applyNameAutofill()
		w.maxReached = maxStep(w.maxReached, StepContainerName)
		w.setFocus(StepContainerName)
		return true
	case StepContainerName:
		w.maxReached = maxStep(w.maxReached, StepPort)
		w.setFocus(StepPort)
		return true
	case StepPort:
		w.maxReached = maxStep(w.maxReached, StepDatabase)
		w.setFocus(StepDatabase)
		return true
	case StepDatabase:
		w.maxReached = maxStep(w.maxReached, StepPassword)
		w.setFocus(StepPassword)
		return true
	case StepPassword:
		w.maxReached = maxStep(w.maxReached, StepMemoryLimit)
		w.setFocus(StepMemoryLimit)
		return true
	case StepMemoryLimit:
		w.maxReached = maxStep(w.maxReached, StepReview)
		w.setFocus(StepReview)
		return true
	default:
		return false
	}
}

func maxStep(a, b wizardStep) wizardStep {
	if a > b {
		return a
	}
	return b
}

func (w *wizardModel) applyNameAutofill() {
	if w.kind == wizardEdit {
		return
	}
	name := core.SanitizeIdent(w.inputs[0].Value())
	if name != w.inputs[0].Value() {
		w.inputs[0].SetValue(name)
	}
	engine := w.engines[w.selectedEngineIdx]

	prefix, _, defaultPort, defaultPass, defaultMem := engineDefaults(engine)

	if w.inputs[1].Value() == "" {
		w.inputs[1].SetValue(fmt.Sprintf("%s-%s", prefix, name))
	}
	if w.inputs[2].Value() == "" || w.inputs[2].Value() == "5432" {
		freePort := core.FindNextFreePort(mustAtoi(defaultPort), w.instances)
		w.inputs[2].SetValue(strconv.Itoa(freePort))
	}
	if w.inputs[3].Value() == "" {
		w.inputs[3].SetValue(fmt.Sprintf("%s_db", name))
	}
	if w.inputs[4].Value() == "" || w.inputs[4].Value() == "postgres" {
		w.inputs[4].SetValue(defaultPass)
	}
	if w.inputs[5].Value() == "" || w.inputs[5].Value() == "512M" {
		w.inputs[5].SetValue(defaultMem)
	}
}

func engineDefaults(engine string) (prefix, volPrefix, port, pass, mem string) {
	if engine == "sqlserver" {
		return "sql", "sqlserver", "1433", "SuperPassword123!", "2G"
	}
	return "pg", "pgdata", "5432", "postgres", "512M"
}

func (w *wizardModel) applyEngineDefaults(prevEngine, nextEngine string) {
	if prevEngine == nextEngine {
		return
	}
	name := core.SanitizeIdent(w.inputs[0].Value())
	prevP, _, prevPort, prevPass, prevMem := engineDefaults(prevEngine)
	nextP, _, nextPort, nextPass, nextMem := engineDefaults(nextEngine)

	if name != "" {
		oldCont := fmt.Sprintf("%s-%s", prevP, name)
		newCont := fmt.Sprintf("%s-%s", nextP, name)
		if w.inputs[1].Value() == oldCont {
			w.inputs[1].SetValue(newCont)
		}
	}
	if w.inputs[2].Value() == prevPort {
		free := core.FindNextFreePort(mustAtoi(nextPort), w.instances)
		w.inputs[2].SetValue(strconv.Itoa(free))
	}
	if w.inputs[4].Value() == prevPass {
		w.inputs[4].SetValue(nextPass)
	}
	if w.inputs[5].Value() == prevMem {
		w.inputs[5].SetValue(nextMem)
	}
	if nextEngine == "postgres" {
		if w.selectedVersionIdx < 0 || w.selectedVersionIdx >= len(core.PostgresVersions) {
			w.selectedVersionIdx = defaultPostgresVersionIdx()
		}
	}
	if w.step == StepVersion && !w.isPostgres() {
		w.setFocus(StepName)
	}
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (w *wizardModel) onTextStep() bool {
	return w.step >= StepName && w.step <= StepMemoryLimit
}

func (m *AppModel) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	w := &m.wizard

	switch msg := msg.(type) {
	case tea.KeyMsg:
		onText := w.onTextStep()
		switch msg.String() {
		case "esc", "ctrl+c":
			m.mode = ModeMain
			if w.kind == wizardEdit {
				m.statusMsg = "Instance edit cancelled"
			} else {
				m.statusMsg = "Instance creation cancelled"
			}
			m.statusIsErr = false
			return m, nil

		case "o":
			if w.kind == wizardEdit && !onText {
				if err := core.OpenInEditor(w.sourceEnvPath); err != nil {
					m.statusMsg = fmt.Sprintf("Failed to open editor: %v", err)
					m.statusIsErr = true
				} else {
					m.statusMsg = fmt.Sprintf("Opening %s in editor...", w.sourceEnvPath)
					m.statusIsErr = false
				}
				return m, nil
			}

		case "enter":
			if w.step == StepReview {
				name := core.SanitizeIdent(w.inputs[0].Value())
				if w.nameTaken(name) {
					m.statusMsg = fmt.Sprintf("Instance name '%s' is already taken", name)
					m.statusIsErr = true
					return m, nil
				}

				var oldInst *core.DatabaseInstance
				if w.kind == wizardEdit && w.wasRunning {
					oldInst = &core.DatabaseInstance{
						Name:          w.sourceName,
						Runtime:       w.sourceRuntime,
						ProjectName:   w.sourceProjectName,
						ContainerName: w.sourceContainerName,
						EnvFilePath:   w.sourceEnvPath,
						EngineType:    w.sourceEngine,
					}
				}

				if err := w.saveInstance(); err != nil {
					m.statusMsg = fmt.Sprintf("Error saving instance: %v", err)
					m.statusIsErr = true
					return m, nil
				}

				newVolume := w.derivedVolume()
				volumeChanged := w.kind == wizardEdit && w.sourceVolume != newVolume

				if w.kind == wizardEdit && w.wasRunning {
					m.clearConfirms()
					m.confirmRestartAfterEdit = true
					m.pendingRestartOld = oldInst
					m.pendingRestartNewName = name
					if oldInst != nil && oldInst.EnvFilePath != filepath.Join(w.instancesDir, name+".env") {
						m.pendingDeleteEnvPath = oldInst.EnvFilePath
					}
					m.mode = ModeMain
					if volumeChanged {
						m.statusMsg = fmt.Sprintf("Saved (volume → %s; old volume kept). Restart container with new config? Press 'y' to confirm, 'n' to cancel", newVolume)
					} else {
						m.statusMsg = "Saved. Restart container with new config? Press 'y' to confirm, 'n' to cancel"
					}
					m.statusIsErr = true
					return m, m.reloadInstancesCmd()
				}

				m.mode = ModeMain
				if w.kind == wizardEdit {
					if volumeChanged {
						m.statusMsg = fmt.Sprintf("Volume will change to %s. Previous volume %s is kept until you Purge it.", newVolume, w.sourceVolume)
					} else {
						m.statusMsg = fmt.Sprintf("Instance '%s' saved", name)
					}
				} else {
					m.statusMsg = fmt.Sprintf("Instance '%s' created successfully!", name)
				}
				m.statusIsErr = false
				return m, m.reloadInstancesCmd()
			}
			_ = w.confirmAdvance()
			return m, nil

		case "up":
			w.moveFocus(-1)
			return m, nil

		case "down":
			w.moveFocus(1)
			return m, nil

		case "k":
			if !onText {
				w.moveFocus(-1)
				return m, nil
			}

		case "j":
			if !onText {
				w.moveFocus(1)
				return m, nil
			}

		case "left":
			if !onText {
				prev := w.engines[w.selectedEngineIdx]
				w.cycleOption(-1)
				if w.step == StepEngine {
					w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
				}
				return m, nil
			}

		case "right":
			if !onText {
				prev := w.engines[w.selectedEngineIdx]
				w.cycleOption(1)
				if w.step == StepEngine {
					w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
				}
				return m, nil
			}

		case "h":
			if !onText {
				prev := w.engines[w.selectedEngineIdx]
				w.cycleOption(-1)
				if w.step == StepEngine {
					w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
				}
				return m, nil
			}

		case "l":
			if !onText {
				prev := w.engines[w.selectedEngineIdx]
				w.cycleOption(1)
				if w.step == StepEngine {
					w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
				}
				return m, nil
			}

		case "backspace":
			if onText {
				idx := int(w.step) - int(StepName)
				if w.inputs[idx].Value() == "" {
					if w.step > StepEngine {
						w.moveFocus(-1)
					}
					return m, nil
				}
			}
		}
	}

	if w.onTextStep() {
		idx := int(w.step) - int(StepName)
		var cmd tea.Cmd
		w.inputs[idx], cmd = w.inputs[idx].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (w *wizardModel) nameTaken(name string) bool {
	name = core.SanitizeIdent(name)
	for _, inst := range w.instances {
		if inst.Name == name {
			if w.kind == wizardEdit && name == w.sourceName {
				continue
			}
			return true
		}
	}
	return false
}

func (w *wizardModel) saveInstance() error {
	name := core.SanitizeIdent(w.inputs[0].Value())
	containerName := core.SanitizeIdent(w.inputs[1].Value())
	port := strings.TrimSpace(w.inputs[2].Value())
	db := core.SanitizeIdent(w.inputs[3].Value())
	pass := strings.TrimSpace(w.inputs[4].Value())
	memLimit := strings.TrimSpace(w.inputs[5].Value())
	if memLimit == "" {
		memLimit = "512M"
	}
	if name != "" {
		w.inputs[0].SetValue(name)
	}
	if containerName != "" {
		w.inputs[1].SetValue(containerName)
	}
	if db != "" {
		w.inputs[3].SetValue(db)
	}

	engine := w.engines[w.selectedEngineIdx]
	runtime := w.runtimes[w.selectedRuntimeIdx]
	volume := w.derivedVolume()

	var content string
	if engine == "postgres" {
		version := w.selectedVersion()
		content = fmt.Sprintf(`ENGINE=postgres
RUNTIME=%s

CONTAINER_NAME=%s
COMPOSE_PROJECT_NAME=%s
MEMORY_LIMIT=%s

POSTGRES_PORT=%s
POSTGRES_USER=postgres
POSTGRES_PASSWORD=%s
POSTGRES_DB=%s
POSTGRES_SCHEMA=public
POSTGRES_VERSION=%s
POSTGRES_VOLUME=%s
`, runtime, containerName, containerName, memLimit, port, pass, db, version, volume)
	} else {
		content = fmt.Sprintf(`ENGINE=sqlserver
RUNTIME=%s

CONTAINER_NAME=%s
COMPOSE_PROJECT_NAME=%s
MEMORY_LIMIT=%s

SQLSERVER_PORT=%s
SA_PASSWORD=%s
SQLSERVER_DB=%s
SQLSERVER_SCHEMA=dbo
SQLSERVER_VOLUME=%s
`, runtime, containerName, containerName, memLimit, port, pass, db, volume)
	}

	newPath := filepath.Join(w.instancesDir, fmt.Sprintf("%s.env", name))
	if err := os.WriteFile(newPath, []byte(content), 0644); err != nil {
		return err
	}
	if w.kind == wizardEdit && newPath != w.sourceEnvPath {
		if w.wasRunning {
			w.sourceEnvPath = newPath
			w.sourceName = name
		} else {
			if err := os.Remove(w.sourceEnvPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			w.sourceEnvPath = newPath
			w.sourceName = name
		}
	}
	return nil
}

func (m *AppModel) wizardPreviewRow(inner int, label, value string) string {
	return lipgloss.NewStyle().Width(inner).Background(BgSurface).Render(
		LabelStyle.Render(label) + " " + MutedStyle.Render(value),
	)
}

func (m *AppModel) wizardValueRow(inner int, label, value string, inputIdx int, extra string) string {
	w := &m.wizard
	parts := []string{LabelStyle.Render(label)}
	active := w.step == wizardStep(int(StepName)+inputIdx)
	if active && w.step != StepReview {
		parts = append(parts, wrapInputField(w.inputs[inputIdx].View()))
	} else {
		parts = append(parts, ValueStyle.Render(value))
	}
	if extra != "" {
		parts = append(parts, MutedStyle.Render(extra))
	}
	return surfaceLine(inner, joinWithSurfaceGaps(parts, 1))
}

func (w *wizardModel) bodyLineForStep(step wizardStep) int {
	// Body is only unlocked field rows (title/hints are fixed outside the viewport).
	focusStep := step
	if focusStep > StepMemoryLimit {
		focusStep = StepMemoryLimit
	}
	line := 0
	for s := StepEngine; s <= focusStep; s++ {
		if s == StepVersion && !w.isPostgres() {
			continue
		}
		if s <= w.maxReached {
			line++
		}
	}
	if focusStep >= StepPassword && w.maxReached >= StepDatabase {
		line++
	}
	if line == 0 {
		return 0
	}
	return line - 1
}

func (w *wizardModel) syncScrollToFocus() {
	if w.step == StepReview {
		w.scrollViewport.GotoBottom()
		return
	}
	target := w.bodyLineForStep(w.step)
	if target < w.scrollViewport.YOffset {
		w.scrollViewport.YOffset = target
	}
	if w.scrollViewport.Height > 0 && target >= w.scrollViewport.YOffset+w.scrollViewport.Height {
		w.scrollViewport.YOffset = target - w.scrollViewport.Height + 1
	}
	if w.scrollViewport.YOffset < 0 {
		w.scrollViewport.YOffset = 0
	}
}

func (w *wizardModel) wizardHintsLine(inner int) string {
	if w.step == StepReview {
		if w.kind == wizardEdit {
			return surfaceLine(inner, RunningStyle.Render("All set! Press [Enter] to save, [↑/b] to edit, [o] external editor, or [Esc] to cancel."))
		}
		return surfaceLine(inner, RunningStyle.Render("All set! Press [Enter] to create the instance, [↑/b] to edit, or [Esc] to cancel."))
	}
	if w.kind == wizardEdit {
		return surfaceLine(inner, MutedStyle.Render("[↑↓] rows  [←→] options  [Enter] next  [o] editor  [Esc] cancel  (Runtime locked)"))
	}
	return surfaceLine(inner, MutedStyle.Render("[↑↓] rows  [←→] options  [Enter] next  [Esc] cancel"))
}

func (m *AppModel) viewWizardDock(innerWidth, dockHeight int) string {
	w := &m.wizard
	inputWidth := innerWidth - 14 - 1
	if inputWidth < 8 {
		inputWidth = 8
	}
	for i := range w.inputs {
		w.inputs[i].Width = inputWidth
	}

	titleText := "New Database Instance"
	if w.kind == wizardEdit {
		titleText = "Edit Instance"
	}
	title := surfaceLine(innerWidth, TitleStyle.Render(titleText))
	hints := w.wizardHintsLine(innerWidth)
	// Title + hints are fixed; body scrolls in the remaining rows.
	scrollHeight := dockHeight - 2
	if scrollHeight < 1 {
		scrollHeight = 1
	}

	body := m.buildWizardBodyRows(innerWidth, inputWidth)
	w.scrollViewport.Width = innerWidth
	w.scrollViewport.Height = scrollHeight
	w.scrollViewport.SetContent(body)
	w.syncScrollToFocus()

	return lipgloss.JoinVertical(lipgloss.Left, title, w.scrollViewport.View(), hints)
}

func (m *AppModel) buildWizardBodyRows(inner, inputWidth int) string {
	w := &m.wizard

	row := func(parts ...string) string {
		return surfaceLine(inner, joinWithSurfaceGaps(parts, 1))
	}

	var content []string

	if w.maxReached >= StepEngine {
		if w.step == StepEngine {
			parts := []string{LabelStyle.Render("1. Engine:")}
			for i, eng := range w.engines {
				label := engineDisplay(eng)
				if i == w.selectedEngineIdx {
					parts = append(parts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", label)))
				} else {
					parts = append(parts, NormalItemStyle.Render(fmt.Sprintf(" %s ", label)))
				}
			}
			content = append(content, row(parts...))
		} else {
			content = append(content, row(LabelStyle.Render("1. Engine:"), ValueHighlightStyle.Render(engineDisplay(w.engines[w.selectedEngineIdx]))))
		}
	}

	if w.maxReached >= StepRuntime {
		runtimeLabel := runtimeDisplay(w.runtimes[w.selectedRuntimeIdx])
		if w.runtimeLocked() {
			content = append(content, m.wizardPreviewRow(inner, "2. Runtime:", runtimeLabel+" (locked)"))
		} else if w.step == StepRuntime {
			parts := []string{LabelStyle.Render("2. Runtime:")}
			for i, r := range w.runtimes {
				label := runtimeDisplay(r)
				if i == w.selectedRuntimeIdx {
					parts = append(parts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", label)))
				} else {
					parts = append(parts, NormalItemStyle.Render(fmt.Sprintf(" %s ", label)))
				}
			}
			content = append(content, row(parts...))
		} else {
			content = append(content, row(LabelStyle.Render("2. Runtime:"), ValueHighlightStyle.Render(runtimeLabel)))
		}
	}

	if w.isPostgres() && w.maxReached >= StepVersion {
		if w.step == StepVersion {
			parts := []string{LabelStyle.Render("3. Version:")}
			for i, ver := range core.PostgresVersions {
				if i == w.selectedVersionIdx {
					parts = append(parts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", ver)))
				} else {
					parts = append(parts, NormalItemStyle.Render(fmt.Sprintf(" %s ", ver)))
				}
			}
			content = append(content, row(parts...))
		} else {
			content = append(content, row(LabelStyle.Render("3. Version:"), ValueHighlightStyle.Render(w.selectedVersion())))
		}
	}

	nameLabel := "3. Name:"
	containerLabel := "4. Container:"
	portLabel := "5. Port:"
	dbLabel := "6. Database:"
	passLabel := "7. Password:"
	memLabel := "8. Memory:"
	if w.isPostgres() && w.maxReached >= StepVersion {
		nameLabel = "4. Name:"
		containerLabel = "5. Container:"
		portLabel = "6. Port:"
		dbLabel = "7. Database:"
		passLabel = "8. Password:"
		memLabel = "9. Memory:"
	}

	if w.maxReached >= StepName {
		content = append(content, m.wizardValueRow(inner, nameLabel, truncateEnd(w.inputs[0].Value(), inputWidth), 0, ""))
	}
	if w.maxReached >= StepContainerName {
		content = append(content, m.wizardValueRow(inner, containerLabel, truncateEnd(w.inputs[1].Value(), inputWidth), 1, ""))
	}
	if w.maxReached >= StepPort {
		content = append(content, m.wizardValueRow(inner, portLabel, truncateEnd(w.inputs[2].Value(), inputWidth), 2, ""))
	}
	if w.maxReached >= StepDatabase {
		content = append(content, m.wizardValueRow(inner, dbLabel, truncateEnd(w.inputs[3].Value(), inputWidth), 3, ""))
	}
	if w.maxReached >= StepDatabase {
		content = append(content, m.wizardPreviewRow(inner, "Volume:", truncateEnd(w.derivedVolume(), inputWidth)))
	}
	if w.maxReached >= StepPassword {
		content = append(content, m.wizardValueRow(inner, passLabel, truncateEnd(w.inputs[4].Value(), inputWidth), 4, ""))
	}
	if w.maxReached >= StepMemoryLimit {
		recommendation := "(Recommended: 512M - 1G)"
		if w.engines[w.selectedEngineIdx] == "sqlserver" {
			recommendation = "(Recommended: 2G min for MSSQL)"
		}
		content = append(content, m.wizardValueRow(inner, memLabel, truncateEnd(w.inputs[5].Value(), inputWidth), 5, recommendation))
	}

	if len(content) == 0 {
		return surfaceBlankLine(inner)
	}
	return lipgloss.JoinVertical(lipgloss.Left, content...)
}

func (m *AppModel) viewWizard() string {
	inner := screenInnerWidth(m.width)
	_, rightWidth, _ := splitPanelWidths(inner)
	contentHeight := mainContentHeight(m.height)
	_, dockHeight := splitPanelHalfHeight(contentHeight - 1)
	rightInner := panelInnerWidth(rightWidth)
	dock := m.viewWizardDock(rightInner, dockHeight)
	return ActivePanelStyle.
		Width(rightWidth).
		Height(dockHeight + 2).
		Render(dock)
}
