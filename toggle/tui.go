package toggle

import (
	"fmt"
	"gitte/config"
	"gitte/state"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectItem represents a project with its toggle state
type ProjectItem struct {
	Name         string
	DefaultState bool
	CurrentState bool
	IsCustom     bool
}

type toggleModel struct {
	projects       []ProjectItem
	cursor         int
	viewportOffset int
	cwd            string
	st             *state.GitteState
	width          int
	height         int
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

func newToggleModel(cfg *config.GitteConfig, cwd string, st *state.GitteState) *toggleModel {
	projects := make([]ProjectItem, 0, len(cfg.Projects))
	for name, proj := range cfg.Projects {
		defaultState := !proj.DefaultDisabled
		currentState := defaultState
		isCustom := false

		if toggled, exists := st.Toggles[name]; exists {
			currentState = toggled
			isCustom = currentState != defaultState
		}

		projects = append(projects, ProjectItem{
			Name:         name,
			DefaultState: defaultState,
			CurrentState: currentState,
			IsCustom:     isCustom,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return &toggleModel{
		projects: projects,
		cursor:   0,
		cwd:      cwd,
		st:       st,
	}
}

func (m *toggleModel) Init() tea.Cmd { return nil }

func (m *toggleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.saveState()
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
			avail := m.availableHeight()
			m.cursor -= avail
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateViewport()

		case tea.KeyPgDown:
			avail := m.availableHeight()
			m.cursor += avail
			if m.cursor >= len(m.projects) {
				m.cursor = len(m.projects) - 1
			}
			m.updateViewport()

		case tea.KeyHome:
			m.cursor = 0
			m.updateViewport()

		case tea.KeyEnd:
			m.cursor = len(m.projects) - 1
			m.updateViewport()

		case tea.KeySpace, tea.KeyEnter:
			m.toggleProject(m.cursor)

		default:
			switch msg.String() {
			case "q":
				m.saveState()
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
				m.cursor = 0
				m.updateViewport()
			case "G":
				m.cursor = len(m.projects) - 1
				m.updateViewport()
			case "r":
				m.resetProject(m.cursor)
			case "R":
				m.resetAllProjects()
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *toggleModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Gitte Project Toggle"))
	b.WriteString("\n\n")

	enabledCount, customCount := 0, 0
	for _, p := range m.projects {
		if p.CurrentState {
			enabledCount++
		}
		if p.IsCustom {
			customCount++
		}
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf(
		"Projects: %d total, %d enabled, %d custom",
		len(m.projects), enabledCount, customCount,
	)))
	b.WriteString("\n\n")

	if len(m.projects) == 0 {
		b.WriteString(helpStyle.Render("No projects found in configuration."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press q/esc to quit"))
		return b.String()
	}

	start, end := m.getVisibleRange()
	if start > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ^ %d more above...", start)))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		p := m.projects[i]
		isSelected := i == m.cursor

		baseLine := fmt.Sprintf("  %-40s", p.Name)

		var statusStr string
		if p.CurrentState {
			if isSelected {
				statusStr = enabledStyle.Underline(true).Render("+ enabled")
			} else {
				statusStr = enabledStyle.Render("+ enabled")
			}
		} else {
			if isSelected {
				statusStr = disabledStyle.Underline(true).Render("- disabled")
			} else {
				statusStr = disabledStyle.Render("- disabled")
			}
		}

		customIndicator := ""
		if p.IsCustom {
			customIndicator = customStyle.Render(" [custom]")
		}

		if isSelected {
			baseLine = cursorStyle.Render("> ") + selectedStyle.Render(fmt.Sprintf("%-40s", p.Name))
		}

		b.WriteString(baseLine + " " + statusStr + customIndicator + "\n")
	}

	if end < len(m.projects) {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  v %d more below...", len(m.projects)-end)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(
		"up/down j/k: nav | PgUp/PgDown: page | Home/End g/G: jump | Space: toggle | r: reset | R: reset all | q: quit",
	))

	return b.String()
}

func (m *toggleModel) toggleProject(index int) {
	if index < 0 || index >= len(m.projects) {
		return
	}
	p := &m.projects[index]
	p.CurrentState = !p.CurrentState
	p.IsCustom = p.CurrentState != p.DefaultState

	if p.IsCustom {
		m.st.Toggles[p.Name] = p.CurrentState
	} else {
		delete(m.st.Toggles, p.Name)
	}
}

func (m *toggleModel) resetProject(index int) {
	if index < 0 || index >= len(m.projects) {
		return
	}
	p := &m.projects[index]
	p.CurrentState = p.DefaultState
	p.IsCustom = false
	delete(m.st.Toggles, p.Name)
}

func (m *toggleModel) resetAllProjects() {
	for i := range m.projects {
		p := &m.projects[i]
		p.CurrentState = p.DefaultState
		p.IsCustom = false
	}
	m.st.Toggles = make(map[string]bool)
}

func (m *toggleModel) saveState() {
	_ = state.Save(m.cwd, m.st)
}

func (m *toggleModel) availableHeight() int {
	h := m.height - 7
	if h < 1 {
		return 10
	}
	return h
}

func (m *toggleModel) updateViewport() {
	avail := m.availableHeight()
	if m.cursor >= m.viewportOffset+avail {
		m.viewportOffset = m.cursor - avail + 1
	}
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}
	if m.viewportOffset < 0 {
		m.viewportOffset = 0
	}
}

func (m *toggleModel) getVisibleRange() (start, end int) {
	avail := m.availableHeight()
	start = m.viewportOffset
	end = start + avail
	if end > len(m.projects) {
		end = len(m.projects)
	}
	return
}

// Run starts the toggle TUI
func Run(cfg *config.GitteConfig, cwd string, st *state.GitteState) error {
	model := newToggleModel(cfg, cwd, st)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
