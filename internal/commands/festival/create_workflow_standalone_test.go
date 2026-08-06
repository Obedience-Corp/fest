package festival

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRejectFestivalOnlyFlags(t *testing.T) {
	cases := []struct {
		name string
		opts CreateWorkflowOptions
		want string
	}{
		{"festival flag", CreateWorkflowOptions{Festival: "/some/path"}, "--festival is not valid"},
		{"path flag", CreateWorkflowOptions{Path: "./elsewhere"}, "--path is not valid"},
		{"path . is allowed", CreateWorkflowOptions{Path: "."}, ""},
		{"empty is allowed", CreateWorkflowOptions{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := rejectFestivalOnlyFlags(&c.opts)
			if c.want == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}

func TestCreateWorkflowHelpMentionsStandaloneFlow(t *testing.T) {
	cmd := NewCreateWorkflowCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command: %v", err)
	}
	help := out.String()
	for _, want := range []string{
		"Generate a parseable WORKFLOW.md",
		"current directory",
		"starts a tracked run so fest next works",
		"fest create workflow demo",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestPromptForStandaloneInput_HappyPath(t *testing.T) {
	in := strings.NewReader("My Title\nThe intent line\nALIGN|Get aligned\nBUILD|Do the work\n\n")
	var out bytes.Buffer
	got, err := promptForStandaloneInput(context.Background(), in, &out, "default-name")
	if err != nil {
		t.Fatalf("promptForStandaloneInput: %v", err)
	}
	if got.Title != "My Title" {
		t.Errorf("Title = %q, want My Title", got.Title)
	}
	if got.Description != "The intent line" {
		t.Errorf("Description = %q, want The intent line", got.Description)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("Steps len = %d, want 2", len(got.Steps))
	}
	if got.Steps[0].Name != "ALIGN" || got.Steps[0].Goal != "Get aligned" {
		t.Errorf("Steps[0] = %+v", got.Steps[0])
	}
	if got.Steps[1].Name != "BUILD" || got.Steps[1].Goal != "Do the work" {
		t.Errorf("Steps[1] = %+v", got.Steps[1])
	}
}

func TestPromptForStandaloneInput_TitleDefaultsToName(t *testing.T) {
	in := strings.NewReader("\nintent\nSTEP|goal\n\n")
	var out bytes.Buffer
	got, err := promptForStandaloneInput(context.Background(), in, &out, "fallback-name")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "fallback-name" {
		t.Errorf("Title = %q, want fallback-name", got.Title)
	}
}

func TestPromptForStandaloneInput_RequiresAtLeastOneStep(t *testing.T) {
	in := strings.NewReader("title\nintent\n\n")
	var out bytes.Buffer
	_, err := promptForStandaloneInput(context.Background(), in, &out, "name")
	if err == nil {
		t.Fatal("expected error when no steps provided")
	}
	if !strings.Contains(err.Error(), "at least one step") {
		t.Errorf("error %q does not mention at least one step", err.Error())
	}
}

func TestPromptForStandaloneInput_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in := strings.NewReader("ignored\n")
	var out bytes.Buffer
	_, err := promptForStandaloneInput(ctx, in, &out, "name")
	if err == nil {
		t.Fatal("expected ctx.Err() before reading prompt")
	}
}

func TestBuildStandaloneWorkflowInput_FromInlineSteps(t *testing.T) {
	opts := &CreateWorkflowOptions{
		Name:  "demo",
		Steps: `{"title":"Inline","steps":[{"name":"A","goal":"do A"}]}`,
	}
	got, err := buildStandaloneWorkflowInput(context.Background(), opts)
	if err != nil {
		t.Fatalf("buildStandaloneWorkflowInput: %v", err)
	}
	if got.Title != "Inline" {
		t.Errorf("Title = %q, want Inline", got.Title)
	}
	if len(got.Steps) != 1 || got.Steps[0].Name != "A" {
		t.Errorf("Steps = %+v", got.Steps)
	}
}

func TestRejectBlockingCheckpoints(t *testing.T) {
	tests := []struct {
		name    string
		input   *WorkflowInput
		wantErr bool
	}{
		{
			name: "approval_required step rejected",
			input: &WorkflowInput{Title: "t", Steps: []WorkflowStepInput{
				{Name: "PLAN", Goal: "g", Checkpoint: "none"},
				{Name: "GATE", Goal: "g", Checkpoint: "approval_required"},
			}},
			wantErr: true,
		},
		{
			name: "non-blocking checkpoints allowed",
			input: &WorkflowInput{Title: "t", Steps: []WorkflowStepInput{
				{Name: "A", Goal: "g", Checkpoint: "verification"},
				{Name: "B", Goal: "g", Checkpoint: "documentation"},
				{Name: "C", Goal: "g", Checkpoint: "none"},
				{Name: "D", Goal: "g"},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectBlockingCheckpoints(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected rejection, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "blocking checkpoints") {
				t.Fatalf("error should name blocking checkpoints: %v", err)
			}
		})
	}
}
