package startup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View handles startup check output in either plain or TUI mode.
type View interface {
	// OnStart is called when a check begins.
	OnStart(name string)
	// OnFinish is called when a check completes. err is nil on success.
	OnFinish(name string, err error, elapsed time.Duration)
	// Wait blocks until the view has finished rendering all output.
	Wait()
}

// ---- Plain view --------------------------------------------------------

type plainView struct {
	mu sync.Mutex
}

func newPlainView() *plainView { return &plainView{} }

func (v *plainView) OnStart(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "[startup:%s] RUNNING\n", name)
}

func (v *plainView) OnFinish(name string, err error, elapsed time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "[startup:%s] FAILED (%s): %s\n", name, fmtDuration(elapsed), err)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "[startup:%s] OK (%s)\n", name, fmtDuration(elapsed))
	}
}

func (v *plainView) Wait() {}

// ---- TUI view ----------------------------------------------------------

type checkState int

const (
	checkPending checkState = iota
	checkRunning
	checkOK
	checkFailed
)

type checkEntry struct {
	name    string
	state   checkState
	elapsed time.Duration
	err     error
}

// tuiUpdateMsg carries a state change for a single check.
type tuiUpdateMsg struct {
	name    string
	state   checkState
	elapsed time.Duration
	err     error
}

// allDoneMsg is sent when msgCh is closed (all updates delivered).
type allDoneMsg struct{}

type tuiTickMsg time.Time

type failureRecord struct {
	name    string
	errMsg  string
	hint    string
	elapsed time.Duration
}

type tuiView struct {
	program   *tea.Program
	msgCh     chan tuiUpdateMsg
	doneCh    chan error
	drainedCh chan struct{} // closed by listen() after the last buffered message is consumed
	mu        sync.Mutex
	failures  []failureRecord
}

func newTUIView(checkNames []string, cancel context.CancelFunc) *tuiView {
	msgCh := make(chan tuiUpdateMsg, 100)
	drainedCh := make(chan struct{})
	m := newStartupModel(checkNames, msgCh, drainedCh, cancel)
	// No alt-screen: startup checks render inline so the output stays visible.
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
	v.msgCh <- tuiUpdateMsg{name: name, state: checkRunning}
}

func (v *tuiView) OnFinish(name string, err error, elapsed time.Duration) {
	state := checkOK
	if err != nil {
		state = checkFailed
		errMsg, hint := splitErrHint(err)
		v.mu.Lock()
		v.failures = append(v.failures, failureRecord{name: name, errMsg: errMsg, hint: hint, elapsed: elapsed})
		v.mu.Unlock()
	}
	v.msgCh <- tuiUpdateMsg{name: name, state: state, elapsed: elapsed, err: err}
}

// Wait closes the message channel (signalling no more updates), waits until
// the listen goroutine has delivered the last buffered message to BubbleTea
// (ensuring the final frame is rendered), then quits the program and prints
// a summary of any failed checks.
func (v *tuiView) Wait() {
	close(v.msgCh)
	<-v.drainedCh    // last message consumed → renderer has the final frame
	v.program.Quit() // safe to quit now
	<-v.doneCh
	v.printFailureSummary()
}

// printFailureSummary prints a clean per-failure block after the TUI exits.
func (v *tuiView) printFailureSummary() {
	v.mu.Lock()
	failures := v.failures
	v.mu.Unlock()
	if len(failures) == 0 {
		return
	}
	fmt.Println()
	for _, f := range failures {
		fmt.Printf(" %s %s  %s\n", failStyle.Render("✗"), failStyle.Render(f.name), dimStyle.Render(fmtDuration(f.elapsed)))
		fmt.Printf("   %s\n", dimStyle.Render(f.errMsg))
		if f.hint != "" {
			fmt.Printf("   %s %s\n", hintLabelStyle.Render("hint:"), hintTextStyle.Render(f.hint))
		}
		fmt.Println()
	}
}

// ---- BubbleTea model ---------------------------------------------------

var (
	pendingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	runningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hintLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	hintTextStyle  = lipgloss.NewStyle()
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type startupModel struct {
	checks      []*checkEntry
	index       map[string]int
	msgCh       <-chan tuiUpdateMsg
	spinnerTick int
	drainedCh   chan struct{}
	drainOnce   sync.Once
	cancel      context.CancelFunc
	width       int
	startedAt   time.Time
	finishedAt  time.Time
}

func newStartupModel(names []string, msgCh <-chan tuiUpdateMsg, drainedCh chan struct{}, cancel context.CancelFunc) *startupModel {
	checks := make([]*checkEntry, len(names))
	idx := make(map[string]int, len(names))
	for i, n := range names {
		checks[i] = &checkEntry{name: n, state: checkPending}
		idx[n] = i
	}
	return &startupModel{checks: checks, index: idx, msgCh: msgCh, drainedCh: drainedCh, cancel: cancel}
}

func (m *startupModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return tuiTickMsg(t) }),
		m.listen(),
	)
}

// listen waits for the next message on msgCh.
// When the channel is closed (and drained) it signals drainedCh so that
// Wait() knows the final frame has been handed to BubbleTea, then returns
// allDoneMsg so the model can quit.
func (m *startupModel) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			m.drainOnce.Do(func() { close(m.drainedCh) })
			return allDoneMsg{}
		}
		return msg
	}
}

func (m *startupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil // wait for tasks to finish, then allDoneMsg quits
		}

	case tuiTickMsg:
		m.spinnerTick++
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return tuiTickMsg(t) })

	case tuiUpdateMsg:
		if m.startedAt.IsZero() {
			m.startedAt = time.Now()
		}
		if i, ok := m.index[msg.name]; ok {
			m.checks[i].state = msg.state
			m.checks[i].elapsed = msg.elapsed
			m.checks[i].err = msg.err
		}
		// Always re-queue listen so we drain the channel completely.
		return m, m.listen()

	case allDoneMsg:
		m.finishedAt = time.Now()
		return m, tea.Quit
	}

	return m, nil
}

func (m *startupModel) View() string {
	var b strings.Builder
	title := "Startup checks"
	if !m.startedAt.IsZero() {
		end := m.finishedAt
		if end.IsZero() {
			end = time.Now()
		}
		title += " " + dimStyle.Render("("+fmtDuration(end.Sub(m.startedAt))+")")
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	width := m.width
	if width <= 0 {
		width = 80
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

	lines := make([]string, len(m.checks))
	for i, c := range m.checks {
		lines[i] = m.renderEntry(c, colW)
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

	return b.String()
}

func (m *startupModel) renderEntry(c *checkEntry, colW int) string {
	var icon, nameStr, extra string
	switch c.state {
	case checkPending:
		icon = pendingStyle.Render("○")
		nameStr = dimStyle.Render(c.name)
	case checkRunning:
		frame := spinnerFrames[m.spinnerTick%len(spinnerFrames)]
		icon = runningStyle.Render(frame)
		nameStr = labelStyle.Render(c.name)
	case checkOK:
		icon = okStyle.Render("✓")
		nameStr = okStyle.Render(c.name)
		extra = dimStyle.Render("  " + fmtDuration(c.elapsed))
	case checkFailed:
		icon = failStyle.Render("✗")
		nameStr = failStyle.Render(c.name)
		extra = dimStyle.Render("  " + fmtDuration(c.elapsed))
	}

	line := fmt.Sprintf(" %s %s%s", icon, nameStr, extra)
	return fitToWidth(line, colW)
}

// ---- helpers -----------------------------------------------------------

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
		if vis >= maxWidth {
			result.WriteString("\x1b[0m")
			return result.String()
		}
		result.WriteRune(r)
		vis++
	}
	return result.String()
}

// splitErrHint splits an error message of the form "msg\nhint: hint" into its
// two parts. If there is no hint suffix the second return value is empty.
func splitErrHint(err error) (string, string) {
	const sep = "\nhint: "
	msg := err.Error()
	if i := strings.Index(msg, sep); i >= 0 {
		return msg[:i], msg[i+len(sep):]
	}
	return msg, ""
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
