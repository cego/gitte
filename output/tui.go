package output

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskState holds the display state of a single task
type TaskState struct {
	Name      string
	Status    TaskStateStatus
	StartTime time.Time
	EndTime   time.Time
	Lines     []string // accumulated log lines
}

// TaskStateStatus represents the current state of a task in the TUI
type TaskStateStatus int

const (
	TaskPending TaskStateStatus = iota
	TaskRunning
	TaskSuccess
	TaskFailed
)

func (s TaskStateStatus) Icon() string {
	switch s {
	case TaskPending:
		return "○"
	case TaskRunning:
		return "●"
	case TaskSuccess:
		return "✓"
	case TaskFailed:
		return "✗"
	}
	return "?"
}

// TUIMsg is sent to the BubbleTea program to update task state
type TUIMsg struct {
	TaskName string
	Status   TaskStateStatus
	Line     string // if non-empty, a log line to append
}

type runModel struct {
	tasks       []*TaskState
	taskIndex   map[string]int
	cursor      int
	focusedTask string // if set, show only this task's logs
	msgCh       <-chan TUIMsg
	mu          sync.Mutex
	width       int
	height      int
	startTime   time.Time
	done        bool
	ticker      *time.Ticker
}

type tickMsg time.Time
type tuiUpdateMsg TUIMsg

func newRunModel(taskNames []string, msgCh <-chan TUIMsg) *runModel {
	tasks := make([]*TaskState, len(taskNames))
	idx := make(map[string]int, len(taskNames))
	for i, name := range taskNames {
		tasks[i] = &TaskState{Name: name, Status: TaskPending}
		idx[name] = i
	}
	return &runModel{
		tasks:     tasks,
		taskIndex: idx,
		msgCh:     msgCh,
		startTime: time.Now(),
	}
}

func (m *runModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.listenForUpdates(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *runModel) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			return nil
		}
		return tuiUpdateMsg(msg)
	}
}

func (m *runModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		if m.done {
			return m, nil
		}
		return m, tea.Batch(tickCmd(), m.listenForUpdates())

	case tuiUpdateMsg:
		m.applyUpdate(TUIMsg(msg))
		if !m.done {
			return m, m.listenForUpdates()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "f":
			if m.focusedTask == "" && m.cursor < len(m.tasks) {
				m.focusedTask = m.tasks[m.cursor].Name
			} else {
				m.focusedTask = ""
			}
		}
	}

	return m, nil
}

func (m *runModel) applyUpdate(msg TUIMsg) {
	idx, ok := m.taskIndex[msg.TaskName]
	if !ok {
		return
	}
	task := m.tasks[idx]

	if msg.Status != 0 || (msg.Status == 0 && msg.Line == "") {
		task.Status = msg.Status
		switch msg.Status {
		case TaskPending:
			// no time to record
		case TaskRunning:
			task.StartTime = time.Now()
		case TaskSuccess, TaskFailed:
			task.EndTime = time.Now()
		}
	}

	if msg.Line != "" {
		task.Lines = append(task.Lines, strings.TrimRight(msg.Line, "\n\r"))
	}

	// Check if all tasks are done
	allDone := true
	for _, t := range m.tasks {
		if t.Status == TaskPending || t.Status == TaskRunning {
			allDone = false
			break
		}
	}
	m.done = allDone
}

var (
	tuiPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	tuiRunningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	tuiSuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	tuiFailedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	tuiSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	tuiHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	tuiHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	tuiLogStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func (m *runModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header with elapsed time and counts
	running, success, failed, total := 0, 0, 0, len(m.tasks)
	for _, t := range m.tasks {
		switch t.Status {
		case TaskPending:
			// pending tasks not counted in running/success/failed
		case TaskRunning:
			running++
		case TaskSuccess:
			success++
		case TaskFailed:
			failed++
		}
	}
	elapsed := time.Since(m.startTime)
	header := fmt.Sprintf("Gitte | running: %d | done: %d/%d | failed: %d | %s",
		running, success+failed, total, failed, formatDuration(elapsed))
	b.WriteString(tuiHeaderStyle.Render(header))
	b.WriteString("\n\n")

	// Split layout: left = task list, right = logs
	listWidth := 35
	if m.width < 60 {
		listWidth = m.width / 2
	}
	logWidth := m.width - listWidth - 3

	// Task list
	taskListLines := m.renderTaskList(listWidth)
	// Log panel
	logLines := m.renderLogPanel(logWidth)

	// Merge side by side
	maxLines := max(len(taskListLines), len(logLines))
	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(taskListLines) {
			left = taskListLines[i]
		}
		right := ""
		if i < len(logLines) {
			right = logLines[i]
		}
		// Pad left to listWidth
		padded := padRight(left, listWidth)
		b.WriteString(padded + " │ " + right + "\n")
	}

	b.WriteString("\n")
	helpStr := "j/k: navigate | f: focus task logs | q: quit"
	if m.focusedTask != "" {
		helpStr = "f: back to interleaved | j/k: navigate | q: quit"
	}
	b.WriteString(tuiHelpStyle.Render(helpStr))

	return b.String()
}

func (m *runModel) renderTaskList(width int) []string {
	var lines []string
	for i, t := range m.tasks {
		var style lipgloss.Style
		switch t.Status {
		case TaskPending:
			style = tuiPendingStyle
		case TaskRunning:
			style = tuiRunningStyle
		case TaskSuccess:
			style = tuiSuccessStyle
		case TaskFailed:
			style = tuiFailedStyle
		}

		icon := style.Render(t.Status.Icon())
		name := t.Name
		if len(name) > width-5 {
			name = name[:width-5] + "..."
		}

		line := fmt.Sprintf("%s %s", icon, name)
		if i == m.cursor {
			line = tuiSelectedStyle.Render("> " + name[:min(len(name), width-4)])
		}
		lines = append(lines, line)
	}
	return lines
}

func (m *runModel) renderLogPanel(width int) []string {
	// Available height for logs (minus header, footer)
	maxLogLines := m.height - 6
	if maxLogLines < 5 {
		maxLogLines = 5
	}

	var allLines []string
	if m.focusedTask != "" {
		// Show only focused task's logs
		if idx, ok := m.taskIndex[m.focusedTask]; ok {
			t := m.tasks[idx]
			for _, line := range t.Lines {
				allLines = append(allLines, tuiLogStyle.Render(line))
			}
		}
	} else {
		// Interleaved logs with task prefix
		for _, t := range m.tasks {
			for _, line := range t.Lines {
				prefix := tuiRunningStyle.Render("[" + t.Name + "] ")
				allLines = append(allLines, prefix+tuiLogStyle.Render(line))
			}
		}
	}

	// Show last maxLogLines lines (auto-scroll to bottom)
	if len(allLines) > maxLogLines {
		allLines = allLines[len(allLines)-maxLogLines:]
	}

	return allLines
}

func padRight(s string, width int) string {
	// Strip ANSI for length calculation (simplified)
	visLen := len(stripANSI(s))
	if visLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visLen)
}

func stripANSI(s string) string {
	// Simplified ANSI stripper
	result := ""
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
		result += string(r)
	}
	return result
}

