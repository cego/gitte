package gitops

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gitte/output"
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
func newView(mode output.OutputMode, taskNames []string, cancel context.CancelFunc) View {
	if mode == output.ModePlain {
		return &plainView{details: make(map[string]string)}
	}
	return newTUIView(taskNames, cancel)
}

// ---- Plain view --------------------------------------------------------

type plainView struct {
	mu      sync.Mutex
	details map[string]string
}

func (v *plainView) OnStart(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	fmt.Fprintf(os.Stdout, "[%s] RUNNING\n", name)
}

func (v *plainView) OnFinish(name string, err error, elapsed time.Duration) {
	v.mu.Lock()
	detail := v.details[name]
	v.mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stdout, "[%s] FAILED (%s): %s\n", name, fmtDuration(elapsed), err)
		return
	}
	switch {
	case detail == "skipped":
		fmt.Fprintf(os.Stdout, "[%s] SKIPPED: local changes\n", name)
	case strings.HasPrefix(detail, "detached:"):
		fmt.Fprintf(os.Stdout, "[%s] WARNING (%s): %s\n", name, fmtDuration(elapsed), strings.TrimSpace(strings.TrimPrefix(detail, "detached:")))
	case strings.HasPrefix(detail, "stale:"):
		fmt.Fprintf(os.Stdout, "[%s] OK (%s) — WARNING: %s\n", name, fmtDuration(elapsed), strings.TrimSpace(strings.TrimPrefix(detail, "stale:")))
	default:
		d := detail
		if d == "" {
			d = "ok"
		}
		fmt.Fprintf(os.Stdout, "[%s] OK (%s) %s\n", name, fmtDuration(elapsed), d)
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
	goStatePending  goState = iota
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
	program   *tea.Program
	msgCh     chan goUpdateMsg
	doneCh    chan error
	drainedCh chan struct{} // closed by listen() after the last buffered message is consumed
}

func newTUIView(taskNames []string, cancel context.CancelFunc) *tuiView {
	msgCh := make(chan goUpdateMsg, 100)
	drainedCh := make(chan struct{})
	m := newGitopsModel(taskNames, msgCh, drainedCh, cancel)
	p := tea.NewProgram(m)
	v := &tuiView{
		program:   p,
		msgCh:     msgCh,
		doneCh:    make(chan error, 1),
		drainedCh: drainedCh,
	}
	go func() {
		_, err := p.Run()
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
	state := goStateOK
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
// (ensuring the final frame is rendered), then quits the program.
func (v *tuiView) Wait() {
	close(v.msgCh)
	<-v.drainedCh    // last message consumed → renderer has the final frame
	v.program.Quit() // safe to quit now
	<-v.doneCh
}

// ---- BubbleTea model ---------------------------------------------------

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
	entries     []*goEntry
	index       map[string]int
	msgCh       <-chan goUpdateMsg
	spinnerTick int
	drainedCh   chan struct{}
	drainOnce   sync.Once
	cancel      context.CancelFunc
}

func newGitopsModel(names []string, msgCh <-chan goUpdateMsg, drainedCh chan struct{}, cancel context.CancelFunc) *gitopsModel {
	entries := make([]*goEntry, len(names))
	idx := make(map[string]int, len(names))
	for i, n := range names {
		entries[i] = &goEntry{name: n, state: goStatePending}
		idx[n] = i
	}
	return &gitopsModel{entries: entries, index: idx, msgCh: msgCh, drainedCh: drainedCh, cancel: cancel}
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
	case tea.KeyMsg:
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
			m.entries[i].state = msg.state
			m.entries[i].detail = msg.detail
		}

	case goUpdateMsg:
		if i, ok := m.index[msg.name]; ok {
			e := m.entries[i]
			e.elapsed = msg.elapsed
			if msg.state == goStateFailed {
				// Hard failure always wins.
				e.state = goStateFailed
				e.detail = msg.detail
			} else if e.state == goStateRunning {
				// No SetDetail was called; take the state from OnFinish.
				e.state = msg.state
				e.detail = msg.detail
			}
			// Otherwise keep the state set by SetDetail (skipped/stale/detached).
		}
		return m, m.listen()

	case goDoneMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m *gitopsModel) View() string {
	var b strings.Builder
	b.WriteString(goTitleStyle.Render("Syncing repositories") + "\n\n")

	for _, e := range m.entries {
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
			extra = goFailStyle.Render("  " + e.detail)
		}
		b.WriteString(fmt.Sprintf("  %s  %-30s%s\n", icon, nameStr, extra))
	}

	return b.String()
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
