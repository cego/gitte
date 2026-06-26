package telemetry

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

// ActionTracker opens one span per action (e.g. "build", "up") under the
// actions phase context, driven by the executor's task hooks. An action span
// opens on its first task start and closes when its last task finishes.
type ActionTracker struct {
	phaseCtx context.Context
	mu       sync.Mutex
	spans    map[string]trace.Span      // action -> span
	ctxs     map[string]context.Context // action -> span context
	active   map[string]int             // action -> live task count
	started  map[string]struct{}        // tasks that have called OnStart
}

// NewActionTracker creates a tracker rooted at the actions phase context.
func NewActionTracker(phaseCtx context.Context) *ActionTracker {
	return &ActionTracker{
		phaseCtx: phaseCtx,
		spans:    map[string]trace.Span{},
		ctxs:     map[string]context.Context{},
		active:   map[string]int{},
		started:  map[string]struct{}{},
	}
}

// ActionOf extracts the action name from a "project:action:group" task name.
func ActionOf(taskName string) string {
	parts := strings.Split(taskName, ":")
	if len(parts) >= 2 {
		return parts[1] // project:action:group
	}
	return taskName
}

// OnStart opens the action span if needed and increments its live-task count.
func (t *ActionTracker) OnStart(taskName string) {
	action := ActionOf(taskName)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.spans[action]; !ok {
		ctx, span := Tracer().Start(t.phaseCtx, action)
		t.spans[action] = span
		t.ctxs[action] = ctx
	}
	t.started[taskName] = struct{}{}
	t.active[action]++
}

// OnFinish decrements the live-task count and ends the action span at zero.
// If the task never called OnStart (e.g. it was skipped due to a failed
// dependency), this is a no-op to avoid corrupting the active count.
func (t *ActionTracker) OnFinish(taskName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.started[taskName]; !ok {
		return // skipped task — no matching OnStart, nothing to do
	}
	delete(t.started, taskName)
	action := ActionOf(taskName)
	t.active[action]--
	if t.active[action] <= 0 {
		if span, ok := t.spans[action]; ok {
			span.End()
			delete(t.spans, action)
			delete(t.ctxs, action)
			delete(t.active, action)
		}
	}
}

// ActionContext returns the action span's context, or the phase context if the
// action span is not open.
func (t *ActionTracker) ActionContext(action string) context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx, ok := t.ctxs[action]; ok {
		return ctx
	}
	return t.phaseCtx
}
