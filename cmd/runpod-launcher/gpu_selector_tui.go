package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// GPUSelectorModel is a bubble tea model for selecting a GPU
type GPUSelectorModel struct {
	gpus          []pod.GPUType
	cursor        int
	selected      string
	width         int
	height        int
	filterInput   string
	filteredGPUs  []pod.GPUType
	filterMode    bool
	quitting      bool
}

// NewGPUSelectorModel creates a new GPU selector model
func NewGPUSelectorModel(gpus []pod.GPUType) *GPUSelectorModel {
	m := &GPUSelectorModel{
		gpus:         gpus,
		filteredGPUs: gpus,
		cursor:       0,
	}
	return m
}

func (m *GPUSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *GPUSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if len(m.filteredGPUs) > 0 {
				m.selected = m.filteredGPUs[m.cursor].ID
			}
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.filterMode = !m.filterMode
			m.filterInput = ""
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
			if m.cursor < len(m.filteredGPUs)-1 {
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

func (m *GPUSelectorModel) applyFilter() {
	if m.filterInput == "" {
		m.filteredGPUs = m.gpus
		return
	}

	var filtered []pod.GPUType
	for _, gpu := range m.gpus {
		if matchesFilter(gpu, m.filterInput) {
			filtered = append(filtered, gpu)
		}
	}
	m.filteredGPUs = filtered
}

func matchesFilter(gpu pod.GPUType, filter string) bool {
	// Case-insensitive substring matching
	filterLower := strings.ToLower(filter)
	idLower := strings.ToLower(gpu.ID)
	nameLower := strings.ToLower(gpu.DisplayName)

	return strings.Contains(idLower, filterLower) || strings.Contains(nameLower, filterLower)
}

func (m *GPUSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	var s string

	// Header
	s += lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Bold(true).
		Render("┌─ Select GPU (Secure Cloud)") + "\n"

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

	// GPU list
	if len(m.filteredGPUs) == 0 {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("└─ No GPUs match your filter")
		return s
	}

	maxLines := m.height - 10
	if maxLines < 1 {
		maxLines = 10
	}

	for i, gpu := range m.filteredGPUs {
		if i >= maxLines {
			s += lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(fmt.Sprintf("└─ ... and %d more", len(m.filteredGPUs)-i)) + "\n"
			break
		}

		prefix := "│ "
		if i == len(m.filteredGPUs)-1 && i < maxLines-1 {
			prefix = "└─"
		}

		selected := "  "
		if i == m.cursor {
			selected = "→ "
		}

		// Show availability indicator based on max GPU count
		availability := "?"
		if gpu.MaxGpuCountSecureCloud > 10 {
			availability = "✓"
		} else if gpu.MaxGpuCountSecureCloud > 0 {
			availability = "◐"
		} else {
			availability = "✗"
		}
		line := fmt.Sprintf("%s%-40s %3dGB  [%s]  $%.4f/hr",
			selected, gpu.DisplayName, gpu.MemoryInGb, availability, gpu.SecurePrice)

		if i == m.cursor {
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

// selectGPUTypeTUI displays an interactive GPU selector using bubble tea
func selectGPUTypeTUI(gpus []pod.GPUType) (string, error) {
	m := NewGPUSelectorModel(gpus)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	selector := finalModel.(*GPUSelectorModel)
	if selector.selected == "" && len(gpus) > 0 {
		// Default to first if nothing selected (e.g., quit without selecting)
		return gpus[0].ID, nil
	}
	return selector.selected, nil
}
