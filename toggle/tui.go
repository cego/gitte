package toggle

import (
	"fmt"
	"sort"
	"strings"

	"gitte/config"
	"gitte/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectItem holds the mutable toggle state for one project.
type ProjectItem struct {
	Name         string
	DefaultState bool
	CurrentState bool
	IsCustom     bool
}

// rowKind distinguishes host headers, group headers, and project rows.
type rowKind int

const (
	rowKindHost rowKind = iota
	rowKindGroup
	rowKindProject
)

type row struct {
	kind  rowKind
	label string       // for host/group headers
	item  *ProjectItem // non-nil for rowKindProject
}

type toggleModel struct {
	rows           []row
	cursor         int // always points at a rowKindProject row
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

	hostStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

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

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

func newToggleModel(cfg *config.GitteConfig, cwd string, st *state.GitteState) *toggleModel {
	type projEntry struct {
		name      string
		proj      config.ProjectConfig
		host      string
		namespace string
	}

	entries := make([]projEntry, 0, len(cfg.Projects))
	for name, proj := range cfg.Projects {
		host, path, _, err := config.ParseRemoteURL(proj.Remote)
		if err != nil {
			host = "unknown"
			path = name
		}
		namespace := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			namespace = path[:i]
		}
		entries = append(entries, projEntry{name: name, proj: proj, host: host, namespace: namespace})
	}

	// Group by host → namespace.
	type groupKey struct{ host, namespace string }
	type group struct {
		key     groupKey
		entries []projEntry
	}
	groupMap := make(map[groupKey]*group)
	var groupOrder []groupKey

	for _, e := range entries {
		key := groupKey{e.host, e.namespace}
		if _, ok := groupMap[key]; !ok {
			groupMap[key] = &group{key: key}
			groupOrder = append(groupOrder, key)
		}
		groupMap[key].entries = append(groupMap[key].entries, e)
	}

	sort.Slice(groupOrder, func(i, j int) bool {
		a, b := groupOrder[i], groupOrder[j]
		if a.host != b.host {
			return a.host < b.host
		}
		return a.namespace < b.namespace
	})
	for _, key := range groupOrder {
		grp := groupMap[key]
		sort.Slice(grp.entries, func(i, j int) bool {
			return grp.entries[i].name < grp.entries[j].name
		})
	}

	// Build flat row list.
	var rows []row
	prevHost := ""
	for _, key := range groupOrder {
		if key.host != prevHost {
			rows = append(rows, row{kind: rowKindHost, label: key.host})
			prevHost = key.host
		}
		groupLabel := key.namespace
		if groupLabel == "" {
			groupLabel = "(root)"
		}
		rows = append(rows, row{kind: rowKindGroup, label: groupLabel})

		for _, e := range groupMap[key].entries {
			defaultState := !e.proj.DefaultDisabled
			currentState := defaultState
			isCustom := false
			if toggled, exists := st.Toggles[e.name]; exists {
				currentState = toggled
				isCustom = currentState != defaultState
			}
			item := &ProjectItem{
				Name:         e.name,
				DefaultState: defaultState,
				CurrentState: currentState,
				IsCustom:     isCustom,
			}
			rows = append(rows, row{kind: rowKindProject, item: item})
		}
	}

	// Initial cursor: first project row.
	cursor := 0
	for i, r := range rows {
		if r.kind == rowKindProject {
			cursor = i
			break
		}
	}

	return &toggleModel{rows: rows, cursor: cursor, cwd: cwd, st: st}
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
			m.movePrev()
		case tea.KeyDown:
			m.moveNext()
		case tea.KeyPgUp:
			m.movePrevPage()
		case tea.KeyPgDown:
			m.moveNextPage()
		case tea.KeyHome:
			m.moveFirst()
		case tea.KeyEnd:
			m.moveLast()
		case tea.KeySpace, tea.KeyEnter:
			m.toggleCurrent()
		default:
			switch msg.String() {
			case "q":
				m.saveState()
				return m, tea.Quit
			case "k":
				m.movePrev()
			case "j":
				m.moveNext()
			case "g":
				m.moveFirst()
			case "G":
				m.moveLast()
			case "r":
				m.resetCurrent()
			case "R":
				m.resetAll()
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

	total, enabled, custom := 0, 0, 0
	for _, r := range m.rows {
		if r.kind != rowKindProject {
			continue
		}
		total++
		if r.item.CurrentState {
			enabled++
		}
		if r.item.IsCustom {
			custom++
		}
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf(
		"Projects: %d total, %d enabled, %d custom",
		total, enabled, custom,
	)))
	b.WriteString("\n\n")

	if total == 0 {
		b.WriteString(helpStyle.Render("No projects found in configuration."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press q/esc to quit"))
		return b.String()
	}

	start, end := m.getVisibleRange()
	if start > 0 {
		// Count hidden project rows above
		hidden := 0
		for i := 0; i < start; i++ {
			if m.rows[i].kind == rowKindProject {
				hidden++
			}
		}
		if hidden > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ^ %d more above…", hidden)))
			b.WriteString("\n")
		}
	}

	for i := start; i < end; i++ {
		r := m.rows[i]
		switch r.kind {
		case rowKindHost:
			b.WriteString(hostStyle.Render(r.label))
			b.WriteString("\n")
		case rowKindGroup:
			b.WriteString("  " + groupStyle.Render(r.label))
			b.WriteString("\n")
		case rowKindProject:
			p := r.item
			isSelected := i == m.cursor

			cursor := "  "
			if isSelected {
				cursor = cursorStyle.Render("> ")
			}

			var nameStr string
			if isSelected {
				nameStr = selectedStyle.Render(fmt.Sprintf("%-36s", p.Name))
			} else {
				nameStr = fmt.Sprintf("%-36s", p.Name)
			}

			var statusStr string
			if p.CurrentState {
				statusStr = enabledStyle.Render("+ enabled ")
			} else {
				statusStr = disabledStyle.Render("- disabled")
			}

			customIndicator := ""
			if p.IsCustom {
				customIndicator = "  " + customStyle.Render("[custom]")
			}

			b.WriteString("    " + cursor + nameStr + "  " + statusStr + customIndicator + "\n")
		}
	}

	if end < len(m.rows) {
		hidden := 0
		for i := end; i < len(m.rows); i++ {
			if m.rows[i].kind == rowKindProject {
				hidden++
			}
		}
		if hidden > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  v %d more below…", hidden)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(
		"↑↓/jk: nav  PgUp/PgDown: page  g/G: top/bottom  Space: toggle  r: reset  R: reset all  q: quit",
	))

	return b.String()
}

// ── navigation ───────────────────────────────────────────────────────────────

func (m *toggleModel) moveNext() {
	for i := m.cursor + 1; i < len(m.rows); i++ {
		if m.rows[i].kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
}

func (m *toggleModel) movePrev() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
}

func (m *toggleModel) moveFirst() {
	for i, r := range m.rows {
		if r.kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
}

func (m *toggleModel) moveLast() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if m.rows[i].kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
}

func (m *toggleModel) moveNextPage() {
	avail := m.availableHeight()
	target := m.cursor + avail
	if target >= len(m.rows) {
		target = len(m.rows) - 1
	}
	// snap to nearest project row at or before target
	for i := target; i > m.cursor; i-- {
		if m.rows[i].kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
	// already at last project; move to absolute last
	m.moveLast()
}

func (m *toggleModel) movePrevPage() {
	avail := m.availableHeight()
	target := m.cursor - avail
	if target < 0 {
		target = 0
	}
	// snap to nearest project row at or after target
	for i := target; i < m.cursor; i++ {
		if m.rows[i].kind == rowKindProject {
			m.cursor = i
			m.updateViewport()
			return
		}
	}
	m.moveFirst()
}

// ── toggle / reset ───────────────────────────────────────────────────────────

func (m *toggleModel) toggleCurrent() {
	r := &m.rows[m.cursor]
	if r.kind != rowKindProject || r.item == nil {
		return
	}
	p := r.item
	p.CurrentState = !p.CurrentState
	p.IsCustom = p.CurrentState != p.DefaultState
	if p.IsCustom {
		m.st.Toggles[p.Name] = p.CurrentState
	} else {
		delete(m.st.Toggles, p.Name)
	}
}

func (m *toggleModel) resetCurrent() {
	r := &m.rows[m.cursor]
	if r.kind != rowKindProject || r.item == nil {
		return
	}
	p := r.item
	p.CurrentState = p.DefaultState
	p.IsCustom = false
	delete(m.st.Toggles, p.Name)
}

func (m *toggleModel) resetAll() {
	for i := range m.rows {
		if m.rows[i].kind == rowKindProject && m.rows[i].item != nil {
			p := m.rows[i].item
			p.CurrentState = p.DefaultState
			p.IsCustom = false
		}
	}
	m.st.Toggles = make(map[string]bool)
}

func (m *toggleModel) saveState() {
	_ = state.Save(m.cwd, m.st)
}

// ── viewport ─────────────────────────────────────────────────────────────────

func (m *toggleModel) availableHeight() int {
	h := m.height - 8 // title + blank + stats + blank + help + margins
	if h < 5 {
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
	if end > len(m.rows) {
		end = len(m.rows)
	}
	return
}

// Run starts the toggle TUI.
func Run(cfg *config.GitteConfig, cwd string, st *state.GitteState) error {
	model := newToggleModel(cfg, cwd, st)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
