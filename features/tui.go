package features

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenGateList screen = iota
	screenScopeTree
)

type gateInfo struct {
	Name        string
	Description string
	Gate        config.FeatureGate
}

type featuresModel struct {
	screen screen
	width  int
	height int
	cwd    string
	st     *state.GitteState
	cfg    *config.GitteConfig

	// Gate list state
	gates      []gateInfo
	gateCursor int

	// Scope tree state (populated when entering a gate)
	scopeGate     string
	scopeRows     []ScopeRow
	scopeChecked  map[string]bool
	scopeProjects map[string]ScopeProject
	scopeCursor   int
	scopeOffset   int
	undoStack     []map[string]bool
}

var (
	ftTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	ftHostStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	ftNsStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	ftSelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	ftCurStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	ftCheckStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	ftDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ftHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func newFeaturesModel(cfg *config.GitteConfig, cwd string, st *state.GitteState) *featuresModel {
	gates := make([]gateInfo, 0, len(cfg.FeatureGates))
	for name, gate := range cfg.FeatureGates {
		gates = append(gates, gateInfo{Name: name, Description: gate.Description, Gate: gate})
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Name < gates[j].Name })

	return &featuresModel{
		screen: screenGateList,
		cwd:    cwd,
		st:     st,
		cfg:    cfg,
		gates:  gates,
	}
}

func (m *featuresModel) Init() tea.Cmd { return nil }

func (m *featuresModel) gateStatus(g gateInfo) string {
	fs, ok := m.st.Features[g.Name]
	if !ok || !fs.Enabled {
		return "[ ]"
	}
	if fs.OverrideScope == nil {
		return ftCheckStyle.Render("[✓]")
	}
	if len(fs.OverrideScope.Projects) > 0 || len(fs.OverrideScope.GitlabGroups) > 0 || len(fs.OverrideScope.GithubOrgs) > 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("[·]")
	}
	return "[ ]"
}

func (m *featuresModel) viewGateList() string {
	var b strings.Builder
	b.WriteString(ftTitleStyle.Render("Feature Gates"))
	b.WriteString("\n\n")

	if len(m.gates) == 0 {
		b.WriteString(ftDimStyle.Render("No feature gates defined in configuration."))
		b.WriteString("\n\n")
		b.WriteString(ftHelpStyle.Render("Press q to quit"))
		return b.String()
	}

	for i, g := range m.gates {
		cursor := "  "
		if i == m.gateCursor {
			cursor = ftCurStyle.Render("> ")
		}

		status := m.gateStatus(g)

		var nameStr string
		if i == m.gateCursor {
			nameStr = ftSelStyle.Render(g.Name)
		} else {
			nameStr = g.Name
		}

		desc := ""
		if g.Description != "" {
			desc = " — " + ftDimStyle.Render(g.Description)
		}

		b.WriteString(cursor + status + " " + nameStr + desc + "\n")
	}

	b.WriteString("\n")
	b.WriteString(ftHelpStyle.Render("↑↓/jk: nav  Space: toggle  Enter: edit scope  q/Esc: save & quit"))
	return b.String()
}

// branchState returns tri-state for a branch node.
func (m *featuresModel) branchState(children []string) string {
	all := true
	any := false
	for _, name := range children {
		if m.scopeChecked[name] {
			any = true
		} else {
			all = false
		}
	}
	if all {
		return ftCheckStyle.Render("[✓]")
	}
	if any {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("[·]")
	}
	return "[ ]"
}

func (m *featuresModel) viewScopeTree() string {
	var b strings.Builder

	g := m.gates[m.gateCursor]
	desc := g.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	b.WriteString(ftTitleStyle.Render("← " + g.Name))
	if desc != "" {
		b.WriteString(" — " + ftDimStyle.Render(desc))
	}
	b.WriteString("\n\n")

	avail := m.height - 5
	if avail < 5 {
		avail = 10
	}

	start := m.scopeOffset
	end := start + avail
	if end > len(m.scopeRows) {
		end = len(m.scopeRows)
	}

	for i := start; i < end; i++ {
		row := m.scopeRows[i]
		indent := strings.Repeat("  ", row.Depth)
		isSelected := i == m.scopeCursor

		cursor := "  "
		if isSelected {
			cursor = ftCurStyle.Render("> ")
		}

		switch row.Kind {
		case ScopeRowHost:
			st := m.branchState(row.Children)
			label := ftHostStyle.Render(row.Label)
			if isSelected {
				label = ftSelStyle.Render(row.Label)
			}
			b.WriteString(cursor + indent + st + " " + label + "\n")

		case ScopeRowNamespace:
			st := m.branchState(row.Children)
			label := ftNsStyle.Render(row.Label)
			if isSelected {
				label = ftSelStyle.Render(row.Label)
			}
			b.WriteString(cursor + indent + st + " " + label + "\n")

		case ScopeRowProject:
			var check string
			if m.scopeChecked[row.ProjName] {
				check = ftCheckStyle.Render("[✓]")
			} else {
				check = "[ ]"
			}
			label := row.Label
			if isSelected {
				label = ftSelStyle.Render(label)
			}
			b.WriteString(cursor + indent + check + " " + label + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ftHelpStyle.Render("↑↓/jk: nav  Space/Enter: toggle  Ctrl-Z: undo  Esc: back  q: save & quit"))
	return b.String()
}

// Update handles all key messages.
func (m *featuresModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.screen == screenGateList {
			return m.updateGateList(msg)
		}
		return m.updateScopeTree(msg)
	}
	return m, nil
}

// View renders the current screen.
func (m *featuresModel) View() string {
	if m.screen == screenScopeTree {
		return m.viewScopeTree()
	}
	return m.viewGateList()
}

func (m *featuresModel) updateGateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.save()
		return m, tea.Quit
	case tea.KeyUp:
		if m.gateCursor > 0 {
			m.gateCursor--
		}
	case tea.KeyDown:
		if m.gateCursor < len(m.gates)-1 {
			m.gateCursor++
		}
	case tea.KeyEnter:
		if len(m.gates) > 0 {
			m.enterScopeTree()
		}
	case tea.KeySpace:
		if len(m.gates) > 0 {
			m.quickToggleGate()
		}
	default:
		switch msg.String() {
		case "q":
			m.save()
			return m, tea.Quit
		case "k":
			if m.gateCursor > 0 {
				m.gateCursor--
			}
		case "j":
			if m.gateCursor < len(m.gates)-1 {
				m.gateCursor++
			}
		}
	}
	return m, nil
}

func (m *featuresModel) updateScopeTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.applyScopeChanges()
		m.save()
		return m, tea.Quit
	case tea.KeyEsc:
		m.applyScopeChanges()
		m.screen = screenGateList
		return m, nil
	case tea.KeyUp:
		m.scopeMovePrev()
	case tea.KeyDown:
		m.scopeMoveNext()
	case tea.KeySpace, tea.KeyEnter:
		m.scopeToggle()
	default:
		switch msg.String() {
		case "q":
			m.applyScopeChanges()
			m.save()
			return m, tea.Quit
		case "k":
			m.scopeMovePrev()
		case "j":
			m.scopeMoveNext()
		case "ctrl+z":
			m.scopeUndo()
		}
	}
	return m, nil
}

func (m *featuresModel) scopeMoveNext() {
	if m.scopeCursor < len(m.scopeRows)-1 {
		m.scopeCursor++
		m.scopeUpdateViewport()
	}
}

func (m *featuresModel) scopeMovePrev() {
	if m.scopeCursor > 0 {
		m.scopeCursor--
		m.scopeUpdateViewport()
	}
}

func (m *featuresModel) scopeUpdateViewport() {
	avail := m.height - 5
	if avail < 5 {
		avail = 10
	}
	if m.scopeCursor >= m.scopeOffset+avail {
		m.scopeOffset = m.scopeCursor - avail + 1
	}
	if m.scopeCursor < m.scopeOffset {
		m.scopeOffset = m.scopeCursor
	}
}

func (m *featuresModel) scopeToggle() {
	row := m.scopeRows[m.scopeCursor]

	diff := make(map[string]bool)

	switch row.Kind {
	case ScopeRowProject:
		diff[row.ProjName] = m.scopeChecked[row.ProjName]
		m.scopeChecked[row.ProjName] = !m.scopeChecked[row.ProjName]

	case ScopeRowHost, ScopeRowNamespace:
		anyUnchecked := false
		for _, name := range row.Children {
			if !m.scopeChecked[name] {
				anyUnchecked = true
				break
			}
		}
		target := anyUnchecked
		for _, name := range row.Children {
			diff[name] = m.scopeChecked[name]
			m.scopeChecked[name] = target
		}
	}

	if len(diff) > 0 {
		m.undoStack = append(m.undoStack, diff)
	}
}

func (m *featuresModel) scopeUndo() {
	if len(m.undoStack) == 0 {
		return
	}
	diff := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	for name, prev := range diff {
		m.scopeChecked[name] = prev
	}
}

func (m *featuresModel) enterScopeTree() {
	g := m.gates[m.gateCursor]

	projects := make(map[string]ScopeProject)
	for projName, proj := range m.cfg.Projects {
		host, path, _, err := config.ParseRemoteURL(proj.Remote)
		if err != nil {
			continue
		}
		if projectMatchesScopeByName(projName, host, path, g.Gate.Scope) {
			projects[projName] = ScopeProject{Host: host, Path: path}
		}
	}

	fs := m.st.Features[g.Name]
	var checked map[string]bool
	if !fs.Enabled {
		checked = make(map[string]bool, len(projects))
		for name := range projects {
			checked[name] = false
		}
	} else {
		checked = OverrideToCheckedState(fs.OverrideScope, projects)
	}

	m.scopeGate = g.Name
	m.scopeRows = BuildScopeTree(projects)
	m.scopeChecked = checked
	m.scopeProjects = projects
	m.scopeCursor = 0
	m.scopeOffset = 0
	m.undoStack = nil
	m.screen = screenScopeTree
}

// projectMatchesScopeByName checks if a project matches a config scope (not override).
func projectMatchesScopeByName(projName, host, path string, scope config.FeatureScope) bool {
	if len(scope.Projects) == 0 && len(scope.GitlabGroups) == 0 && len(scope.GithubOrgs) == 0 {
		return true
	}
	for _, p := range scope.Projects {
		if p == projName {
			return true
		}
	}
	for _, gs := range scope.GitlabGroups {
		if gs.Host == host && (path == gs.Group || strings.HasPrefix(path, gs.Group+"/")) {
			return true
		}
	}
	for _, ghs := range scope.GithubOrgs {
		if ghs.Host == host && strings.HasPrefix(path, ghs.Org+"/") {
			return true
		}
	}
	return false
}

func (m *featuresModel) quickToggleGate() {
	g := m.gates[m.gateCursor]
	fs, ok := m.st.Features[g.Name]
	if ok && fs.Enabled {
		delete(m.st.Features, g.Name)
	} else {
		m.st.Features[g.Name] = state.FeatureState{Enabled: true}
	}
}

func (m *featuresModel) applyScopeChanges() {
	anyChecked := false
	for _, v := range m.scopeChecked {
		if v {
			anyChecked = true
			break
		}
	}

	if !anyChecked {
		delete(m.st.Features, m.scopeGate)
		return
	}

	override := CheckedStateToOverride(m.scopeChecked, m.scopeProjects)
	m.st.Features[m.scopeGate] = state.FeatureState{
		Enabled:       true,
		OverrideScope: override,
	}
}

func (m *featuresModel) save() {
	_ = state.Save(m.cwd, m.st)
}

// Run starts the feature gate TUI.
func Run(cfg *config.GitteConfig, cwd string, st *state.GitteState) error {
	if len(cfg.FeatureGates) == 0 {
		fmt.Println("No feature gates defined in configuration.")
		return nil
	}
	model := newFeaturesModel(cfg, cwd, st)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
