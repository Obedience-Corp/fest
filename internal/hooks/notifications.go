package hooks

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
)

// Notification definitions are local preferences, never shared gate bindings.
const (
	PresentationOpenName   = "presentation_open"
	ApprovalNotifyName     = "approval_notify"
	NotificationTimeout    = 5 * time.Second
	MaxNotificationTimeout = 30 * time.Second
)

// IsNotificationName identifies definitions excluded from ordinary hook resolution.
func IsNotificationName(name string) bool {
	return name == PresentationOpenName || name == ApprovalNotifyName
}

// ResolveNotification resolves only an explicitly enabled machine definition.
// Invalid optional notifications must never poison ordinary gate resolution.
func ResolveNotification(cfg *config.HooksConfig, name string) (ResolvedHook, bool, error) {
	if !IsNotificationName(name) || cfg == nil || (cfg.Enabled != nil && !*cfg.Enabled) {
		return ResolvedHook{}, false, nil
	}
	if on, set := cfg.Levels[string(LevelWorkflow)]; set && !on {
		return ResolvedHook{}, false, nil
	}
	def, ok := cfg.Definitions[name]
	if !ok || def.Enabled == nil || !*def.Enabled {
		return ResolvedHook{}, false, nil
	}
	if strings.TrimSpace(def.Command) == "" {
		return ResolvedHook{}, false, fmt.Errorf("%s: command is required", name)
	}
	if def.Fail != "" && def.Fail != string(FailOpen) {
		return ResolvedHook{}, false, fmt.Errorf("%s: notifications require fail=open", name)
	}
	timeout := NotificationTimeout
	if def.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(def.Timeout)
		if err != nil || timeout <= 0 || timeout > MaxNotificationTimeout {
			return ResolvedHook{}, false, fmt.Errorf("%s: timeout must be positive and at most %s", name, MaxNotificationTimeout)
		}
	}
	return ResolvedHook{Name: name, Command: def.Command, Fail: FailOpen, Timeout: timeout, Enabled: true, Source: LayerMachine}, true, nil
}

// Notification describes a committed workflow outcome, not a durable delivery job.
type Notification struct {
	Workflow string // internal workflow state key; no persistence or public schema change
	Name     string
	Verb     Verb
	Coord    Coord
}

type notificationKey struct{}

// Notifications is a command-scoped outbox. Producers only enqueue; the command
// owner drains after execution returns and every workflow/checkpoint lock is released.
// There is no replay, persistence, retry, or cross-process delivery guarantee.
type Notifications struct {
	mu      sync.Mutex
	pending []Notification
	closed  bool
}

// WithNotifications installs a fresh outbox for one command execution.
func WithNotifications(ctx context.Context) (context.Context, *Notifications) {
	q := &Notifications{}
	return context.WithValue(ctx, notificationKey{}, q), q
}

// QueueNotification does nothing outside a command-owned notification scope.
func QueueNotification(ctx context.Context, n Notification) {
	if ctx == nil {
		return
	}
	q, _ := ctx.Value(notificationKey{}).(*Notifications)
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.pending = append(q.pending, n)
	}
}

// Drain consumes the outbox once, even when delivery fails. It must be called
// outside all workflow/checkpoint locks, including on command error paths.
func (q *Notifications) Drain(ctx context.Context, deliver func(context.Context, Notification)) {
	q.mu.Lock()
	pending := q.pending
	q.pending = nil
	q.closed = true
	q.mu.Unlock()
	for _, n := range pending {
		if ctx.Err() != nil {
			return
		}
		deliver(ctx, n)
	}
}

// DeliverNotification uses local configuration only and never changes command status.
func DeliverNotification(ctx context.Context, n Notification, diagnostics io.Writer) {
	cfg, err := config.Load(ctx, "")
	if err != nil {
		_, _ = fmt.Fprintf(diagnostics, "Warning: workflow notification configuration: %s\n", stderrTail(err.Error()))
		return
	}
	if cfg == nil {
		return
	}
	deliverNotification(ctx, cfg.Hooks, n, diagnostics, newNotificationRunner)
}

func deliverNotification(ctx context.Context, cfg *config.HooksConfig, n Notification, diagnostics io.Writer, newRunner func(string) *Runner) {
	hook, enabled, err := ResolveNotification(cfg, n.Name)
	if err != nil {
		_, _ = fmt.Fprintf(diagnostics, "Warning: %s\n", err)
		return
	}
	if !enabled || ctx.Err() != nil {
		return
	}
	runner := newRunner(n.Coord.FestivalPath)
	runner.Coord = n.Coord
	runner.Coord.FestivalID = FestivalID(n.Coord.FestivalPath)
	runs, _, err := runner.RunPost(ctx, LevelWorkflow, n.Verb, []PlannedHook{{Name: n.Name, Timing: TimingPost, Hook: hook}}, nil)
	if err != nil {
		_, _ = fmt.Fprintf(diagnostics, "Warning: %s: %s\n", n.Name, stderrTail(err.Error()))
		return
	}
	for _, run := range runs {
		if run.Outcome != OutcomePass {
			detail := string(run.Outcome)
			if run.Err != nil {
				detail += ": " + stderrTail(run.Err.Error())
			}
			_, _ = fmt.Fprintf(diagnostics, "Warning: %s: %s (workflow progress is saved)\n", n.Name, detail)
		}
	}
}
