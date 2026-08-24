package telemetry

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ActionTracker opens one span per action (e.g. "build", "up") under the
// actions phase context, driven by the executor's task hooks. Action spans are
// closed explicitly by the action runner after all executor passes finish.
type ActionTracker struct {
	phaseCtx context.Context
	mu       sync.Mutex
	spans    map[string]trace.Span      // action -> span
	ctxs     map[string]context.Context // action -> span context
	started  map[string]struct{}        // tasks that have called OnStart
}

// NewActionTracker creates a tracker rooted at the actions phase context.
func NewActionTracker(phaseCtx context.Context) *ActionTracker {
	return &ActionTracker{
		phaseCtx: phaseCtx,
		spans:    map[string]trace.Span{},
		ctxs:     map[string]context.Context{},
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

// OnStart opens the action span if needed and records that the task ran.
func (t *ActionTracker) OnStart(taskName string) {
	action := ActionOf(taskName)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.openLocked(action)
	t.started[taskName] = struct{}{}
}

// OnFinish records a task error on its action span. If the task never called
// OnStart (e.g. it was skipped due to a failed dependency), this is a no-op.
func (t *ActionTracker) OnFinish(taskName string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.started[taskName]; !ok {
		return // skipped task — no matching OnStart, nothing to do
	}
	delete(t.started, taskName)
	action := ActionOf(taskName)
	// Propagate a failed task onto its action span so failure shows at every
	// level of the trace, not just on the task span.
	if err != nil {
		if span, ok := t.spans[action]; ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}
}

// Close ends every action span exactly once. It is safe to call more than once.
func (t *ActionTracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for action, span := range t.spans {
		span.End()
		delete(t.spans, action)
		delete(t.ctxs, action)
	}
}

func (t *ActionTracker) openLocked(action string) {
	if _, ok := t.spans[action]; ok {
		return
	}
	ctx, span := Tracer().Start(t.phaseCtx, action)
	t.spans[action] = span
	t.ctxs[action] = ctx
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
