package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cego/gitte/executor"
	"github.com/cego/gitte/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskInfo carries metadata needed for tree display.
type TaskInfo struct {
	TaskName      string
	Project       string
	Action        string
	Group         string
	Host          string
	PathSegs      []string // namespace path segments (not the leaf)
	ProjLeaf      string   // last segment of remote path (project folder name)
	ProjectDir    string   // absolute path to the project on disk (empty if unknown)
	DefaultBranch string   // default branch (e.g. "master", "main")
}

// View handles action execution output.
type View interface {
	OnStart(name string)
	OnReset(name string) // task was re-queued for retry by the executor
	OnFinish(name string, err error, elapsed time.Duration)
	Handler() executor.OutputHandler
	// WaitAndGetRetry blocks after Execute finishes. Returns nil (user quit) or task
	// names to retry (user pressed r/R after all tasks completed).
	WaitAndGetRetry() []string
	// PrepareRetry resets the named tasks to pending so the TUI can be reused for
	// a new executor run. Only called when WaitAndGetRetry returns non-nil.
	PrepareRetry(taskNames []string, retryCh chan []string, cancel context.CancelFunc)
}

func newView(mode output.OutputMode, tasks []TaskInfo, actionOrder []string, cancel context.CancelFunc, retryCh chan []string, gitCleanExcludes []string) View {
	if mode == output.ModePlain {
		return newPlainActionsView()
	}
	return newTUIActionsView(tasks, actionOrder, cancel, retryCh, gitCleanExcludes)
}

// ── Plain view ────────────────────────────────────────────────────────────────

type plainActionsView struct{ mu sync.Mutex }

func newPlainActionsView() *plainActionsView { return &plainActionsView{} }

func (v *plainActionsView) OnStart(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "[action:%s] RUNNING\n", name)
}

func (v *plainActionsView) OnReset(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "[action:%s] RETRY\n", name)
}

func (v *plainActionsView) OnFinish(name string, err error, elapsed time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "[action:%s] FAILED (%s): %s\n", name, fmtActionDuration(elapsed), err)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "[action:%s] OK (%s)\n", name, fmtActionDuration(elapsed))
	}
}

func (v *plainActionsView) Handler() executor.OutputHandler                                { return &plainActionsHandler{mu: &v.mu} }
func (v *plainActionsView) WaitAndGetRetry() []string                                      { return nil }
func (v *plainActionsView) PrepareRetry(_ []string, _ chan []string, _ context.CancelFunc) {}

type plainActionsHandler struct{ mu *sync.Mutex }

func (h *plainActionsHandler) HandleOutput(_ context.Context, out executor.Output) error {
	line := strings.TrimRight(string(out.Output), "\n\r")
	if line == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "[action:%s] %s\n", out.CmdName, line)
	return nil
}

// ── Tree structure ────────────────────────────────────────────────────────────

type rowKind int

const (
	rowKindAction        rowKind = iota
	rowKindHost                  // remote host (e.g. gitlab.cego.dk)
	rowKindNamespace             // namespace path segment
	rowKindProjectHeader         // project label when it has >1 group
	rowKindTask                  // selectable leaf
)

type treeRow struct {
	kind     rowKind
	label    string
	depth    int
	taskName string // only for rowKindTask
	action   string // action this row belongs to
}

// buildTreeRows builds a flat list of display rows from a set of task infos.
// Tree shape: action → host → namespace-segments → project (→ group if >1).
// actionOrder controls the display order of action sections.
func buildTreeRows(tasks []TaskInfo, actionOrder []string) []treeRow {
	actMap := make(map[string][]TaskInfo)
	for _, t := range tasks {
		actMap[t.Action] = append(actMap[t.Action], t)
	}

	// Collect any actions not in actionOrder (edge case) so nothing is lost.
	inOrder := make(map[string]bool, len(actionOrder))
	for _, a := range actionOrder {
		inOrder[a] = true
	}
	ordered := make([]string, 0, len(actMap))
	ordered = append(ordered, actionOrder...)
	for a := range actMap {
		if !inOrder[a] {
			ordered = append(ordered, a)
		}
	}

	var rows []treeRow
	for _, action := range ordered {
		ts, ok := actMap[action]
		if !ok {
			continue
		}
		rows = append(rows, treeRow{kind: rowKindAction, label: action, depth: 0, action: action})
		hostMap := make(map[string][]TaskInfo)
		for _, t := range ts {
			hostMap[t.Host] = append(hostMap[t.Host], t)
		}
		hosts := stringKeys(hostMap)
		sort.Strings(hosts)
		for _, host := range hosts {
			rows = append(rows, treeRow{kind: rowKindHost, label: host, depth: 1, action: action})
			rows = append(rows, flattenNS(hostMap[host], 2, action)...)
		}
	}
	return rows
}

// flattenNS recursively flattens the namespace segments into rows starting at depth.
func flattenNS(tasks []TaskInfo, depth int, action string) []treeRow {
	nsMap := make(map[string][]TaskInfo)   // first path seg → tasks
	leafMap := make(map[string][]TaskInfo) // projLeaf → tasks (path segs exhausted)

	for _, t := range tasks {
		if len(t.PathSegs) == 0 {
			leafMap[t.ProjLeaf] = append(leafMap[t.ProjLeaf], t)
		} else {
			first := t.PathSegs[0]
			t2 := t
			t2.PathSegs = t.PathSegs[1:]
			nsMap[first] = append(nsMap[first], t2)
		}
	}

	// Namespaces (folders) first, sorted; then leaf projects, sorted.
	nsLabels := make([]string, 0, len(nsMap))
	for ns := range nsMap {
		nsLabels = append(nsLabels, ns)
	}
	sort.Strings(nsLabels)

	leafLabels := make([]string, 0, len(leafMap))
	for leaf := range leafMap {
		if _, isNS := nsMap[leaf]; !isNS {
			leafLabels = append(leafLabels, leaf)
		}
	}
	sort.Strings(leafLabels)

	var rows []treeRow
	for _, label := range leafLabels {
		leafTasks := leafMap[label]
		sort.Slice(leafTasks, func(i, j int) bool { return leafTasks[i].Group < leafTasks[j].Group })
		if len(leafTasks) == 1 {
			rows = append(rows, treeRow{kind: rowKindTask, label: label, depth: depth, taskName: leafTasks[0].TaskName, action: action})
		} else {
			rows = append(rows, treeRow{kind: rowKindProjectHeader, label: label, depth: depth, action: action})
			for _, t := range leafTasks {
				rows = append(rows, treeRow{kind: rowKindTask, label: t.Group, depth: depth + 1, taskName: t.TaskName, action: action})
			}
		}
	}
	for _, label := range nsLabels {
		rows = append(rows, treeRow{kind: rowKindNamespace, label: label, depth: depth, action: action})
		rows = append(rows, flattenNS(nsMap[label], depth+1, action)...)
	}
	return rows
}

func stringKeys(m map[string][]TaskInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ── TUI view ──────────────────────────────────────────────────────────────────

type actionState int

const (
	actionPending actionState = iota
	actionRunning
	actionOK
	actionFailed
	actionSkipped // task never ran because a dependency failed
)

type taskEntry struct {
	state     actionState
	elapsed   time.Duration
	err       error
	startedAt time.Time // set when state transitions to actionRunning
}

type actionUpdateMsg struct {
	taskName string
	state    actionState
	elapsed  time.Duration
	err      error
	logLine  string // if non-empty, a log line (no state change)
}

type actionAllDoneMsg struct{}
type actionTickMsg time.Time

type actionPrepareRetryMsg struct {
	resetNames   []string
	newMsgCh     <-chan actionUpdateMsg
	newDrainedCh chan struct{}
	newRetryCh   chan []string
	newCancel    context.CancelFunc
}

// ── Quick solve overlay ───────────────────────────────────────────────────────

type quickSolveStep int

const (
	qsStepMenu         quickSolveStep = iota
	qsStepConfirmReset                // waiting for Enter/Esc on reset confirm screen
	qsStepConfirmClean                // waiting for Enter/Esc on clean confirm screen
	qsStepRunning                     // async operation in progress
	qsStepResult                      // operation finished; press any key to dismiss
)

type quickSolveOverlay struct {
	step     quickSolveStep
	taskInfo TaskInfo
	result   string
	isErr    bool
}

type quickSolveDoneMsg struct{ err error }

type tuiActionsView struct {
	program         *tea.Program
	msgCh           chan actionUpdateMsg
	drainedCh       chan struct{}
	postDoneRetryCh chan []string // model writes retry names here when allDone=true
	doneCh          chan struct{} // closed when p.Run() returns (program fully exited)
}

func newTUIActionsView(tasks []TaskInfo, actionOrder []string, cancel context.CancelFunc, retryCh chan []string, gitCleanExcludes []string) *tuiActionsView {
	msgCh := make(chan actionUpdateMsg, 1000)
	drainedCh := make(chan struct{})
	postDoneRetryCh := make(chan []string, 1)
	rows := buildTreeRows(tasks, actionOrder)

	taskState := make(map[string]*taskEntry, len(tasks))
	taskInfoMap := make(map[string]TaskInfo, len(tasks))
	for _, t := range tasks {
		taskState[t.TaskName] = &taskEntry{state: actionPending}
		taskInfoMap[t.TaskName] = t
	}

	// Build action→tasks index.
	tasksByAction := make(map[string][]string)
	for _, t := range tasks {
		tasksByAction[t.Action] = append(tasksByAction[t.Action], t.TaskName)
	}

	m := &actionsModel{
		rows:             rows,
		cursorTask:       "", // no selection until user navigates
		taskState:        taskState,
		taskInfoMap:      taskInfoMap,
		taskLogs:         make(map[string][]string, len(tasks)),
		actionOrder:      actionOrder,
		tasksByAction:    tasksByAction,
		collapsed:        make(map[string]bool),
		msgCh:            msgCh,
		drainedCh:        drainedCh,
		retryCh:          retryCh,
		postDoneRetryCh:  postDoneRetryCh,
		startTime:        time.Now(),
		cancel:           cancel,
		gitCleanExcludes: gitCleanExcludes,
	}
	m.updateCollapsed()

	p := tea.NewProgram(m)
	doneCh := make(chan struct{})
	v := &tuiActionsView{
		program:         p,
		msgCh:           msgCh,
		drainedCh:       drainedCh,
		postDoneRetryCh: postDoneRetryCh,
		doneCh:          doneCh,
	}

	go func() {
		_, _ = p.Run()
		close(doneCh)
	}()

	return v
}

func (v *tuiActionsView) OnStart(name string) {
	v.msgCh <- actionUpdateMsg{taskName: name, state: actionRunning}
}

func (v *tuiActionsView) OnReset(name string) {
	v.msgCh <- actionUpdateMsg{taskName: name, state: actionPending}
}

func (v *tuiActionsView) OnFinish(name string, err error, elapsed time.Duration) {
	s := actionOK
	if err != nil {
		if errors.Is(err, executor.ErrTaskSkipped) {
			s = actionSkipped
		} else {
			s = actionFailed
		}
	}
	v.msgCh <- actionUpdateMsg{taskName: name, state: s, elapsed: elapsed, err: err}
}

func (v *tuiActionsView) Handler() executor.OutputHandler {
	return &tuiActionsHandler{msgCh: v.msgCh}
}

// WaitAndGetRetry closes the update channel and waits for the user to either quit
// or request a post-run retry. Returns nil on quit, or task names to retry.
// When names are returned the TUI is still alive; call PrepareRetry before the next run.
func (v *tuiActionsView) WaitAndGetRetry() []string {
	close(v.msgCh)
	// Wait for model to drain messages, or program to quit first.
	select {
	case <-v.drainedCh:
	case <-v.doneCh:
		return nil
	}
	select {
	case names := <-v.postDoneRetryCh:
		return names // caller must call PrepareRetry then start a new executor
	case <-v.doneCh:
		return nil
	}
}

// PrepareRetry resets the named tasks to pending in the live TUI and wires up
// a new message channel and retry channel for the next executor run.
func (v *tuiActionsView) PrepareRetry(taskNames []string, retryCh chan []string, cancel context.CancelFunc) {
	newMsgCh := make(chan actionUpdateMsg, 1000)
	newDrainedCh := make(chan struct{})
	v.msgCh = newMsgCh
	v.drainedCh = newDrainedCh
	v.program.Send(actionPrepareRetryMsg{
		resetNames:   taskNames,
		newMsgCh:     newMsgCh,
		newDrainedCh: newDrainedCh,
		newRetryCh:   retryCh,
		newCancel:    cancel,
	})
}

type tuiActionsHandler struct {
	msgCh chan<- actionUpdateMsg
}

func (h *tuiActionsHandler) HandleOutput(_ context.Context, out executor.Output) error {
	line := strings.TrimRight(string(out.Output), "\n\r")
	if line == "" {
		return nil
	}
	h.msgCh <- actionUpdateMsg{taskName: out.CmdName, logLine: line}
	return nil
}

// ── BubbleTea model ───────────────────────────────────────────────────────────

const (
	maxAllLogs  = 500
	maxTaskLogs = 1000
)

type actionsModel struct {
	rows       []treeRow
	cursorTask string // task name the cursor is on
	treeOffset int    // viewport scroll offset (into visibleRows)

	taskState   map[string]*taskEntry
	taskInfoMap map[string]TaskInfo // for quick solve lookup
	taskLogs    map[string][]string // per-task log lines
	allLogs     []string            // interleaved (capped at maxAllLogs)
	focusTask   string              // if set, right pane shows only this task's logs

	actionOrder   []string
	tasksByAction map[string][]string
	collapsed     map[string]bool // collapsed[action] = true → only header row shown

	msgCh           <-chan actionUpdateMsg
	drainedCh       chan struct{}
	drainOnce       sync.Once
	retryCh         chan []string   // send retry task names here when Execute is running (inline retry)
	postDoneRetryCh chan<- []string // send retry task names here when allDone=true (new run)

	allDone    bool
	cancelling bool // context was cancelled; waiting for tasks to stop
	cancel     context.CancelFunc

	width, height int
	spinTick      int
	startTime     time.Time
	endTime       time.Time // set when all tasks finish; zero while running

	clipboardMsg       string    // brief toast shown in help bar after copy
	clipboardMsgExpiry time.Time // when to clear the toast
	clipboardOK        bool      // true = success toast, false = error toast

	gitCleanExcludes []string
	quickSolve       *quickSolveOverlay
}

// updateCollapsed recalculates which action sections are collapsed.
//
// Only one action section is expanded at a time: the "current" one.
// Current = the action with running tasks, or (when nothing runs) the last
// action that has made any non-skipped progress.
// Actions with real failures are always kept expanded as well.
// All-skipped actions (deps failed, task never ran) are always collapsed.
func (m *actionsModel) updateCollapsed() {
	type actionFlags struct {
		hasRunning bool
		notStarted bool // all tasks still pending
		hasFailed  bool // at least one real (non-skip) failure
		allSkipped bool // every task was skipped due to dep failure
	}

	flags := make(map[string]actionFlags, len(m.actionOrder))
	anyRunning := false

	for _, action := range m.actionOrder {
		taskNames := m.tasksByAction[action]
		f := actionFlags{
			notStarted: len(taskNames) > 0,
			allSkipped: len(taskNames) > 0,
		}
		for _, name := range taskNames {
			e := m.taskState[name]
			if e == nil {
				f.allSkipped = false
				continue
			}
			switch e.state {
			case actionRunning:
				f.hasRunning = true
				anyRunning = true
				f.notStarted = false
				f.allSkipped = false
			case actionOK:
				f.notStarted = false
				f.allSkipped = false
			case actionFailed:
				f.hasFailed = true
				f.notStarted = false
				f.allSkipped = false
			case actionSkipped:
				f.notStarted = false
				// allSkipped stays true
			default: // actionPending — not started yet
				f.allSkipped = false
			}
		}
		flags[action] = f
	}

	// The "current" action:
	//   - while running: the action that has running tasks
	//   - while stopped: the last action that has made non-skipped progress
	// Falls back to the first action when nothing has started yet.
	currentAction := ""
	if anyRunning {
		for _, a := range m.actionOrder {
			if flags[a].hasRunning {
				currentAction = a
				break
			}
		}
	} else {
		for _, a := range m.actionOrder {
			f := flags[a]
			if !f.allSkipped && !f.notStarted {
				currentAction = a // keep updating to find the last one
			}
		}
	}
	if currentAction == "" && len(m.actionOrder) > 0 {
		currentAction = m.actionOrder[0] // nothing started yet: show first
	}

	for _, a := range m.actionOrder {
		f := flags[a]
		switch {
		case f.allSkipped:
			m.collapsed[a] = true
		case f.hasFailed || a == currentAction:
			m.collapsed[a] = false
		default:
			m.collapsed[a] = true
		}
	}

	m.ensureCursorVisible()
}

// visibleRows returns the rows that should be shown (honouring collapsed state).
func (m *actionsModel) visibleRows() []treeRow {
	visible := make([]treeRow, 0, len(m.rows))
	for _, row := range m.rows {
		if row.kind != rowKindAction && m.collapsed[row.action] {
			continue
		}
		visible = append(visible, row)
	}
	return visible
}

// cursorTaskIndex returns the index of the cursor task in visible, or -1 if unset/not found.
func (m *actionsModel) cursorTaskIndex(visible []treeRow) int {
	if m.cursorTask == "" {
		return -1
	}
	for i, row := range visible {
		if row.kind == rowKindTask && row.taskName == m.cursorTask {
			return i
		}
	}
	return -1
}

func (m *actionsModel) ensureCursorVisible() {
	if m.cursorTask == "" {
		return // no selection, nothing to fix
	}
	visible := m.visibleRows()
	for _, row := range visible {
		if row.kind == rowKindTask && row.taskName == m.cursorTask {
			return // still visible
		}
	}
	// Selected task collapsed away — move to first visible task.
	for _, row := range visible {
		if row.kind == rowKindTask {
			m.cursorTask = row.taskName
			m.updateViewport(visible)
			return
		}
	}
}

func (m *actionsModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return actionTickMsg(t) }),
		m.listen(),
	)
}

func (m *actionsModel) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			m.drainOnce.Do(func() { close(m.drainedCh) })
			return actionAllDoneMsg{}
		}
		return msg
	}
}

func (m *actionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case actionTickMsg:
		m.spinTick++
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return actionTickMsg(t) })

	case actionUpdateMsg:
		if msg.logLine != "" {
			m.allLogs = append(m.allLogs, fmt.Sprintf("[%s] %s", msg.taskName, msg.logLine))
			if len(m.allLogs) > maxAllLogs {
				m.allLogs = m.allLogs[len(m.allLogs)-maxAllLogs:]
			}
			tl := append(m.taskLogs[msg.taskName], msg.logLine)
			if len(tl) > maxTaskLogs {
				tl = tl[len(tl)-maxTaskLogs:]
			}
			m.taskLogs[msg.taskName] = tl
		} else if e, ok := m.taskState[msg.taskName]; ok {
			if msg.state == actionRunning && e.state != actionRunning {
				e.startedAt = time.Now()
			}
			if msg.state == actionPending {
				// Task was re-queued for retry: reset all state.
				*e = taskEntry{state: actionPending}
				delete(m.taskLogs, msg.taskName)
			} else {
				e.state = msg.state
				e.elapsed = msg.elapsed
				e.err = msg.err
			}
			m.updateCollapsed()
		}
		return m, m.listen()

	case actionAllDoneMsg:
		m.allDone = true
		m.endTime = time.Now()
		if m.cancelling {
			return m, tea.Quit
		}
		return m, nil // stay alive; wait for user to press q/ctrl-c or retry

	case quickSolveDoneMsg:
		if m.quickSolve != nil {
			m.quickSolve.step = qsStepResult
			if msg.err != nil {
				m.quickSolve.result = msg.err.Error()
				m.quickSolve.isErr = true
			} else {
				m.quickSolve.result = "done — press Esc to close, then r to retry the task"
				m.quickSolve.isErr = false
			}
		}

	case actionPrepareRetryMsg:
		for _, name := range msg.resetNames {
			if e, ok := m.taskState[name]; ok {
				*e = taskEntry{state: actionPending}
			}
			delete(m.taskLogs, name)
		}
		m.msgCh = msg.newMsgCh
		m.drainedCh = msg.newDrainedCh
		m.drainOnce = sync.Once{}
		m.retryCh = msg.newRetryCh
		m.cancel = msg.newCancel
		m.allDone = false
		m.endTime = time.Time{}
		m.cancelling = false
		m.quickSolve = nil
		m.updateCollapsed()
		return m, m.listen()

	case tea.KeyMsg:
		// Route all key events to the quick solve overlay when it is active.
		if m.quickSolve != nil {
			return m.updateQuickSolve(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			if m.allDone || m.cancelling {
				// Already done or already cancelling: quit immediately.
				return m, tea.Quit
			}
			if m.cancel != nil {
				m.cancel()
			}
			m.cancelling = true
			return m, nil // wait for tasks to stop; second ctrl-c force-quits
		case "q":
			if m.allDone || m.cancelling {
				return m, tea.Quit
			}
			if m.cancel != nil {
				m.cancel()
			}
			m.cancelling = true
			return m, nil
		case "up", "k":
			m.movePrev()
		case "down", "j":
			m.moveNext()
		case "enter", "f":
			m.toggleFocus()
		case "c":
			if m.focusTask != "" {
				if err := copyToClipboard(context.Background(), strings.Join(m.taskLogs[m.focusTask], "\n")); err != nil {
					m.clipboardMsg = err.Error()
					m.clipboardOK = false
				} else {
					m.clipboardMsg = "copied to clipboard"
					m.clipboardOK = true
				}
				m.clipboardMsgExpiry = time.Now().Add(4 * time.Second)
			}
		case "r":
			if t := m.cursorTask; t != "" {
				if e, ok := m.taskState[t]; ok && e.state == actionFailed {
					m.sendRetry([]string{t})
				}
			}
		case "R":
			var failed []string
			for name, e := range m.taskState {
				if e.state == actionFailed {
					failed = append(failed, name)
				}
			}
			if len(failed) > 0 {
				sort.Strings(failed)
				m.sendRetry(failed)
			}
		case "s":
			if t := m.cursorTask; t != "" {
				if e, ok := m.taskState[t]; ok && e.state == actionFailed {
					if info, ok := m.taskInfoMap[t]; ok && info.ProjectDir != "" {
						m.quickSolve = &quickSolveOverlay{
							step:     qsStepMenu,
							taskInfo: info,
						}
					}
				}
			}
		}
	}
	return m, nil
}

// updateQuickSolve handles key events while the quick solve overlay is open.
func (m *actionsModel) updateQuickSolve(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	qs := m.quickSolve
	switch qs.step {
	case qsStepMenu:
		switch msg.String() {
		case "1":
			qs.step = qsStepConfirmReset
		case "2":
			qs.step = qsStepConfirmClean
		case "esc", "q", "s":
			m.quickSolve = nil
		}
	case qsStepConfirmReset:
		switch msg.String() {
		case "enter":
			qs.step = qsStepRunning
			return m, m.runQuickSolveReset(qs.taskInfo)
		case "esc", "q", "n":
			qs.step = qsStepMenu
		}
	case qsStepConfirmClean:
		switch msg.String() {
		case "enter":
			qs.step = qsStepRunning
			return m, m.runQuickSolveClean(qs.taskInfo)
		case "esc", "q", "n":
			qs.step = qsStepMenu
		}
	case qsStepRunning:
		// Ignore all keys while the operation is running.
	case qsStepResult:
		m.quickSolve = nil
	}
	return m, nil
}

func (m *actionsModel) runQuickSolveReset(info TaskInfo) tea.Cmd {
	return func() tea.Msg {
		return quickSolveDoneMsg{err: ResetToLatest(context.Background(), info.ProjectDir, info.DefaultBranch)}
	}
}

func (m *actionsModel) runQuickSolveClean(info TaskInfo) tea.Cmd {
	return func() tea.Msg {
		return quickSolveDoneMsg{err: GitCleanFdx(context.Background(), info.ProjectDir, m.gitCleanExcludes)}
	}
}

// sendRetry routes retry names to the right channel depending on whether Execute is still running.
func (m *actionsModel) sendRetry(names []string) {
	if m.allDone {
		// Execute has finished — signal runner.go to start a new run.
		select {
		case m.postDoneRetryCh <- names:
		default:
		}
	} else if m.retryCh != nil {
		// Execute is still running — re-queue inline.
		select {
		case m.retryCh <- names:
		default:
		}
	}
}

func (m *actionsModel) movePrev() {
	visible := m.visibleRows()
	curIdx := m.cursorTaskIndex(visible)
	for i := curIdx - 1; i >= 0; i-- {
		if visible[i].kind == rowKindTask {
			m.cursorTask = visible[i].taskName
			m.updateViewport(visible)
			return
		}
	}
}

func (m *actionsModel) moveNext() {
	visible := m.visibleRows()
	curIdx := m.cursorTaskIndex(visible)
	for i := curIdx + 1; i < len(visible); i++ {
		if visible[i].kind == rowKindTask {
			m.cursorTask = visible[i].taskName
			m.updateViewport(visible)
			return
		}
	}
}

func (m *actionsModel) toggleFocus() {
	if m.cursorTask == "" {
		return
	}
	if m.focusTask == m.cursorTask {
		m.focusTask = ""
	} else {
		m.focusTask = m.cursorTask
	}
}

func (m *actionsModel) updateViewport(visible []treeRow) {
	curIdx := m.cursorTaskIndex(visible)
	if curIdx < 0 {
		return // no selection, viewport stays put
	}
	avail := m.availH()
	if curIdx >= m.treeOffset+avail {
		m.treeOffset = curIdx - avail + 1
	}
	if curIdx < m.treeOffset {
		m.treeOffset = curIdx
	}
	if m.treeOffset < 0 {
		m.treeOffset = 0
	}
	maxOffset := len(visible) - avail
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.treeOffset > maxOffset {
		m.treeOffset = maxOffset
	}
}

func (m *actionsModel) availH() int {
	h := m.height - 4 // header + top-sep + bot-sep + help
	if h < 5 {
		return 5
	}
	return h
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	actHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	actSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	actHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	actActionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	actHostStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	actNSStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	actPendStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	actRunStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	actOKStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	actFailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	actSelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	actCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	actFocusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	actLogStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	actDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Quick solve overlay styles.
	qsTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	qsWarnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	qsOptStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	qsDirStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	qsSuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
)

var actSpinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *actionsModel) View() string {
	if m.width == 0 {
		return "Loading...\n"
	}
	if m.quickSolve != nil {
		return m.renderQuickSolveView()
	}

	leftW := 48
	if m.width < 80 {
		leftW = m.width / 2
	}
	logW := m.width - leftW - 3 // 3 = len(" │ ")
	if logW < 10 {
		logW = 10
	}
	avail := m.availH()

	// Counts for header
	running, success, failed, skipped := 0, 0, 0, 0
	for _, e := range m.taskState {
		switch e.state {
		case actionPending:
			// not yet started, not counted
		case actionRunning:
			running++
		case actionOK:
			success++
		case actionFailed:
			failed++
		case actionSkipped:
			skipped++
		}
	}
	total := len(m.taskState)
	end := m.endTime
	if end.IsZero() {
		end = time.Now()
	}
	elapsed := end.Sub(m.startTime)
	hdr := fmt.Sprintf("Actions  running:%d  done:%d/%d  failed:%d",
		running, success+failed, total-skipped, failed)
	if skipped > 0 {
		hdr += fmt.Sprintf("  skipped:%d", skipped)
	}
	hdr += "  " + fmtActionDuration(elapsed)

	topSep := strings.Repeat("─", leftW) + "─┬─" + strings.Repeat("─", logW)
	botSep := strings.Repeat("─", leftW) + "─┴─" + strings.Repeat("─", logW)

	var b strings.Builder
	b.WriteString(actHeaderStyle.Render(hdr) + "\n")
	b.WriteString(actSepStyle.Render(topSep) + "\n")

	treeLines := m.renderTree(leftW, avail)
	logLines := m.renderLogs(logW, avail)

	maxLines := len(treeLines)
	if len(logLines) > maxLines {
		maxLines = len(logLines)
	}
	if maxLines > avail {
		maxLines = avail
	}

	for i := 0; i < maxLines; i++ {
		l, r := "", ""
		if i < len(treeLines) {
			l = treeLines[i]
		}
		if i < len(logLines) {
			r = logLines[i]
		}
		b.WriteString(padToVisualWidth(l, leftW) + actSepStyle.Render(" │ ") + r + "\n")
	}

	b.WriteString(actSepStyle.Render(botSep) + "\n")

	// Help line
	var parts []string
	parts = append(parts, "↑↓/jk: nav")
	if m.focusTask != "" {
		parts = append(parts, "Enter/f: all logs")
		parts = append(parts, "c: copy logs")
	} else {
		parts = append(parts, "Enter/f: focus logs")
	}
	if failed > 0 {
		if e, ok := m.taskState[m.cursorTask]; ok && e.state == actionFailed {
			parts = append(parts, "r: retry selected")
			if info, ok := m.taskInfoMap[m.cursorTask]; ok && info.ProjectDir != "" {
				parts = append(parts, "s: quick solve")
			}
		}
		parts = append(parts, "R: retry all failed")
	}
	switch {
	case m.cancelling && !m.allDone:
		parts = append(parts, "Ctrl-C/q: force quit")
	default:
		parts = append(parts, "q: quit")
	}
	if m.clipboardMsg != "" && time.Now().Before(m.clipboardMsgExpiry) {
		if m.clipboardOK {
			parts = append(parts, actOKStyle.Render("✓ "+m.clipboardMsg))
		} else {
			parts = append(parts, actFailStyle.Render("✗ "+m.clipboardMsg))
		}
	}
	b.WriteString(actHelpStyle.Render(strings.Join(parts, "  ")))

	return b.String()
}

// renderQuickSolveView renders the full screen with the quick solve modal overlaying the body.
func (m *actionsModel) renderQuickSolveView() string {
	qs := m.quickSolve
	var b strings.Builder

	// Header
	b.WriteString(qsTitleStyle.Render("⚡ Quick Solve — "+qs.taskInfo.TaskName) + "\n")
	sep := strings.Repeat("─", m.width)
	b.WriteString(actSepStyle.Render(sep) + "\n")

	avail := m.availH()
	var lines []string

	switch qs.step {
	case qsStepMenu:
		lines = append(lines,
			"",
			qsOptStyle.Render("  1  Reset to latest default branch"),
			actDimStyle.Render("     Fetch from origin and reset to origin/"+qs.taskInfo.DefaultBranch),
			"",
			qsOptStyle.Render("  2  Git clean -fdx"),
			actDimStyle.Render("     Remove all untracked files and directories"),
			"",
			actDimStyle.Render("  Project: "+qs.taskInfo.ProjectDir),
		)
	case qsStepConfirmReset:
		branch := qs.taskInfo.DefaultBranch
		lines = append(lines,
			"",
			qsWarnStyle.Render("  ⚠  DESTRUCTIVE OPERATION — CANNOT BE UNDONE"),
			"",
			"  This will run:",
			actDimStyle.Render("    git fetch origin "+branch),
			actDimStyle.Render("    git reset --hard origin/"+branch),
			"",
			qsWarnStyle.Render("  ⚠  ALL LOCAL CHANGES IN THIS PROJECT WILL BE PERMANENTLY DESTROYED."),
			qsWarnStyle.Render("  ⚠  Uncommitted code, staged changes, and modified files — gone forever."),
			qsWarnStyle.Render("  ⚠  This action cannot be undone. There is no recovery."),
			"",
			qsDirStyle.Render("  Project: "+qs.taskInfo.ProjectDir),
			"",
			"  Press "+qsWarnStyle.Render("Enter")+" to confirm, Esc to go back.",
		)
	case qsStepConfirmClean:
		excl := strings.Join(m.gitCleanExcludes, ", ")
		if excl == "" {
			excl = "(none)"
		}
		lines = append(lines,
			"",
			qsWarnStyle.Render("  ⚠  DESTRUCTIVE OPERATION — CANNOT BE UNDONE"),
			"",
			"  This will run:",
			actDimStyle.Render("    git clean -fdx (with exclusions: "+excl+")"),
			"",
			qsWarnStyle.Render("  ⚠  ALL UNTRACKED FILES AND DIRECTORIES WILL BE PERMANENTLY DELETED."),
			qsWarnStyle.Render("  ⚠  Files are NOT moved to trash — they are gone forever."),
			qsWarnStyle.Render("  ⚠  This action cannot be undone. There is no recovery."),
			"",
			qsDirStyle.Render("  Project: "+qs.taskInfo.ProjectDir),
			"",
			"  Press "+qsWarnStyle.Render("Enter")+" to confirm, Esc to go back.",
		)
	case qsStepRunning:
		lines = append(lines, "", "  Running…")
	case qsStepResult:
		if qs.isErr {
			lines = append(lines,
				"",
				actFailStyle.Render("  ✗ Failed:"),
				actFailStyle.Render("    "+qs.result),
				"",
				actDimStyle.Render("  Press any key to close."),
			)
		} else {
			lines = append(lines,
				"",
				qsSuccessStyle.Render("  ✓ "+qs.result),
				"",
				actDimStyle.Render("  Press any key to close."),
			)
		}
	}

	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	for i := len(lines); i < avail; i++ {
		b.WriteString("\n")
	}

	b.WriteString(actSepStyle.Render(sep) + "\n")

	// Help line
	var help string
	switch qs.step {
	case qsStepMenu:
		help = "1/2: choose action  Esc/s: cancel"
	case qsStepConfirmReset, qsStepConfirmClean:
		help = "Enter: confirm  Esc: go back"
	case qsStepRunning:
		help = "running…"
	case qsStepResult:
		help = "any key: close"
	}
	b.WriteString(actHelpStyle.Render(help))

	return b.String()
}

func (m *actionsModel) renderTree(width, height int) []string {
	visible := m.visibleRows()
	start := m.treeOffset
	end := start + height
	if end > len(visible) {
		end = len(visible)
	}

	var lines []string
	for i := start; i < end; i++ {
		row := visible[i]
		indent := strings.Repeat("  ", row.depth)
		isSelected := row.kind == rowKindTask && row.taskName == m.cursorTask

		var line string
		switch row.kind {
		case rowKindAction:
			if m.collapsed[row.action] {
				line = actActionStyle.Render(indent+row.label) + actDimStyle.Render(m.actionSummary(row.action))
			} else {
				line = actActionStyle.Render(indent + row.label)
			}
		case rowKindHost:
			line = actHostStyle.Render(indent + row.label)
		case rowKindNamespace, rowKindProjectHeader:
			line = actNSStyle.Render(indent + row.label)
		case rowKindTask:
			cursor := "  "
			if isSelected {
				cursor = actCursorStyle.Render("> ")
			}
			e := m.taskState[row.taskName]
			icon, iconSty := m.taskIconStyle(e)

			label := row.label
			if m.focusTask == row.taskName {
				label += " " + actFocusStyle.Render("[focused]")
			}

			var labelSty lipgloss.Style
			if isSelected {
				labelSty = actSelStyle
			} else if e != nil {
				switch e.state {
				case actionOK:
					labelSty = actOKStyle
				case actionFailed:
					labelSty = actFailStyle
				default:
					labelSty = lipgloss.NewStyle()
				}
			} else {
				labelSty = lipgloss.NewStyle()
			}

			extra := ""
			if e != nil {
				switch e.state {
				case actionPending, actionSkipped:
					// no extra text
				case actionRunning:
					if !e.startedAt.IsZero() {
						extra = actDimStyle.Render("  " + fmtActionDuration(time.Since(e.startedAt)))
					}
				case actionOK:
					extra = actDimStyle.Render("  " + fmtActionDuration(e.elapsed))
				case actionFailed:
					msg := "  FAILED"
					if e.err != nil {
						short := e.err.Error()
						if len([]rune(short)) > 28 {
							short = string([]rune(short)[:28]) + "…"
						}
						msg += ": " + short
					}
					extra = actFailStyle.Render(msg)
				}
			}

			line = indent + cursor + iconSty.Render(icon+" ") + labelSty.Render(label) + extra
		}
		lines = append(lines, truncateANSILine(line, width))
	}
	return lines
}

// actionSummary returns a short compact status for a collapsed action header.
func (m *actionsModel) actionSummary(action string) string {
	taskNames := m.tasksByAction[action]
	done, failed, running := 0, 0, 0
	for _, name := range taskNames {
		e := m.taskState[name]
		if e == nil {
			continue
		}
		switch e.state {
		case actionPending, actionSkipped:
			// not counted in done/failed/running
		case actionOK:
			done++
		case actionFailed:
			failed++
		case actionRunning:
			running++
		}
	}
	total := len(taskNames)
	fin := done + failed
	if failed > 0 {
		return fmt.Sprintf("  [%d/%d, %d ✗]", fin, total, failed)
	}
	if fin == total {
		return fmt.Sprintf("  [%d/%d ✓]", fin, total)
	}
	if running > 0 {
		return fmt.Sprintf("  [%d running, %d/%d done]", running, fin, total)
	}
	return fmt.Sprintf("  [%d/%d]", fin, total)
}

func (m *actionsModel) taskIconStyle(e *taskEntry) (string, lipgloss.Style) {
	if e == nil {
		return "○", actPendStyle
	}
	switch e.state {
	case actionPending:
		return "○", actPendStyle
	case actionRunning:
		return actSpinFrames[m.spinTick%len(actSpinFrames)], actRunStyle
	case actionOK:
		return "✓", actOKStyle
	case actionFailed:
		return "✗", actFailStyle
	case actionSkipped:
		return "─", actDimStyle
	}
	return "○", actPendStyle
}

func (m *actionsModel) renderLogs(width, height int) []string {
	var src []string
	var header string

	if m.focusTask != "" {
		header = "● " + m.focusTask
		src = m.taskLogs[m.focusTask]
	} else {
		src = m.allLogs
	}

	var lines []string
	if header != "" {
		lines = append(lines, actFocusStyle.Render(header))
		height--
	}

	start := 0
	if len(src) > height {
		start = len(src) - height
	}
	for _, l := range src[start:] {
		l = stripActANSI(l) // strip ANSI from process output before display
		r := []rune(l)
		if len(r) > width {
			l = string(r[:width-1]) + "…"
		}
		lines = append(lines, actLogStyle.Render(l))
	}
	return lines
}

// truncateANSILine truncates s to maxWidth visible characters, preserving ANSI
// escape sequences and appending a reset code if truncation occurred.
func truncateANSILine(s string, maxWidth int) string {
	var result strings.Builder
	visWidth := 0
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
		if visWidth >= maxWidth {
			result.WriteString("\x1b[0m")
			return result.String()
		}
		result.WriteRune(r)
		visWidth++
	}
	return result.String()
}

// padToVisualWidth pads s to the given visual width (ignoring ANSI escape codes).
func padToVisualWidth(s string, width int) string {
	vis := len([]rune(stripActANSI(s)))
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func stripActANSI(s string) string {
	var b strings.Builder
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
		b.WriteRune(r)
	}
	return b.String()
}

// copyToClipboard writes text to the system clipboard.
// Tries stdin-based tools first, then KDE klipper via qdbus.
func copyToClipboard(ctx context.Context, text string) error {
	// Tools that read from stdin.
	stdinTools := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	}
	var runErr error
	for _, args := range stdinTools {
		cmd, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}
		c := exec.CommandContext(ctx, cmd, args[1:]...) //nolint:gosec
		c.Stdin = strings.NewReader(text)
		if err := c.Run(); err == nil {
			return nil
		} else {
			runErr = fmt.Errorf("%s: %w", args[0], err)
		}
	}

	// KDE klipper via qdbus (available on KDE Plasma without extra packages).
	if qdbus, err := exec.LookPath("qdbus"); err == nil {
		c := exec.CommandContext(ctx, qdbus, "org.kde.klipper", "/klipper", //nolint:gosec
			"org.kde.klipper.klipper.setClipboardContents", text)
		if err := c.Run(); err == nil {
			return nil
		} else {
			runErr = fmt.Errorf("qdbus klipper: %w", err)
		}
	}

	if runErr != nil {
		return runErr
	}
	return fmt.Errorf("no clipboard tool found — install wl-clipboard: apt install wl-clipboard")
}

func fmtActionDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
