package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/romanvolkov/runpod-launcher/internal/models"
)

// ModelSelectorModel is a bubble tea model for selecting a model
type ModelSelectorModel struct {
	models        []string
	modelSpecs    map[string]models.ModelSpec
	cursor        int
	selected      string
	width         int
	height        int
	filterInput   string
	filteredIdx   []int
	filterMode    bool
	quitting      bool
}

// NewModelSelectorModel creates a new model selector model
func NewModelSelectorModel(modelNames []string, specs map[string]models.ModelSpec) *ModelSelectorModel {
	m := &ModelSelectorModel{
		models:     modelNames,
		modelSpecs: specs,
		cursor:     0,
	}
	m.filteredIdx = make([]int, len(modelNames))
	for i := range modelNames {
		m.filteredIdx[i] = i
	}
	return m
}

func (m *ModelSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *ModelSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if len(m.filteredIdx) > 0 {
				m.selected = m.models[m.filteredIdx[m.cursor]]
			}
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.filterMode = !m.filterMode
			m.filterInput = ""
			m.cursor = 0
			m.applyFilter()
			return m, nil

		case "backspace":
			if m.filterMode && len(m.filterInput) > 0 {
				m.filterInput = m.filterInput[:len(m.filterInput)-1]
				m.cursor = 0
				m.applyFilter()
			}
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.filteredIdx)-1 {
				m.cursor++
			}
			return m, nil

		default:
			if m.filterMode && len(msg.Runes) > 0 {
				for _, r := range msg.Runes {
					m.filterInput += string(r)
				}
				m.cursor = 0
				m.applyFilter()
			}
		}
	}

	return m, nil
}

func (m *ModelSelectorModel) applyFilter() {
	if m.filterInput == "" {
		m.filteredIdx = make([]int, len(m.models))
		for i := range m.models {
			m.filteredIdx[i] = i
		}
		return
	}

	var filtered []int
	filterLower := strings.ToLower(m.filterInput)
	for i, name := range m.models {
		if strings.Contains(strings.ToLower(name), filterLower) {
			filtered = append(filtered, i)
		}
	}
	m.filteredIdx = filtered
}

func (m *ModelSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	var s string

	// Header
	s += lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Bold(true).
		Render("┌─ Select Model") + "\n"

	s += lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("│  ↑/↓ or k/j: navigate | Enter: select | /: filter | q: quit") + "\n"

	// Filter input
	if m.filterMode {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("│  ") +
			lipgloss.NewStyle().
				Background(lipgloss.Color("33")).
				Foreground(lipgloss.Color("0")).
				Render(" Search: " + m.filterInput + " ") + "\n"
	} else if m.filterInput != "" {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf("│  Filter: %s (press / to edit)", m.filterInput)) + "\n"
	} else {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("│") + "\n"
	}
	s += "\n"

	// Model list
	if len(m.filteredIdx) == 0 {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("└─ No models match your filter")
		return s
	}

	maxLines := m.height - 10
	if maxLines < 1 {
		maxLines = 10
	}

	for displayIdx, modelIdx := range m.filteredIdx {
		if displayIdx >= maxLines {
			s += lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(fmt.Sprintf("└─ ... and %d more", len(m.filteredIdx)-displayIdx)) + "\n"
			break
		}

		prefix := "│ "
		if displayIdx == len(m.filteredIdx)-1 && displayIdx < maxLines-1 {
			prefix = "└─"
		}

		selected := "  "
		if displayIdx == m.cursor {
			selected = "→ "
		}

		modelName := m.models[modelIdx]
		spec := m.modelSpecs[modelName]
		contextStr := fmt.Sprintf("%dK", spec.ContextWindow/1000)
		if spec.ContextWindow < 1000 {
			contextStr = fmt.Sprintf("%dK", spec.ContextWindow)
		}

		line := fmt.Sprintf("%s%-20s %3dGB VRAM  %4s context",
			selected, modelName, spec.MinVramGb, contextStr)

		if displayIdx == m.cursor {
			s += prefix + lipgloss.NewStyle().
				Background(lipgloss.Color("33")).
				Foreground(lipgloss.Color("0")).
				Render(line) + "\n"
		} else {
			s += prefix + line + "\n"
		}
	}

	return s
}

// selectModelTUI displays an interactive model selector using bubble tea
func selectModelTUI(modelNames []string, specs map[string]models.ModelSpec) (string, error) {
	m := NewModelSelectorModel(modelNames, specs)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	selector := finalModel.(*ModelSelectorModel)
	// Return selected model, or default to first model if nothing selected
	if selector.selected != "" {
		return selector.selected, nil
	}
	if len(modelNames) > 0 {
		return modelNames[0], nil
	}
	return "", fmt.Errorf("no models available to select")
}
