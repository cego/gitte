package internal

import (
	"fmt"
	"gitte/config"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectItem represents a project with its state
type ProjectItem struct {
	Name         string
	DefaultState bool // true = enabled by default, false = disabled by default
	CurrentState bool // current enabled/disabled state
	IsCustom     bool // whether this differs from default
}

type toggleModel struct {
	projects        []ProjectItem
	cursor          int
	viewportOffset  int // For scrolling large lists
	cwd             string
	toggledProjects config.ToggledProjects
	cfg             *config.GitteConfig
	width           int
	height          int
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170"))

	enabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	disabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	customStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

func newToggleModel(cfg *config.GitteConfig, cwd string) (*toggleModel, error) {
	toggledProjects, err := ReadToggledProjects(cwd)
	if err != nil {
		return nil, err
	}

	projects := make([]ProjectItem, 0, len(cfg.Projects))
	for name, proj := range cfg.Projects {
		defaultState := !proj.DefaultDisabled
		currentState := defaultState
		isCustom := false

		if toggled, exists := toggledProjects[name]; exists {
			currentState = toggled
			isCustom = (currentState != defaultState)
		}

		projects = append(projects, ProjectItem{
			Name:         name,
			DefaultState: defaultState,
			CurrentState: currentState,
			IsCustom:     isCustom,
		})
	}

	// Sort projects alphabetically for consistent display
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return &toggleModel{
		projects:        projects,
		cursor:          0,
		cwd:             cwd,
		toggledProjects: toggledProjects,
		cfg:             cfg,
	}, nil
}

func (m *toggleModel) Init() tea.Cmd {
	return nil
}

func (m *toggleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save before quitting
			if err := SaveToggledProjects(m.cwd, m.toggledProjects); err != nil {
				// Handle error gracefully - could show a message but for now just quit
				return m, tea.Quit
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
				m.updateViewport()
			}

		case tea.KeyDown:
			if m.cursor < len(m.projects)-1 {
				m.cursor++
				m.updateViewport()
			}

		case tea.KeyPgUp:
			// Jump up by page
			availableHeight := m.height - 7
			if availableHeight < 1 {
				availableHeight = 10
			}
			m.cursor -= availableHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateViewport()

		case tea.KeyPgDown:
			// Jump down by page
			availableHeight := m.height - 7
			if availableHeight < 1 {
				availableHeight = 10
			}
			m.cursor += availableHeight
			if m.cursor >= len(m.projects) {
				m.cursor = len(m.projects) - 1
			}
			m.updateViewport()

		case tea.KeyHome:
			// Jump to top
			m.cursor = 0
			m.updateViewport()

		case tea.KeyEnd:
			// Jump to bottom
			m.cursor = len(m.projects) - 1
			m.updateViewport()

		case tea.KeySpace, tea.KeyEnter:
			// Toggle the current project
			m.toggleProject(m.cursor)

		default:
			// Handle character keys
			switch msg.String() {
			case "q":
				// Save before quitting
				if err := SaveToggledProjects(m.cwd, m.toggledProjects); err != nil {
					return m, tea.Quit
				}
				return m, tea.Quit

			case "k":
				if m.cursor > 0 {
					m.cursor--
					m.updateViewport()
				}

			case "j":
				if m.cursor < len(m.projects)-1 {
					m.cursor++
					m.updateViewport()
				}

			case "g":
				// Jump to top (vim-style gg, but single g for simplicity)
				m.cursor = 0
				m.updateViewport()

			case "G":
				// Jump to bottom (vim-style)
				m.cursor = len(m.projects) - 1
				m.updateViewport()

			case "r":
				// Reset the current project to default
				m.resetProject(m.cursor)

			case "R":
				// Reset all projects to default
				m.resetAllProjects()
			}
		}
		// Return the updated model after handling key events
		return m, nil
	}

	return m, nil
}

func (m *toggleModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🔄 Gitte Project Toggle"))
	b.WriteString("\n\n")

	// Project count summary
	enabledCount := 0
	customCount := 0
	for _, proj := range m.projects {
		if proj.CurrentState {
			enabledCount++
		}
		if proj.IsCustom {
			customCount++
		}
	}
	summary := fmt.Sprintf("Projects: %d total, %d enabled, %d custom",
		len(m.projects), enabledCount, customCount)
	b.WriteString(helpStyle.Render(summary))
	b.WriteString("\n\n")

	// Handle empty project list
	if len(m.projects) == 0 {
		b.WriteString(helpStyle.Render("No projects found in configuration."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press q/esc to quit"))
		return b.String()
	}

	// Get visible range
	start, end := m.getVisibleProjects()

	// Show scroll indicator at top if not at beginning
	if start > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↑ %d more above...", start)))
		b.WriteString("\n")
	}

	// Project list - only visible items
	for i := start; i < end; i++ {
		proj := m.projects[i]
		isSelected := i == m.cursor

		// Format the base line first (without styles) to ensure proper spacing
		baseLine := fmt.Sprintf("%s%-40s", "  ", proj.Name)

		// Status indicator
		status := ""
		if proj.CurrentState {
			if isSelected {
				status = enabledStyle.Underline(true).Render("✓ enabled")
			} else {
				status = enabledStyle.Render("✓ enabled")
			}
		} else {
			if isSelected {
				status = disabledStyle.Underline(true).Render("✗ disabled")
			} else {
				status = disabledStyle.Render("✗ disabled")
			}
		}

		// Custom indicator
		customIndicator := ""
		if proj.IsCustom {
			customIndicator = customStyle.Render(" [custom]")
		}

		// Apply cursor and selection style to the base line
		if isSelected {
			baseLine = cursorStyle.Render("▸ ") + selectedStyle.Render(fmt.Sprintf("%-40s", proj.Name))
		}

		line := baseLine + " " + status + customIndicator
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator at bottom if not at end
	if end < len(m.projects) {
		remaining := len(m.projects) - end
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↓ %d more below...", remaining)))
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	helpText := "↑/↓ j/k: nav • PgUp/PgDown: page • Home/End g/G: jump • Space: toggle • r: reset • R: reset all • q: quit"
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

func (m *toggleModel) toggleProject(index int) {
	if index < 0 || index >= len(m.projects) {
		return
	}

	proj := &m.projects[index]
	proj.CurrentState = !proj.CurrentState
	proj.IsCustom = (proj.CurrentState != proj.DefaultState)

	if proj.IsCustom {
		m.toggledProjects[proj.Name] = proj.CurrentState
	} else {
		delete(m.toggledProjects, proj.Name)
	}
}

func (m *toggleModel) resetProject(index int) {
	if index < 0 || index >= len(m.projects) {
		return
	}

	proj := &m.projects[index]
	proj.CurrentState = proj.DefaultState
	proj.IsCustom = false
	delete(m.toggledProjects, proj.Name)
}

func (m *toggleModel) resetAllProjects() {
	for i := range m.projects {
		proj := &m.projects[i]
		proj.CurrentState = proj.DefaultState
		proj.IsCustom = false
	}
	m.toggledProjects = make(config.ToggledProjects)
}

// updateViewport adjusts the viewport offset to keep the cursor visible
func (m *toggleModel) updateViewport() {
	if m.height == 0 {
		return
	}

	// Calculate available height for project list
	// Title (3 lines) + Summary (2 lines) + Help (2 lines) = 7 lines reserved
	availableHeight := m.height - 7
	if availableHeight < 1 {
		availableHeight = 10 // Minimum sensible height
	}

	// Scroll down if cursor is below viewport
	if m.cursor >= m.viewportOffset+availableHeight {
		m.viewportOffset = m.cursor - availableHeight + 1
	}

	// Scroll up if cursor is above viewport
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}

	// Ensure viewport doesn't go negative
	if m.viewportOffset < 0 {
		m.viewportOffset = 0
	}
}

// getVisibleProjects returns the slice of projects that should be visible
func (m *toggleModel) getVisibleProjects() (start, end int) {
	if m.height == 0 {
		return 0, len(m.projects)
	}

	availableHeight := m.height - 7
	if availableHeight < 1 {
		availableHeight = 10
	}

	start = m.viewportOffset
	end = m.viewportOffset + availableHeight

	if end > len(m.projects) {
		end = len(m.projects)
	}

	return start, end
}

// RunToggleTUI starts the interactive TUI for toggling projects
func RunToggleTUI(cfg *config.GitteConfig, cwd string) error {
	model, err := newToggleModel(cfg, cwd)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
