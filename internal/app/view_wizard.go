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

func newWizardModel(projectRoot, instancesDir string, existing []*core.DatabaseInstance) wizardModel {
	engines := []string{"postgres", "sqlserver"}
	runtimes := []string{"docker", "podman"}

	inputs := make([]textinput.Model, 7)

	inputs[0] = styleTextInput(textinput.New())
	inputs[0].Placeholder = "my_new_instance"
	inputs[0].Focus()

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

	return wizardModel{
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
				return m, nil

			case StepRuntime:
				w.step = StepName
				w.inputs[0].Focus()
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
				w.inputs[1].Focus()
				return m, nil

			case StepContainerName:
				w.step = StepPort
				w.inputs[2].Focus()
				return m, nil

			case StepPort:
				w.step = StepDatabase
				w.inputs[3].Focus()
				return m, nil

			case StepDatabase:
				w.step = StepVolume
				w.inputs[4].Focus()
				return m, nil

			case StepVolume:
				w.step = StepPassword
				w.inputs[5].Focus()
				return m, nil

			case StepPassword:
				w.step = StepMemoryLimit
				w.inputs[6].Focus()
				return m, nil

			case StepMemoryLimit:
				w.step = StepReview
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

func (m *AppModel) viewWizard() string {
	w := &m.wizard
	boxWidth := m.width - 12
	if boxWidth < 50 {
		boxWidth = 50
	}
	if boxWidth > 72 {
		boxWidth = 72
	}

	var content []string
	content = append(content, TitleStyle.Render("New Database Instance"))
	content = append(content, SeparatorStyle.Render(strings.Repeat("─", boxWidth-4)))
	content = append(content, surfaceGap(1))

	engineParts := []string{LabelStyle.Render("1. Engine:")}
	if w.step == StepEngine {
		for i, eng := range w.engines {
			label := eng
			if eng == "postgres" {
				label = "Postgres"
			} else if eng == "sqlserver" {
				label = "SQL Server"
			}
			if i == w.selectedEngineIdx {
				engineParts = append(engineParts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", label)))
			} else {
				engineParts = append(engineParts, NormalItemStyle.Render(fmt.Sprintf(" %s ", label)))
			}
		}
	} else {
		engineParts = append(engineParts, ValueHighlightStyle.Render(w.engines[w.selectedEngineIdx]))
	}
	content = append(content, joinWithSurfaceGaps(engineParts, 1))

	if w.step >= StepRuntime {
		runtimeParts := []string{LabelStyle.Render("2. Runtime:")}
		if w.step == StepRuntime {
			for i, r := range w.runtimes {
				if i == w.selectedRuntimeIdx {
					runtimeParts = append(runtimeParts, SelectedItemStyle.Render(fmt.Sprintf(" [%s] ", r)))
				} else {
					runtimeParts = append(runtimeParts, NormalItemStyle.Render(fmt.Sprintf(" %s ", r)))
				}
			}
		} else {
			runtimeParts = append(runtimeParts, ValueHighlightStyle.Render(w.runtimes[w.selectedRuntimeIdx]))
		}
		content = append(content, joinWithSurfaceGaps(runtimeParts, 1))
	}

	if w.step >= StepName {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("3. Name:"), surfaceGap(1), wrapInputField(w.inputs[0].View())))
	}
	if w.step >= StepContainerName {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("4. Container:"), surfaceGap(1), wrapInputField(w.inputs[1].View())))
	}
	if w.step >= StepPort {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("5. Port:"), surfaceGap(1), wrapInputField(w.inputs[2].View())))
	}
	if w.step >= StepDatabase {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("6. Database:"), surfaceGap(1), wrapInputField(w.inputs[3].View())))
	}
	if w.step >= StepVolume {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("7. Volume:"), surfaceGap(1), wrapInputField(w.inputs[4].View())))
	}
	if w.step >= StepPassword {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("8. Password:"), surfaceGap(1), wrapInputField(w.inputs[5].View())))
	}
	if w.step >= StepMemoryLimit {
		engine := w.engines[w.selectedEngineIdx]
		recommendation := "(Recommended: 512M - 1G)"
		if engine == "sqlserver" {
			recommendation = "(Recommended: 2G min for MSSQL)"
		}
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, LabelStyle.Render("9. Memory:"), surfaceGap(1), wrapInputField(w.inputs[6].View()), surfaceGap(1), MutedStyle.Render(recommendation)))
	}

	if w.step == StepReview {
		content = append(content, surfaceGap(1))
		content = append(content, RunningStyle.Render("All set! Press [Enter] to create the instance or [Esc] to cancel."))
	} else {
		content = append(content, surfaceGap(1))
		content = append(content, MutedStyle.Render("Press [Enter] to advance, [↑/↓] for options, [Esc] to cancel."))
	}

	return ActivePanelStyle.
		Width(boxWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
