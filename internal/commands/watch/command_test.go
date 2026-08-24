package watch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/resolver"
	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func TestShowWatchOptionsAlwaysEnableInProgress(t *testing.T) {
	got := showWatchOptions(options{})
	if !got.InProgress {
		t.Fatal("fest watch must always enable in-progress expansion")
	}
}

func TestShowWatchOptionsMapCommandFlags(t *testing.T) {
	got := showWatchOptions(options{
		summary:      true,
		goals:        true,
		showFeedback: true,
		collapsed:    true,
	})

	if !got.Summary {
		t.Fatal("summary flag was not mapped")
	}
	if !got.Goals {
		t.Fatal("goals flag was not mapped")
	}
	if !got.Collapsed {
		t.Fatal("collapsed flag was not mapped")
	}
	if !got.ShowFeedback {
		t.Fatal("show-feedback flag was not mapped")
	}
}

func TestWatchCommandDoesNotExposeJSONFlag(t *testing.T) {
	cmd := NewWatchCommand()
	if flag := cmd.Flags().Lookup("json"); flag != nil {
		t.Fatal("fest watch should not expose --json in the first production PR")
	}
}

func TestWatchCommandDelegatesWithMappedOptions(t *testing.T) {
	var gotSelector string
	var gotFestival *show.FestivalInfo
	var gotOptions show.WatchOptions
	calledWatch := false

	cmd := newWatchCommand(commandDeps{
		resolve: func(_ context.Context, selector string) (*show.FestivalInfo, error) {
			gotSelector = selector
			return &show.FestivalInfo{Path: "/campaign/festivals/active/test-TT0001"}, nil
		},
		watch: func(_ context.Context, festival *show.FestivalInfo, opts show.WatchOptions) error {
			calledWatch = true
			gotFestival = festival
			gotOptions = opts
			return nil
		},
	})
	cmd.SetArgs([]string{"test-TT0001", "--goals", "--show-feedback", "--collapsed", "--summary"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotSelector != "test-TT0001" {
		t.Fatalf("selector = %q", gotSelector)
	}
	if !calledWatch {
		t.Fatal("watch delegate was not called")
	}
	if gotFestival == nil || gotFestival.Path != "/campaign/festivals/active/test-TT0001" {
		t.Fatalf("festival = %#v", gotFestival)
	}
	if !gotOptions.InProgress {
		t.Fatal("watch options must enable in-progress expansion")
	}
	if !gotOptions.Goals || !gotOptions.ShowFeedback || !gotOptions.Collapsed || !gotOptions.Summary {
		t.Fatalf("watch options did not map flags: %#v", gotOptions)
	}
}

func TestWatchCommandStandaloneWorkflowDelegatesBeforeFestivalResolution(t *testing.T) {
	var gotWorkflow *show.StandaloneWorkflowInfo
	var gotOptions show.WatchOptions
	calledStandalone := false

	cmd := newWatchCommand(commandDeps{
		resolveStandalone: func(context.Context) (*show.StandaloneWorkflowInfo, error) {
			return &show.StandaloneWorkflowInfo{StartDir: "/workspace/workflow", WorkflowDoc: "/workspace/workflow/WORKFLOW.md"}, nil
		},
		resolve: func(context.Context, string) (*show.FestivalInfo, error) {
			t.Fatal("festival resolver should not be called for standalone workflow context")
			return nil, nil
		},
		watch: func(context.Context, *show.FestivalInfo, show.WatchOptions) error {
			t.Fatal("festival watch delegate should not be called for standalone workflow context")
			return nil
		},
		watchStandalone: func(_ context.Context, workflow *show.StandaloneWorkflowInfo, opts show.WatchOptions) error {
			calledStandalone = true
			gotWorkflow = workflow
			gotOptions = opts
			return nil
		},
	})
	cmd.SetArgs([]string{"--summary"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !calledStandalone {
		t.Fatal("standalone watch delegate was not called")
	}
	if gotWorkflow == nil || gotWorkflow.WorkflowDoc != "/workspace/workflow/WORKFLOW.md" {
		t.Fatalf("workflow = %#v", gotWorkflow)
	}
	if !gotOptions.InProgress || !gotOptions.Summary {
		t.Fatalf("watch options were not mapped for standalone workflow: %#v", gotOptions)
	}
}

func TestWatchCommandSelectorSkipsStandaloneResolution(t *testing.T) {
	calledResolve := false
	cmd := newWatchCommand(commandDeps{
		resolveStandalone: func(context.Context) (*show.StandaloneWorkflowInfo, error) {
			t.Fatal("standalone resolver should not be called when selector is explicit")
			return nil, nil
		},
		resolve: func(_ context.Context, selector string) (*show.FestivalInfo, error) {
			calledResolve = true
			if selector != "festival-FS0001" {
				t.Fatalf("selector = %q", selector)
			}
			return &show.FestivalInfo{Path: "/campaign/festivals/active/festival-FS0001"}, nil
		},
		watch: func(context.Context, *show.FestivalInfo, show.WatchOptions) error {
			return nil
		},
	})
	cmd.SetArgs([]string{"festival-FS0001"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !calledResolve {
		t.Fatal("festival resolver was not called")
	}
}

func TestWatchCommandCancellationDoesNotCallDelegate(t *testing.T) {
	cmd := newWatchCommand(commandDeps{
		resolve: func(context.Context, string) (*show.FestivalInfo, error) {
			return nil, resolver.ErrPickerCancelled
		},
		watch: func(context.Context, *show.FestivalInfo, show.WatchOptions) error {
			t.Fatal("watch delegate should not be called after picker cancellation")
			return nil
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestWatchCommandPropagatesNilFestivalToDelegate(t *testing.T) {
	calledWatch := false
	cmd := newWatchCommand(commandDeps{
		resolve: func(context.Context, string) (*show.FestivalInfo, error) {
			return nil, nil
		},
		watch: func(context.Context, *show.FestivalInfo, show.WatchOptions) error {
			calledWatch = true
			return errors.New("nil festival")
		},
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected delegate error")
	}
	if !calledWatch {
		t.Fatal("watch delegate should receive nil festival and own validation")
	}
	if !strings.Contains(err.Error(), "nil festival") {
		t.Fatalf("unexpected error: %v", err)
	}
}
