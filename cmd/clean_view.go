package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cego/gitte/output"
)

// ---- states ----------------------------------------------------------------

type cleanState int

const (
	cleanStatePending cleanState = iota
	cleanStateRunning
	cleanStateOK
	cleanStateFailed
)

// ---- data types ------------------------------------------------------------

type cleanEntry struct {
	name   string
	state  cleanState
	detail string
}

type cleanPhaseModel struct {
	title   string
	entries []*cleanEntry
	index   map[string]int
}

// cleanPhaseSpec is used by callers to declare a phase with its repos upfront.
type cleanPhaseSpec struct {
	Title string
	Repos []string
}

// ---- messages --------------------------------------------------------------

// cleanMsg carries a state change for one repo within a phase.
// state must be cleanStateRunning, cleanStateOK, or cleanStateFailed;
// cleanStatePending is the initial state set by newCleanModel, never sent.
type cleanMsg struct {
	phase  string // matches cleanPhaseSpec.Title
	repo   string
	state  cleanState
	detail string
}

// cleanAdvanceMsg is sent via program.Send() (not through msgCh) to increment
// activePhase. Because it bypasses the channel, it never interferes with the
// listen() goroutine that is already waiting for the next cleanMsg.
type cleanAdvanceMsg struct{} // advance to next phase
type cleanDoneMsg struct{}    // all messages consumed
type cleanTickMsg time.Time

// ---- styles ----------------------------------------------------------------

var (
	cleanPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cleanRunningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	cleanOKStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	cleanFailStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	cleanLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cleanSectionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	cleanDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	cleanSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// ---- BubbleTea model -------------------------------------------------------

type cleanModel struct {
	phases      []cleanPhaseModel
	phaseIndex  map[string]int // phase title → index in phases
	activePhase int
	msgCh       <-chan cleanMsg
	spinnerTick int
	width       int
}

func newCleanModel(specs []cleanPhaseSpec, msgCh <-chan cleanMsg) *cleanModel {
	phases := make([]cleanPhaseModel, len(specs))
	idx := make(map[string]int, len(specs))
	for i, spec := range specs {
		entries := make([]*cleanEntry, len(spec.Repos))
		entryIdx := make(map[string]int, len(spec.Repos))
		for j, r := range spec.Repos {
			entries[j] = &cleanEntry{name: r, state: cleanStatePending}
			entryIdx[r] = j
		}
		phases[i] = cleanPhaseModel{title: spec.Title, entries: entries, index: entryIdx}
		idx[spec.Title] = i
	}
	return &cleanModel{
		phases:     phases,
		phaseIndex: idx,
		msgCh:      msgCh,
	}
}

func (m *cleanModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return cleanTickMsg(t) }),
		m.listen(),
	)
}

func (m *cleanModel) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			return cleanDoneMsg{}
		}
		return msg
	}
}

func (m *cleanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case cleanTickMsg:
		m.spinnerTick++
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return cleanTickMsg(t) })

	case cleanAdvanceMsg:
		if m.activePhase < len(m.phases)-1 {
			m.activePhase++
		}

	case cleanMsg:
		if pi, ok := m.phaseIndex[msg.phase]; ok {
			phase := &m.phases[pi]
			if ei, ok := phase.index[msg.repo]; ok {
				e := phase.entries[ei]
				switch msg.state {
				case cleanStatePending:
					// not sent by workers; initial state is set by newCleanModel
				case cleanStateRunning:
					if e.state == cleanStatePending {
						e.state = cleanStateRunning
					}
					if msg.detail != "" {
						e.detail = msg.detail
					}
				case cleanStateOK, cleanStateFailed:
					e.state = msg.state
					if msg.detail != "" {
						e.detail = msg.detail
					}
				}
			}
		}
		return m, m.listen()

	case cleanDoneMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m *cleanModel) View() string {
	var b strings.Builder
	width := m.width
	if width <= 0 {
		width = 80
	}

	for i, phase := range m.phases {
		isUpcoming := i > m.activePhase
		header := renderPhaseHeader(phase.title, width)
		if isUpcoming {
			b.WriteString(cleanDimStyle.Render(header) + "\n")
			continue
		}
		b.WriteString(cleanSectionStyle.Render(header) + "\n")

		colCount := colsForWidth(width)
		colW := width / colCount

		lines := make([]string, len(phase.entries))
		for j, e := range phase.entries {
			lines[j] = m.renderEntry(e, colW)
		}

		nRows := (len(lines) + colCount - 1) / colCount
		for row := 0; row < nRows; row++ {
			for col := 0; col < colCount; col++ {
				idx := col*nRows + row
				if idx >= len(lines) {
					break
				}
				b.WriteString(lines[idx])
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderPhaseHeader(title string, width int) string {
	prefix := "── " + title + " "
	fill := width - len([]rune(prefix))
	if fill < 0 {
		fill = 0
	}
	return prefix + strings.Repeat("─", fill)
}

func (m *cleanModel) renderEntry(e *cleanEntry, colW int) string {
	var icon, nameStr, extra string
	switch e.state {
	case cleanStatePending:
		icon = cleanPendingStyle.Render("○")
		nameStr = cleanDimStyle.Render(e.name)
	case cleanStateRunning:
		frame := cleanSpinnerFrames[m.spinnerTick%len(cleanSpinnerFrames)]
		icon = cleanRunningStyle.Render(frame)
		nameStr = cleanLabelStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanDimStyle.Render("  " + e.detail)
		}
	case cleanStateOK:
		icon = cleanOKStyle.Render("✓")
		nameStr = cleanOKStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanDimStyle.Render("  " + e.detail)
		}
	case cleanStateFailed:
		icon = cleanFailStyle.Render("✗")
		nameStr = cleanFailStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanFailStyle.Render("  " + e.detail)
		}
	}
	line := fmt.Sprintf(" %s %s%s", icon, nameStr, extra)
	return cleanFitToWidth(line, colW)
}

// cleanFitToWidth pads or truncates s to exactly width visible characters.
func cleanFitToWidth(s string, width int) string {
	vis := cleanVisibleWidth(s)
	if vis < width {
		return s + strings.Repeat(" ", width-vis)
	}
	return cleanTruncateToVisualWidth(s, width)
}

// cleanVisibleWidth returns the display width of s, ignoring ANSI escape codes.
func cleanVisibleWidth(s string) int {
	width := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		width++
	}
	return width
}

// cleanTruncateToVisualWidth truncates s to at most maxWidth visible characters,
// preserving ANSI escape codes that appear before the truncation point.
func cleanTruncateToVisualWidth(s string, maxWidth int) string {
	var b strings.Builder
	width := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			b.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			b.WriteRune(r)
			continue
		}
		if width >= maxWidth {
			b.WriteString("\x1b[0m") // reset to avoid color bleed into adjacent columns
			break
		}
		b.WriteRune(r)
		width++
	}
	return b.String()
}

// colsForWidth returns the number of columns based on terminal width.
// ≥180 cols → 3, ≥120 cols → 2, otherwise → 1.
func colsForWidth(width int) int {
	if width >= 180 {
		return 3
	}
	if width >= 120 {
		return 2
	}
	return 1
}

// ---- view wrapper ----------------------------------------------------------

// cleanView wraps the BubbleTea program (TUI mode) or prints plain lines
// (plain mode) for the clean subcommands.
type cleanView struct {
	// TUI fields (nil/zero in plain mode)
	program    *tea.Program
	msgCh      chan cleanMsg
	doneCh     chan error
	finalModel *cleanModel
	// shared
	mode output.OutputMode
}

// newCleanView creates a cleanView for the given phases. In TUI mode the
// BubbleTea program is started immediately in alt-screen mode so the initial
// render never pollutes the terminal scrollback. In plain mode no program is started.
func newCleanView(mode output.OutputMode, specs []cleanPhaseSpec) *cleanView {
	v := &cleanView{mode: mode}
	if mode == output.ModePlain {
		return v
	}
	msgCh := make(chan cleanMsg, 100)
	m := newCleanModel(specs, msgCh)
	p := tea.NewProgram(m, tea.WithAltScreen())
	v.program = p
	v.msgCh = msgCh
	v.doneCh = make(chan error, 1)
	go func() {
		fm, err := p.Run()
		if cm, ok := fm.(*cleanModel); ok {
			v.finalModel = cm
		}
		v.doneCh <- err
	}()
	return v
}

// OnStart marks a repo as running within its phase.
func (v *cleanView) OnStart(phase, repo string) {
	if v.mode == output.ModePlain {
		return
	}
	v.msgCh <- cleanMsg{phase: phase, repo: repo, state: cleanStateRunning}
}

// SetDetail updates the detail text for a running repo (e.g. "cleaning…").
// In plain mode this is a no-op; detail is printed by OnFinish.
func (v *cleanView) SetDetail(phase, repo, detail string) {
	if v.mode == output.ModePlain {
		return
	}
	v.msgCh <- cleanMsg{phase: phase, repo: repo, state: cleanStateRunning, detail: detail}
}

// OnFinish marks a repo as completed. detail is shown alongside the ✓/✗ icon.
// In plain mode it prints "[clean:<phase>] <repo>: <detail>" to stdout (or stderr on error).
func (v *cleanView) OnFinish(phase, repo, detail string, err error) {
	if v.mode == output.ModePlain {
		phaseKey := strings.ToLower(strings.ReplaceAll(phase, " ", "-"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "[clean:%s] %s: failed: %v\n", phaseKey, repo, err)
		} else {
			fmt.Printf("[clean:%s] %s: %s\n", phaseKey, repo, detail)
		}
		return
	}
	state := cleanStateOK
	if err != nil {
		state = cleanStateFailed
		if detail == "" {
			detail = err.Error()
		}
	}
	v.msgCh <- cleanMsg{phase: phase, repo: repo, state: state, detail: detail}
}

// AdvancePhase signals the model to move to the next phase section.
// Call this after all repos in the current phase have reported OnFinish.
// Only meaningful in TUI mode; in plain mode it is a no-op.
func (v *cleanView) AdvancePhase() {
	if v.mode != output.ModePlain && v.program != nil {
		v.program.Send(cleanAdvanceMsg{})
	}
}

// Wait closes the message channel, blocks until the BubbleTea program exits,
// then prints the final frame to the primary screen. Alt-screen mode clears
// the TUI on exit, so we must reprint to leave the results visible.
// In plain mode it is a no-op.
func (v *cleanView) Wait() {
	if v.mode == output.ModePlain {
		return
	}
	close(v.msgCh)
	<-v.doneCh
	if v.finalModel != nil {
		fmt.Print(v.finalModel.View())
	}
}
