package show

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func TestShowCommand_RejectsPositionalWithFestivalFlag(t *testing.T) {
	cmd := NewShowCommand()
	cmd.SetArgs([]string{"launch-readiness-LR0001", "--festival", "LR0001"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot use positional target with --festival") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowCommand_ProvidesFeedbackOptIn(t *testing.T) {
	cmd := NewShowCommand()
	flag := cmd.Flags().Lookup("show-feedback")
	if flag == nil {
		t.Fatal("show command missing --show-feedback")
	}
	if flag.DefValue != "false" {
		t.Fatalf("--show-feedback default = %q, want false", flag.DefValue)
	}
}

func TestRunShowBySelector_FromProjectDirInCampaign(t *testing.T) {
	campaignRoot := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(campaignRoot, ".campaign"), 0755); err != nil {
		t.Fatal(err)
	}

	festivalDir := filepath.Join(campaignRoot, "festivals", "active", "launch-readiness-LR0001")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(campaignRoot, "projects", "app", "src")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	err = runShowBySelector(context.Background(), "LR0001", &showOptions{json: true})
	if err != nil {
		t.Fatalf("runShowBySelector() unexpected error: %v", err)
	}
}

func TestRunShowBySelector_RequiresCampaignWorkspace(t *testing.T) {
	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	err = runShowBySelector(context.Background(), "LR0001", &showOptions{})
	if err == nil {
		t.Fatal("expected camp error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "camp") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunShowCurrent_JSONDoesNotPromptOutsideFestival(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	projectDir := filepath.Join(campaignRoot, "projects", "app")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	restoreCWD := chdirForShowTest(t, projectDir)
	defer restoreCWD()

	output, err := captureShowStdoutAllowError(t, func() error {
		return runShowCurrent(context.Background(), &showOptions{json: true})
	})
	if err != festerrors.ErrAlreadyPrinted {
		t.Fatalf("runShowCurrent() error = %v, want ErrAlreadyPrinted", err)
	}
	if !strings.Contains(output, `"error"`) || !strings.Contains(output, "not in a festival directory or linked project") {
		t.Fatalf("unexpected JSON error output:\n%s", output)
	}
}

func TestRunShowCurrent_NonTerminalOutsideFestivalReturnsHint(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	projectDir := filepath.Join(campaignRoot, "projects", "app")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	restoreCWD := chdirForShowTest(t, projectDir)
	defer restoreCWD()

	// go test runs without a controlling tty, so bare `fest show` outside a
	// festival cannot enter the cycle and must return the actionable hint.
	err := runShowCurrent(context.Background(), &showOptions{})
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	if !strings.Contains(err.Error(), "--festival <selector>") {
		t.Fatalf("error hint should mention explicit selector, got: %v", err)
	}
}

func TestEmitShowFestivalJSONIncludesRenderableTreeView(t *testing.T) {
	festivalDir := makeShowJSONFestival(t)
	festival := &FestivalInfo{
		ID:         "launch-readiness-LR0001",
		MetadataID: "LR0001",
		Name:       "launch-readiness",
		Status:     "active",
		Path:       festivalDir,
		Stats: &FestivalStats{
			Progress: 0,
			Tasks:    StatusCounts{Total: 1, Pending: 1},
		},
	}

	output := captureShowStdout(t, func() error {
		return emitShowFestival(context.Background(), festival, &showOptions{json: true, goals: true}, "")
	})

	var got struct {
		ID         string `json:"id"`
		MetadataID string `json:"metadata_id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		View       struct {
			Mode    string `json:"mode"`
			Options struct {
				ShowGoals bool `json:"show_goals"`
			} `json:"options"`
			Tree struct {
				Name     string `json:"name"`
				NodeType string `json:"node_type"`
				Goal     string `json:"goal"`
				Children []struct {
					Name     string `json:"name"`
					NodeType string `json:"node_type"`
					Goal     string `json:"goal"`
					Children []struct {
						Name     string `json:"name"`
						NodeType string `json:"node_type"`
						Children []struct {
							Name     string `json:"name"`
							NodeType string `json:"node_type"`
							Status   string `json:"status"`
						} `json:"children"`
					} `json:"children"`
				} `json:"children"`
			} `json:"tree"`
		} `json:"view"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("failed to decode JSON: %v\n%s", err, output)
	}

	if got.ID != "launch-readiness-LR0001" || got.MetadataID != "LR0001" {
		t.Fatalf("top-level festival metadata missing: %#v", got)
	}
	if got.View.Mode != "tree" {
		t.Fatalf("view.mode = %q, want tree", got.View.Mode)
	}
	if !got.View.Options.ShowGoals {
		t.Fatal("view.options.show_goals = false, want true")
	}
	if got.View.Tree.Name != filepath.Base(festivalDir) || got.View.Tree.NodeType != "festival" {
		t.Fatalf("unexpected root tree node: %#v", got.View.Tree)
	}
	if got.View.Tree.Goal != "Ship a useful show JSON view." {
		t.Fatalf("tree goal = %q", got.View.Tree.Goal)
	}
	if len(got.View.Tree.Children) != 1 {
		t.Fatalf("tree children = %d, want 1", len(got.View.Tree.Children))
	}
	phase := got.View.Tree.Children[0]
	if phase.Name != "001_PLAN" || phase.NodeType != "phase" || phase.Goal != "Plan the implementation." {
		t.Fatalf("unexpected phase node: %#v", phase)
	}
	if len(phase.Children) != 1 || len(phase.Children[0].Children) != 1 {
		t.Fatalf("expected sequence with one task: %#v", phase.Children)
	}
	task := phase.Children[0].Children[0]
	if task.Name != "01_define_view.md" || task.NodeType != "task" || task.Status != "pending" {
		t.Fatalf("unexpected task node: %#v", task)
	}
}

func TestEmitShowFestivalJSONSummaryModePreservesMetadata(t *testing.T) {
	festivalDir := makeShowJSONFestival(t)
	festival := &FestivalInfo{
		ID:     "launch-readiness-LR0001",
		Name:   "launch-readiness",
		Status: "active",
		Path:   festivalDir,
	}

	output := captureShowStdout(t, func() error {
		return emitShowFestival(context.Background(), festival, &showOptions{json: true, summary: true}, "")
	})

	var got struct {
		ID   string `json:"id"`
		View struct {
			Mode string `json:"mode"`
			Tree *struct {
				Name string `json:"name"`
			} `json:"tree"`
		} `json:"view"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("failed to decode JSON: %v\n%s", err, output)
	}

	if got.ID != "launch-readiness-LR0001" {
		t.Fatalf("top-level id = %q, want launch-readiness-LR0001", got.ID)
	}
	if got.View.Mode != "summary" {
		t.Fatalf("view.mode = %q, want summary", got.View.Mode)
	}
	if got.View.Tree != nil {
		t.Fatalf("summary JSON should not include tree: %#v", got.View.Tree)
	}
}

func makeShowPickerCampaign(t *testing.T) (string, string) {
	t.Helper()

	campaignRoot := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(campaignRoot, ".campaign"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(campaignRoot, "festivals", ".festival"), 0755); err != nil {
		t.Fatal(err)
	}

	festivalDir := filepath.Join(campaignRoot, "festivals", "active", "launch-readiness-LR0001")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Launch Readiness\n"), 0644); err != nil {
		t.Fatal(err)
	}

	return campaignRoot, festivalDir
}

func chdirForShowTest(t *testing.T, dir string) func() {
	t.Helper()

	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(origCWD); err != nil {
			t.Fatal(err)
		}
	}
}

func makeShowJSONFestival(t *testing.T) string {
	t.Helper()

	festivalDir := filepath.Join(t.TempDir(), "launch-readiness-LR0001")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("**Primary Goal:** Ship a useful show JSON view.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	phaseDir := filepath.Join(festivalDir, "001_PLAN")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte("**Primary Goal:** Plan the implementation.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	seqDir := filepath.Join(phaseDir, "01_design")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("**Primary Goal:** Design the schema.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	task := `---
fest_type: task
fest_tracking: true
---
# Define view
`
	if err := os.WriteFile(filepath.Join(seqDir, "01_define_view.md"), []byte(task), 0644); err != nil {
		t.Fatal(err)
	}

	return festivalDir
}

func captureShowStdout(t *testing.T, fn func() error) string {
	t.Helper()

	output, runErr := captureShowStdoutAllowError(t, fn)
	if runErr != nil {
		t.Fatalf("function returned error: %v\noutput:\n%s", runErr, output)
	}
	return output
}

func captureShowStdoutAllowError(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := fn()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, readErr := buf.ReadFrom(r); readErr != nil {
		t.Fatal(readErr)
	}
	return buf.String(), runErr
}

func TestNotInFestivalHint_LeadsWithDiscovery(t *testing.T) {
	// The hint must teach selector discovery before selector usage: an agent
	// outside a festival has no way to know a valid <selector> without
	// `fest list` (intent fest-show-bare-invocation-outside-20260724).
	err := notInFestivalError()
	hint := err.Error()
	listIdx := strings.Index(hint, "fest list")
	selIdx := strings.Index(hint, "--festival <selector>")
	if listIdx < 0 || selIdx < 0 {
		t.Fatalf("hint must mention both fest list and --festival: %v", hint)
	}
	if listIdx > selIdx {
		t.Fatalf("fest list should come before --festival in the hint: %v", hint)
	}
}
