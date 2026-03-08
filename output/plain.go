package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// PlainWriter writes structured plain-text output for non-TTY environments
type PlainWriter struct {
	mu     sync.Mutex
	out    io.Writer
	starts map[string]time.Time
}

// NewPlainWriter creates a PlainWriter that writes to stdout
func NewPlainWriter() *PlainWriter {
	return &PlainWriter{
		out:    os.Stdout,
		starts: make(map[string]time.Time),
	}
}

// TaskStarted prints a RUNNING line for the given task
func (w *PlainWriter) TaskStarted(prefix, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.starts[prefix+":"+name] = time.Now()
	fmt.Fprintf(w.out, "[%s:%s] RUNNING\n", prefix, name)
}

// TaskLine prints a log line for the given task
func (w *PlainWriter) TaskLine(prefix, name, line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	line = strings.TrimRight(line, "\n\r")
	if line != "" {
		fmt.Fprintf(w.out, "[%s:%s] %s\n", prefix, name, line)
	}
}

// TaskDone prints a SUCCESS line with elapsed time
func (w *PlainWriter) TaskDone(prefix, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	elapsed := w.elapsed(prefix, name)
	fmt.Fprintf(w.out, "[%s:%s] SUCCESS (%s)\n", prefix, name, elapsed)
}

// TaskFailed prints a FAILED line with optional retry info
func (w *PlainWriter) TaskFailed(prefix, name string, attempt, maxAttempts int, retryIn string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if attempt < maxAttempts {
		fmt.Fprintf(w.out, "[ERROR] %s:%s FAILED (attempt %d/%d, retrying in %s)\n",
			prefix, name, attempt, maxAttempts, retryIn)
	} else {
		fmt.Fprintf(w.out, "[ERROR] %s:%s FAILED (attempt %d/%d, no more retries)\n",
			prefix, name, attempt, maxAttempts)
	}
}

// Summary prints the final run summary
func (w *PlainWriter) Summary(succeeded, failed int, elapsed time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.out, "[INFO] Run complete: %d succeeded, %d failed (%s)\n",
		succeeded, failed, formatDuration(elapsed))
}

func (w *PlainWriter) elapsed(prefix, name string) string {
	key := prefix + ":" + name
	if start, ok := w.starts[key]; ok {
		return formatDuration(time.Since(start))
	}
	return "?"
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}
