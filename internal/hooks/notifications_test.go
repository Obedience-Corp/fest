package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
)

func TestNotificationResolution(t *testing.T) {
	yes, no := true, false
	for _, name := range []string{PresentationOpenName, ApprovalNotifyName} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name             string
				def              config.HookDefinition
				enabled, invalid bool
			}{
				{"implicit-disabled", config.HookDefinition{Command: "camp app open ."}, false, false},
				{"disabled-invalid", config.HookDefinition{Enabled: &no, Timeout: "bad"}, false, false},
				{"enabled", config.HookDefinition{Enabled: &yes, Command: "camp app open ."}, true, false},
				{"closed", config.HookDefinition{Enabled: &yes, Command: "x", Fail: "closed"}, false, true},
				{"empty", config.HookDefinition{Enabled: &yes}, false, true},
				{"zero", config.HookDefinition{Enabled: &yes, Command: "x", Timeout: "0s"}, false, true},
				{"negative", config.HookDefinition{Enabled: &yes, Command: "x", Timeout: "-1s"}, false, true},
				{"unbounded", config.HookDefinition{Enabled: &yes, Command: "x", Timeout: "31s"}, false, true},
				{"invalid", config.HookDefinition{Enabled: &yes, Command: "x", Timeout: "bad"}, false, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cfg := &config.HooksConfig{Definitions: map[string]config.HookDefinition{name: tc.def}}
					hook, on, err := ResolveNotification(cfg, name)
					if on != tc.enabled || (err != nil) != tc.invalid {
						t.Fatalf("hook=%+v enabled=%v err=%v", hook, on, err)
					}
					if on && (hook.Fail != FailOpen || hook.Timeout != NotificationTimeout || hook.Source != LayerMachine) {
						t.Fatalf("defaults: %+v", hook)
					}
					// Even an invalid reserved definition cannot affect ordinary hooks.
					eff, err := Resolve(cfg, cfg, cfg)
					if err != nil || len(eff.Hooks) != 0 {
						t.Fatalf("reserved definition leaked: %+v %v", eff, err)
					}
				})
			}
			for _, cfg := range []*config.HooksConfig{nil, {}, {Enabled: &no, Definitions: map[string]config.HookDefinition{name: {Enabled: &yes, Command: "x"}}}, {Levels: map[string]bool{"workflow": false}, Definitions: map[string]config.HookDefinition{name: {Enabled: &yes, Command: "x"}}}} {
				_, on, err := ResolveNotification(cfg, name)
				if on || err != nil {
					t.Fatalf("disabled: %v %v", on, err)
				}
			}
		})
	}
}

func TestNotificationOutboxConsumesOnceOutsideProducer(t *testing.T) {
	ctx, q := WithNotifications(context.Background())
	QueueNotification(ctx, Notification{Name: PresentationOpenName})
	QueueNotification(ctx, Notification{Name: ApprovalNotifyName})
	var names []string
	deliver := func(_ context.Context, n Notification) {
		names = append(names, n.Name)
	}
	q.Drain(ctx, deliver)
	QueueNotification(ctx, Notification{Name: PresentationOpenName})
	q.Drain(ctx, deliver)
	if len(names) != 2 || names[0] != PresentationOpenName || names[1] != ApprovalNotifyName {
		t.Fatalf("delivery: %v", names)
	}
}

func TestNotificationCancellationDiscardsPending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx, q := WithNotifications(ctx)
	QueueNotification(ctx, Notification{Name: PresentationOpenName})
	cancel()
	q.Drain(ctx, func(context.Context, Notification) { t.Fatal("delivered after cancellation") })
	q.Drain(context.Background(), func(context.Context, Notification) { t.Fatal("retried cancelled delivery") })
}

func TestNotificationRunnerContextFailureAndTimeout(t *testing.T) {
	yes := true
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "timeout"}[timeout], func(t *testing.T) {
			cfg := &config.HooksConfig{Definitions: map[string]config.HookDefinition{PresentationOpenName: {Enabled: &yes, Command: "camp app open .", Timeout: "5ms"}}}
			var diagnostics bytes.Buffer
			called := 0
			n := Notification{Name: PresentationOpenName, Verb: VerbWorkflowPresent, Coord: Coord{FestivalPath: "/nonexistent/festival with $ characters", Phase: "001_INGEST", Step: 2}}
			deliverNotification(context.Background(), cfg, n, &diagnostics, func(dir string) *Runner {
				return &Runner{WorkDir: dir, Exec: func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
					called++
					if dir != n.Coord.FestivalPath || command != "camp app open ." {
						t.Fatalf("target: %s %s", dir, command)
					}
					var p Payload
					if err := json.Unmarshal(stdin, &p); err != nil {
						t.Fatal(err)
					}
					if p.Step != 2 || p.Verb != "workflow_present" || p.Timing != "post" || p.FestivalPath != dir {
						t.Fatalf("payload %+v", p)
					}
					if timeout {
						<-ctx.Done()
						return CommandResult{ExitCode: -1, Err: ctx.Err()}
					}
					return CommandResult{ExitCode: 1, Err: errors.New("app missing")}
				}}
			})
			if called != 1 || !strings.Contains(diagnostics.String(), "workflow progress is saved") {
				t.Fatalf("called=%d warning=%s", called, diagnostics.String())
			}
		})
	}
}

func TestNotificationDisabledDoesNotConstructRunner(t *testing.T) {
	var diagnostics bytes.Buffer
	deliverNotification(context.Background(), nil, Notification{Name: PresentationOpenName}, &diagnostics, func(string) *Runner { t.Fatal("created runner without opt-in"); return nil })
	if diagnostics.Len() != 0 {
		t.Fatal(diagnostics.String())
	}
}

func TestNotificationTimeoutLimit(t *testing.T) {
	yes := true
	h, on, err := ResolveNotification(&config.HooksConfig{Definitions: map[string]config.HookDefinition{ApprovalNotifyName: {Enabled: &yes, Command: "x", Timeout: "30s"}}}, ApprovalNotifyName)
	if !on || err != nil || h.Timeout != 30*time.Second {
		t.Fatalf("%+v %v %v", h, on, err)
	}
}

func TestNotificationStderrBounded(t *testing.T) {
	b := &notificationTail{}
	for _, chunk := range []string{strings.Repeat("a", 3000), strings.Repeat("b", 2000), "last"} {
		n, err := b.Write([]byte(chunk))
		if n != len(chunk) || err != nil {
			t.Fatalf("write=%d %v", n, err)
		}
		if len(b.data) > stderrTailLimit {
			t.Fatal("unbounded stderr")
		}
	}
	if !strings.HasSuffix(string(b.data), "last") {
		t.Fatal("lost tail")
	}
}

func TestNotificationProcessDeadline(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep executable unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	runner := newNotificationRunner("")
	result := runner.Exec(ctx, "sleep 10", nil, "")
	if result.Err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("result=%+v ctx=%v", result, ctx.Err())
	}
	if time.Since(started) > time.Second {
		t.Fatal("notification exceeded cancellation cleanup bound")
	}
}
