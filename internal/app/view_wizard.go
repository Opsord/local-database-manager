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
		selectedEngineIdx:  0,
		selectedRuntimeIdx: 0,
		engines:            engines,
		runtimes:           runtimes,
		inputs:             inputs,
	}
	w.blurAll()
	return w
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
			switch w.step {
			case StepEngine:
				w.step = StepRuntime
				w.blurAll()
				return m, nil

			case StepRuntime:
				w.step = StepName
				w.focusInput(0)
				return m, nil

			case StepName:
				name := strings.TrimSpace(w.inputs[0].Value())
				if name == "" {
					return m, nil
				}
				engine := w.engines[w.selectedEngineIdx]

				prefix := "pg"
				defaultPort := 5432
				defaultPass := "postgres"
				defaultMem := "512M"
				volPrefix := "pgdata"

				if engine == "sqlserver" {
					prefix = "sql"
					defaultPort = 1433
					defaultPass = "SuperPassword123!"
					defaultMem = "2G"
					volPrefix = "sqlserver"
				}

				if w.inputs[1].Value() == "" {
					w.inputs[1].SetValue(fmt.Sprintf("%s-%s", prefix, name))
				}
				if w.inputs[2].Value() == "" || w.inputs[2].Value() == "5432" {
					freePort := core.FindNextFreePort(defaultPort, w.instances)
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

				w.step = StepContainerName
				w.focusInput(1)
				return m, nil

			case StepContainerName:
				w.step = StepPort
				w.focusInput(2)
				return m, nil

			case StepPort:
				w.step = StepDatabase
				w.focusInput(3)
				return m, nil

			case StepDatabase:
				w.step = StepVolume
				w.focusInput(4)
				return m, nil

			case StepVolume:
				w.step = StepPassword
				w.focusInput(5)
				return m, nil

			case StepPassword:
				w.step = StepMemoryLimit
				w.focusInput(6)
				return m, nil

			case StepMemoryLimit:
				w.step = StepReview
				w.blurAll()
				return m, nil

			case StepReview:
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

		case "up", "k":
			if w.step == StepEngine && w.selectedEngineIdx > 0 {
				w.selectedEngineIdx--
			} else if w.step == StepRuntime && w.selectedRuntimeIdx > 0 {
				w.selectedRuntimeIdx--
			}
			return m, nil

		case "down", "j":
			if w.step == StepEngine && w.selectedEngineIdx < len(w.engines)-1 {
				w.selectedEngineIdx++
			} else if w.step == StepRuntime && w.selectedRuntimeIdx < len(w.runtimes)-1 {
				w.selectedRuntimeIdx++
			}
			return m, nil
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

	if w.step >= StepRuntime {
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

	if w.step >= StepName {
		content = append(content, m.wizardValueRow(inner, "3. Name:", truncateEnd(w.inputs[0].Value(), inputWidth), 0, ""))
	}
	if w.step >= StepContainerName {
		content = append(content, m.wizardValueRow(inner, "4. Container:", truncateEnd(w.inputs[1].Value(), inputWidth), 1, ""))
	}
	if w.step >= StepPort {
		content = append(content, m.wizardValueRow(inner, "5. Port:", truncateEnd(w.inputs[2].Value(), inputWidth), 2, ""))
	}
	if w.step >= StepDatabase {
		content = append(content, m.wizardValueRow(inner, "6. Database:", truncateEnd(w.inputs[3].Value(), inputWidth), 3, ""))
	}
	if w.step >= StepVolume {
		content = append(content, m.wizardValueRow(inner, "7. Volume:", truncateEnd(w.inputs[4].Value(), inputWidth), 4, ""))
	}
	if w.step >= StepPassword {
		content = append(content, m.wizardValueRow(inner, "8. Password:", truncateEnd(w.inputs[5].Value(), inputWidth), 5, ""))
	}
	if w.step >= StepMemoryLimit {
		recommendation := "(Recommended: 512M - 1G)"
		if w.engines[w.selectedEngineIdx] == "sqlserver" {
			recommendation = "(Recommended: 2G min for MSSQL)"
		}
		content = append(content, m.wizardValueRow(inner, "9. Memory:", truncateEnd(w.inputs[6].Value(), inputWidth), 6, recommendation))
	}

	content = append(content, surfaceBlankLine(inner))
	if w.step == StepReview {
		content = append(content, surfaceLine(inner, RunningStyle.Render("All set! Press [Enter] to create the instance or [Esc] to cancel.")))
	} else {
		content = append(content, surfaceLine(inner, MutedStyle.Render("Press [Enter] to advance, [↑/↓] for options, [Esc] to cancel.")))
	}

	return ActivePanelStyle.
		Width(boxWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
