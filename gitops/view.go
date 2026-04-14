package gitops

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/cego/gitte/output"
)

// View handles gitops output in either plain or TUI mode.
type View interface {
	OnStart(name string)
	OnFinish(name string, err error, elapsed time.Duration)
	// SetDetail updates the visual sub-state of a project while it is running.
	// Recognised prefixes: "skipped", "detached: …", "stale: …".
	// Other values ("cloned", "pulled", "up to date") are treated as ok detail text.
	SetDetail(name, detail string)
	Wait()
}

// newView picks the right view implementation based on output mode.
func newView(mode output.OutputMode, title string, taskNames []string, dirs map[string]string, cancel context.CancelFunc) View {
	if mode == output.ModePlain {
		return &plainView{title: title, details: make(map[string]string), dirs: dirs}
	}
	return newTUIView(title, taskNames, cancel)
}

// ---- Plain view --------------------------------------------------------

type plainView struct {
	mu         sync.Mutex
	title      string
	printedHdr bool
	details    map[string]string
	dirs       map[string]string // taskName → relative local dir
}

func (v *plainView) OnStart(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.printedHdr && v.title != "" {
		_, _ = fmt.Fprintf(os.Stdout, "=== %s ===\n", v.title)
		v.printedHdr = true
	}
	if d := v.dirs[name]; d != "" {
		_, _ = fmt.Fprintf(os.Stdout, "[%s] RUNNING  ./%s\n", name, d)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "[%s] RUNNING\n", name)
	}
}

func (v *plainView) OnFinish(name string, err error, elapsed time.Duration) {
	v.mu.Lock()
	detail := v.details[name]
	v.mu.Unlock()

	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "[%s] FAILED (%s): %s\n", name, fmtDuration(elapsed), err)
		return
	}
	switch {
	case detail == "skipped":
		_, _ = fmt.Fprintf(os.Stdout, "[%s] SKIPPED: local changes\n", name)
	case strings.HasPrefix(detail, "detached:"):
		_, _ = fmt.Fprintf(os.Stdout, "[%s] WARNING (%s): %s\n", name, fmtDuration(elapsed), strings.TrimSpace(strings.TrimPrefix(detail, "detached:")))
	case strings.HasPrefix(detail, "stale:"):
		_, _ = fmt.Fprintf(os.Stdout, "[%s] OK (%s) — WARNING: %s\n", name, fmtDuration(elapsed), strings.TrimSpace(strings.TrimPrefix(detail, "stale:")))
	default:
		d := detail
		if d == "" {
			d = "ok"
		}
		_, _ = fmt.Fprintf(os.Stdout, "[%s] OK (%s) %s\n", name, fmtDuration(elapsed), d)
	}
}

func (v *plainView) SetDetail(name, detail string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.details[name] = detail
}

func (v *plainView) Wait() {}

// ---- TUI view ----------------------------------------------------------

type goState int

const (
	goStatePending goState = iota
	goStateRunning
	goStateOK       // cloned / pulled / up to date
	goStateSkipped  // local changes — pull skipped
	goStateDetached // detached HEAD — prompt available
	goStateStale    // on non-default branch, behind by >1 week
	goStateFailed
)

type goEntry struct {
	name    string
	state   goState
	elapsed time.Duration
	detail  string
}

// goUpdateMsg is sent through the listen loop (OnStart / OnFinish).
type goUpdateMsg struct {
	name    string
	state   goState
	elapsed time.Duration
	detail  string
}

// goDetailMsg is sent directly via p.Send() by SetDetail.
type goDetailMsg struct {
	name   string
	state  goState
	detail string
}

type goDoneMsg struct{}
type goTickMsg time.Time

type tuiView struct {
	program    *tea.Program
	msgCh      chan goUpdateMsg
	doneCh     chan error
	drainedCh  chan struct{} // closed by listen() after the last buffered message is consumed
	finalModel *gitopsModel
}

func newTUIView(title string, taskNames []string, cancel context.CancelFunc) *tuiView {
	msgCh := make(chan goUpdateMsg, 100)
	drainedCh := make(chan struct{})
	m := newGitopsModel(title, taskNames, msgCh, drainedCh, cancel)
	p := tea.NewProgram(m)
	v := &tuiView{
		program:   p,
		msgCh:     msgCh,
		doneCh:    make(chan error, 1),
		drainedCh: drainedCh,
	}
	go func() {
		fm, err := p.Run()
		if gm, ok := fm.(*gitopsModel); ok {
			v.finalModel = gm
		}
		v.doneCh <- err
	}()
	return v
}

func (v *tuiView) OnStart(name string) {
	v.msgCh <- goUpdateMsg{name: name, state: goStateRunning}
}

func (v *tuiView) OnFinish(name string, err error, elapsed time.Duration) {
	state := goStateOK
	detail := ""
	if err != nil {
		state = goStateFailed
		detail = err.Error()
	}
	v.msgCh <- goUpdateMsg{name: name, state: state, elapsed: elapsed, detail: detail}
}

// SetDetail signals a sub-state directly to the BubbleTea model, bypassing the
// listen loop so it arrives before the OnFinish goUpdateMsg.
func (v *tuiView) SetDetail(name, detail string) {
	state := goStateRunning
	switch {
	case detail == "skipped":
		state = goStateSkipped
	case strings.HasPrefix(detail, "detached:"):
		state = goStateDetached
	case strings.HasPrefix(detail, "stale:"):
		state = goStateStale
	}
	v.program.Send(goDetailMsg{name: name, state: state, detail: detail})
}

// Wait closes the message channel (signalling no more updates), waits until
// the listen goroutine has delivered the last buffered message to BubbleTea
// (ensuring the final frame is rendered), then quits the program and prints
// a filtered summary of noteworthy entries.
func (v *tuiView) Wait() {
	close(v.msgCh)
	<-v.drainedCh    // last message consumed → renderer has the final frame
	v.program.Quit() // safe to quit now
	<-v.doneCh
	if v.finalModel != nil {
		v.finalModel.printSummary()
	}
}

// ---- BubbleTea model ---------------------------------------------------

var goWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

// PrintWarnings prints collected non-fatal warnings to stderr. Call this after
// all TUI phases have finished so warnings don't corrupt the live output.
func PrintWarnings(mode output.OutputMode, warnings []string) {
	for _, w := range warnings {
		if mode == output.ModePlain {
			fmt.Fprintln(os.Stderr, "warning: "+w)
		} else {
			fmt.Fprintln(os.Stderr, goWarnStyle.Render("⚠ ")+w)
		}
	}
}

var (
	goPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	goRunningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	goOKStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	goSkippedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	goDetachedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	goStaleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	goFailStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	goLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	goTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	goDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

var goSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type gitopsModel struct {
	title       string
	entries     []*goEntry
	index       map[string]int
	msgCh       <-chan goUpdateMsg
	spinnerTick int
	drainedCh   chan struct{}
	drainOnce   sync.Once
	cancel      context.CancelFunc
	width       int
	height      int
	startedAt   time.Time
	finishedAt  time.Time
}

func newGitopsModel(title string, names []string, msgCh <-chan goUpdateMsg, drainedCh chan struct{}, cancel context.CancelFunc) *gitopsModel {
	entries := make([]*goEntry, len(names))
	idx := make(map[string]int, len(names))
	for i, n := range names {
		entries[i] = &goEntry{name: n, state: goStatePending}
		idx[n] = i
	}
	return &gitopsModel{title: title, entries: entries, index: idx, msgCh: msgCh, drainedCh: drainedCh, cancel: cancel}
}

func (m *gitopsModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return goTickMsg(t) }),
		m.listen(),
	)
}

func (m *gitopsModel) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			m.drainOnce.Do(func() { close(m.drainedCh) })
			return goDoneMsg{}
		}
		return msg
	}
}

func (m *gitopsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil // wait for tasks to finish, then goDoneMsg quits
		}

	case goTickMsg:
		m.spinnerTick++
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return goTickMsg(t) })

	case goDetailMsg:
		if i, ok := m.index[msg.name]; ok {
			e := m.entries[i]
			// Don't override a terminal state (OK/Failed) with an in-progress update.
			if e.state != goStateOK && e.state != goStateFailed {
				e.state = msg.state
			}
			e.detail = msg.detail
		}

	case goUpdateMsg:
		if m.startedAt.IsZero() {
			m.startedAt = time.Now()
		}
		if i, ok := m.index[msg.name]; ok {
			e := m.entries[i]
			e.elapsed = msg.elapsed
			if msg.state == goStateFailed {
				// Hard failure always wins.
				e.state = goStateFailed
				e.detail = msg.detail
			} else if msg.state == goStateRunning {
				// OnStart: transition pending → running so the spinner shows.
				if e.state == goStatePending {
					e.state = goStateRunning
				}
			} else if e.state == goStateRunning || e.state == goStatePending {
				// OnFinish: take the terminal state. Preserve detail text already
				// set by SetDetail (msg.detail is empty for non-error OnFinish).
				e.state = msg.state
				if msg.detail != "" {
					e.detail = msg.detail
				}
			}
			// Otherwise keep the state set by SetDetail (skipped/stale/detached).
		}
		return m, m.listen()

	case goDoneMsg:
		m.finishedAt = time.Now()
		return m, tea.Quit
	}

	return m, nil
}

// progressMode reports whether there are too many entries to show in the
// terminal grid — i.e. they would not fit even at maximum column count.
func (m *gitopsModel) progressMode() bool {
	if m.height == 0 {
		return false
	}
	available := m.height - 4 // title + blank line + some margin
	if available < 1 {
		return true
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	nCols := width / 60
	if nCols < 1 {
		nCols = 1
	}
	if nCols > 4 {
		nCols = 4
	}
	return len(m.entries) > available*nCols
}

func (m *gitopsModel) View() tea.View {
	var b strings.Builder
	title := m.title
	if title == "" {
		title = "Syncing repositories"
	}
	if !m.startedAt.IsZero() {
		end := m.finishedAt
		if end.IsZero() {
			end = time.Now()
		}
		title += " " + goDimStyle.Render("("+fmtDuration(end.Sub(m.startedAt))+")")
	}
	b.WriteString(goTitleStyle.Render(title) + "\n\n")

	width := m.width
	if width <= 0 {
		width = 80
	}

	if m.progressMode() {
		b.WriteString(m.renderProgressBar(width) + "\n")
		return tea.NewView(b.String())
	}

	const minColW = 60
	nCols := width / minColW
	if nCols < 1 {
		nCols = 1
	}
	if nCols > 4 {
		nCols = 4
	}
	colW := width / nCols

	lines := make([]string, len(m.entries))
	for i, e := range m.entries {
		lines[i] = m.renderEntry(e, colW)
	}

	nRows := (len(lines) + nCols - 1) / nCols
	for row := 0; row < nRows; row++ {
		for col := 0; col < nCols; col++ {
			idx := col*nRows + row
			if idx >= len(lines) {
				break
			}
			b.WriteString(lines[idx])
		}
		b.WriteString("\n")
	}

	return tea.NewView(b.String())
}

func (m *gitopsModel) renderProgressBar(width int) string {
	total := len(m.entries)
	done, running, failed := 0, 0, 0
	for _, e := range m.entries {
		switch e.state {
		case goStateOK, goStateSkipped, goStateDetached, goStateStale:
			done++
		case goStateFailed:
			done++
			failed++
		case goStateRunning:
			running++
		case goStatePending:
		}
	}

	counts := fmt.Sprintf("  %d/%d", done, total)
	if running > 0 {
		counts += fmt.Sprintf("  running:%d", running)
	}
	if failed > 0 {
		counts += goFailStyle.Render(fmt.Sprintf("  failed:%d", failed))
	}

	barW := width - visibleWidth(counts) - 3 // "[" + "]" + space
	if barW < 4 {
		barW = 4
	}
	filled := 0
	if total > 0 {
		filled = barW * done / total
	}
	bar := goOKStyle.Render(strings.Repeat("█", filled)) + goDimStyle.Render(strings.Repeat("░", barW-filled))
	return "[" + bar + "]" + counts
}

// noteworthyEntries returns entries that need attention after a sync —
// failed, skipped (local changes), detached HEAD, or stale branch.
// All goStateOK entries are excluded regardless of detail (pulled, cloned, up to date).
func (m *gitopsModel) noteworthyEntries() []*goEntry {
	var out []*goEntry
	for _, e := range m.entries {
		if e.state == goStateOK {
			continue
		}
		out = append(out, e)
	}
	return out
}

// printSummary prints a filtered final state after the TUI exits.
// Only noteworthy entries are shown; if all repos are up to date a single
// success line is printed instead.
func (m *gitopsModel) printSummary() {
	noteworthy := m.noteworthyEntries()
	width := m.width
	if width <= 0 {
		width = 80
	}
	if len(noteworthy) == 0 {
		elapsed := m.finishedAt.Sub(m.startedAt)
		fmt.Printf("%s All %d repositories up to date  %s\n",
			goOKStyle.Render("✓"),
			len(m.entries),
			goDimStyle.Render("("+fmtDuration(elapsed)+")"),
		)
		return
	}
	for _, e := range noteworthy {
		if e.state == goStateFailed {
			fmt.Printf(" %s %s  %s\n",
				goFailStyle.Render("✗"),
				goFailStyle.Render(strings.TrimPrefix(e.name, "gitops:")),
				goDimStyle.Render(fmtDuration(e.elapsed)),
			)
			for _, line := range strings.Split(strings.TrimSpace(e.detail), "\n") {
				if line != "" {
					fmt.Printf("   %s\n", goDimStyle.Render(line))
				}
			}
			fmt.Println()
		} else {
			fmt.Println(m.renderEntry(e, width))
		}
	}
}

func (m *gitopsModel) renderEntry(e *goEntry, colW int) string {
	label := strings.TrimPrefix(e.name, "gitops:")
	var icon, nameStr, extra string

	switch e.state {
	case goStatePending:
		icon = goPendingStyle.Render("○")
		nameStr = goDimStyle.Render(label)
	case goStateRunning:
		frame := goSpinnerFrames[m.spinnerTick%len(goSpinnerFrames)]
		icon = goRunningStyle.Render(frame)
		nameStr = goLabelStyle.Render(label)
	case goStateOK:
		icon = goOKStyle.Render("✓")
		nameStr = goOKStyle.Render(label)
		d := e.detail
		if d == "" {
			d = "ok"
		}
		extra = goDimStyle.Render("  " + d + "  " + fmtDuration(e.elapsed))
	case goStateSkipped:
		icon = goSkippedStyle.Render("–")
		nameStr = goSkippedStyle.Render(label)
		extra = goSkippedStyle.Render("  local changes")
	case goStateDetached:
		icon = goDetachedStyle.Render("⚠")
		nameStr = goDetachedStyle.Render(label)
		extra = goDetachedStyle.Render("  " + strings.TrimSpace(strings.TrimPrefix(e.detail, "detached:")))
	case goStateStale:
		icon = goStaleStyle.Render("⚠")
		nameStr = goStaleStyle.Render(label)
		extra = goStaleStyle.Render("  " + strings.TrimSpace(strings.TrimPrefix(e.detail, "stale:")))
	case goStateFailed:
		icon = goFailStyle.Render("✗")
		nameStr = goFailStyle.Render(label)
	}

	line := fmt.Sprintf(" %s %s%s", icon, nameStr, extra)
	return fitToWidth(line, colW)
}

func fitToWidth(s string, width int) string {
	vis := visibleWidth(s)
	if vis > width {
		return truncateToVisualWidth(s, width)
	}
	if vis < width {
		return s + strings.Repeat(" ", width-vis)
	}
	return s
}

func visibleWidth(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

func truncateToVisualWidth(s string, maxWidth int) string {
	if maxWidth < 1 {
		return ""
	}
	var result strings.Builder
	vis := 0
	inEsc := false
	var escBuf strings.Builder
	for _, r := range s {
		if inEsc {
			escBuf.WriteRune(r)
			if r == 'm' {
				inEsc = false
				result.WriteString(escBuf.String())
				escBuf.Reset()
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			escBuf.WriteRune(r)
			continue
		}
		if vis >= maxWidth-1 {
			result.WriteString("\x1b[0m…")
			return result.String()
		}
		result.WriteRune(r)
		vis++
	}
	return result.String()
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
