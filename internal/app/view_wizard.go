package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"local-database-manager/internal/core"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wizardStep int

const (
	StepEngine wizardStep = iota
	StepRuntime
	StepName
	StepContainerName
	StepPort
	StepDatabase
	StepVolume
	StepPassword
	StepMemoryLimit
	StepReview
)

type wizardModel struct {
	projectRoot  string
	instancesDir string
	instances    []*core.DatabaseInstance

	step wizardStep

	maxReached wizardStep

	selectedEngineIdx  int
	selectedRuntimeIdx int

	engines  []string
	runtimes []string

	inputs []textinput.Model
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

	inputs := make([]textinput.Model, 7)

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
	inputs[4].Placeholder = "pgdata_my_new_instance"

	inputs[5] = styleTextInput(textinput.New())
	inputs[5].SetValue("postgres")

	inputs[6] = styleTextInput(textinput.New())
	inputs[6].SetValue("512M")

	for i := range inputs {
		inputs[i].Prompt = ""
		inputs[i].Width = 32
	}

	w := wizardModel{
		projectRoot:        projectRoot,
		instancesDir:       instancesDir,
		instances:          existing,
		step:               StepEngine,
		maxReached:         StepEngine,
		selectedEngineIdx:  0,
		selectedRuntimeIdx: 0,
		engines:            engines,
		runtimes:           runtimes,
		inputs:             inputs,
	}
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
	w.step = step
	w.blurAll()
	if step >= StepName && step <= StepMemoryLimit {
		w.focusInput(int(step) - int(StepName))
	}
}

func (w *wizardModel) moveFocus(delta int) {
	w.setFocus(w.step + wizardStep(delta))
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
		n := w.selectedRuntimeIdx + delta
		if n < 0 {
			n = 0
		}
		if n >= len(w.runtimes) {
			n = len(w.runtimes) - 1
		}
		w.selectedRuntimeIdx = n
	}
}

func (w *wizardModel) confirmAdvance() bool {
	switch w.step {
	case StepEngine:
		w.maxReached = maxStep(w.maxReached, StepRuntime)
		w.setFocus(StepRuntime)
		return true
	case StepRuntime:
		w.maxReached = maxStep(w.maxReached, StepName)
		w.setFocus(StepName)
		return true
	case StepName:
		if strings.TrimSpace(w.inputs[0].Value()) == "" {
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
		w.maxReached = maxStep(w.maxReached, StepVolume)
		w.setFocus(StepVolume)
		return true
	case StepVolume:
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
	name := strings.TrimSpace(w.inputs[0].Value())
	engine := w.engines[w.selectedEngineIdx]

	prefix, volPrefix, defaultPort, defaultPass, defaultMem := engineDefaults(engine)

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
	if w.inputs[4].Value() == "" {
		w.inputs[4].SetValue(fmt.Sprintf("%s_%s", volPrefix, name))
	}
	if w.inputs[5].Value() == "" || w.inputs[5].Value() == "postgres" {
		w.inputs[5].SetValue(defaultPass)
	}
	if w.inputs[6].Value() == "" || w.inputs[6].Value() == "512M" {
		w.inputs[6].SetValue(defaultMem)
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
	name := strings.TrimSpace(w.inputs[0].Value())
	prevP, prevV, prevPort, prevPass, prevMem := engineDefaults(prevEngine)
	nextP, nextV, nextPort, nextPass, nextMem := engineDefaults(nextEngine)

	if name != "" {
		oldCont := fmt.Sprintf("%s-%s", prevP, name)
		newCont := fmt.Sprintf("%s-%s", nextP, name)
		if w.inputs[1].Value() == oldCont {
			w.inputs[1].SetValue(newCont)
		}
		oldVol := fmt.Sprintf("%s_%s", prevV, name)
		newVol := fmt.Sprintf("%s_%s", nextV, name)
		if w.inputs[4].Value() == oldVol {
			w.inputs[4].SetValue(newVol)
		}
	}
	if w.inputs[2].Value() == prevPort {
		free := core.FindNextFreePort(mustAtoi(nextPort), w.instances)
		w.inputs[2].SetValue(strconv.Itoa(free))
	}
	if w.inputs[5].Value() == prevPass {
		w.inputs[5].SetValue(nextPass)
	}
	if w.inputs[6].Value() == prevMem {
		w.inputs[6].SetValue(nextMem)
	}
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (m *AppModel) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	w := &m.wizard

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.mode = ModeMain
			m.statusMsg = "Instance creation cancelled"
			m.statusIsErr = false
			return m, nil

		case "enter":
			if w.step == StepReview {
				if err := w.saveInstance(); err != nil {
					m.statusMsg = fmt.Sprintf("Error saving instance: %v", err)
					m.statusIsErr = true
					m.mode = ModeMain
					return m, nil
				}

				m.mode = ModeMain
				m.statusMsg = fmt.Sprintf("Instance '%s' created successfully!", w.inputs[0].Value())
				m.statusIsErr = false
				return m, m.reloadInstancesCmd()
			}
			_ = w.confirmAdvance()
			return m, nil

		case "up", "k":
			w.moveFocus(-1)
			return m, nil

		case "down", "j":
			w.moveFocus(1)
			return m, nil

		case "left", "h":
			prev := w.engines[w.selectedEngineIdx]
			w.cycleOption(-1)
			if w.step == StepEngine {
				w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
			}
			return m, nil

		case "right", "l":
			prev := w.engines[w.selectedEngineIdx]
			w.cycleOption(1)
			if w.step == StepEngine {
				w.applyEngineDefaults(prev, w.engines[w.selectedEngineIdx])
			}
			return m, nil

		case "b":
			if w.step > StepEngine {
				w.moveFocus(-1)
			}
			return m, nil

		case "backspace":
			if w.step >= StepName && w.step <= StepMemoryLimit {
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

	if w.step >= StepName && w.step <= StepMemoryLimit {
		idx := int(w.step) - int(StepName)
		var cmd tea.Cmd
		w.inputs[idx], cmd = w.inputs[idx].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (w *wizardModel) saveInstance() error {
	name := strings.TrimSpace(w.inputs[0].Value())
	containerName := strings.TrimSpace(w.inputs[1].Value())
	port := strings.TrimSpace(w.inputs[2].Value())
	db := strings.TrimSpace(w.inputs[3].Value())
	volume := strings.TrimSpace(w.inputs[4].Value())
	pass := strings.TrimSpace(w.inputs[5].Value())
	memLimit := strings.TrimSpace(w.inputs[6].Value())
	if memLimit == "" {
		memLimit = "512M"
	}

	engine := w.engines[w.selectedEngineIdx]
	runtime := w.runtimes[w.selectedRuntimeIdx]

	var content string
	if engine == "postgres" {
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
POSTGRES_VOLUME=%s
`, runtime, containerName, containerName, memLimit, port, pass, db, volume)
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

	filePath := filepath.Join(w.instancesDir, fmt.Sprintf("%s.env", name))
	return os.WriteFile(filePath, []byte(content), 0644)
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

func (m *AppModel) viewWizard() string {
	w := &m.wizard
	boxWidth := m.width - 12
	if boxWidth < 50 {
		boxWidth = 50
	}
	if boxWidth > 72 {
		boxWidth = 72
	}

	inner := panelInnerWidth(boxWidth)
	inputWidth := inner - 14 - 1
	if inputWidth < 8 {
		inputWidth = 8
	}
	for i := range w.inputs {
		w.inputs[i].Width = inputWidth
	}

	row := func(parts ...string) string {
		return surfaceLine(inner, joinWithSurfaceGaps(parts, 1))
	}

	content := []string{
		surfaceLine(inner, TitleStyle.Render("New Database Instance")),
		panelSeparator(inner),
		surfaceBlankLine(inner),
	}

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
		if w.step == StepRuntime {
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
			content = append(content, row(LabelStyle.Render("2. Runtime:"), ValueHighlightStyle.Render(runtimeDisplay(w.runtimes[w.selectedRuntimeIdx]))))
		}
	}

	if w.maxReached >= StepName {
		content = append(content, m.wizardValueRow(inner, "3. Name:", truncateEnd(w.inputs[0].Value(), inputWidth), 0, ""))
	}
	if w.maxReached >= StepContainerName {
		content = append(content, m.wizardValueRow(inner, "4. Container:", truncateEnd(w.inputs[1].Value(), inputWidth), 1, ""))
	}
	if w.maxReached >= StepPort {
		content = append(content, m.wizardValueRow(inner, "5. Port:", truncateEnd(w.inputs[2].Value(), inputWidth), 2, ""))
	}
	if w.maxReached >= StepDatabase {
		content = append(content, m.wizardValueRow(inner, "6. Database:", truncateEnd(w.inputs[3].Value(), inputWidth), 3, ""))
	}
	if w.maxReached >= StepVolume {
		content = append(content, m.wizardValueRow(inner, "7. Volume:", truncateEnd(w.inputs[4].Value(), inputWidth), 4, ""))
	}
	if w.maxReached >= StepPassword {
		content = append(content, m.wizardValueRow(inner, "8. Password:", truncateEnd(w.inputs[5].Value(), inputWidth), 5, ""))
	}
	if w.maxReached >= StepMemoryLimit {
		recommendation := "(Recommended: 512M - 1G)"
		if w.engines[w.selectedEngineIdx] == "sqlserver" {
			recommendation = "(Recommended: 2G min for MSSQL)"
		}
		content = append(content, m.wizardValueRow(inner, "9. Memory:", truncateEnd(w.inputs[6].Value(), inputWidth), 6, recommendation))
	}

	content = append(content, surfaceBlankLine(inner))
	if w.step == StepReview {
		content = append(content, surfaceLine(inner, RunningStyle.Render("All set! Press [Enter] to create the instance, [↑/b] to edit, or [Esc] to cancel.")))
	} else {
		content = append(content, surfaceLine(inner, MutedStyle.Render("[↑↓] rows  [←→] options  [Enter] next  [b] back  [Esc] cancel")))
	}

	return ActivePanelStyle.
		Width(boxWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
