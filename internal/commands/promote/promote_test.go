package promote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	ferrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from   string
		wantTo string
		wantOK bool
	}{
		{"planning", "ready", true},
		{"ready", "active", true},
		{"active", "completed", true},
		{"completed", "", false},
		{"dungeon", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.from, func(t *testing.T) {
			to, ok := validTransitions[tt.from]
			if ok != tt.wantOK {
				t.Errorf("validTransitions[%q]: got ok=%v, want ok=%v", tt.from, ok, tt.wantOK)
			}
			if to != tt.wantTo {
				t.Errorf("validTransitions[%q]: got %q, want %q", tt.from, to, tt.wantTo)
			}
		})
	}
}

func TestValidatePlannedToReady(t *testing.T) {
	t.Run("with goal file", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\nTest"), 0644)

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "planning",
		}

		err := validatePlannedToReady(festival)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("without goal file", func(t *testing.T) {
		dir := t.TempDir()

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "planning",
		}

		err := validatePlannedToReady(festival)
		if err == nil {
			t.Error("expected error for missing FESTIVAL_GOAL.md")
		}
	})
}

func TestValidateActiveToCompleted(t *testing.T) {
	t.Run("empty festival", func(t *testing.T) {
		dir := t.TempDir()

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "active",
		}

		err := validateActiveToCompleted(t.Context(), festival)
		if err == nil {
			t.Error("expected error for festival with no tasks")
		}
	})
}

func TestNewPromoteCommand(t *testing.T) {
	cmd := NewPromoteCommand()
	if cmd.Use != "promote [festival]" {
		t.Errorf("expected Use=%q, got %q", "promote [festival]", cmd.Use)
	}
	if cmd.ValidArgsFunction == nil {
		t.Error("expected ValidArgsFunction to be set for festival completion")
	}

	// Check flags exist
	flags := []string{"force", "json", "dungeon"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestPromoteHasColorizedCompletionsSubcommand(t *testing.T) {
	cmd := NewPromoteCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() != "completions" {
			continue
		}
		found = true
		if !sub.Hidden {
			t.Error("completions subcommand should be hidden")
		}
		if sub.Flags().Lookup("color") == nil {
			t.Error("completions subcommand should expose --color")
		}
	}
	if !found {
		t.Fatal("expected hidden 'completions' subcommand for colorized promote completion")
	}
}

func TestDungeonFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		dungeon string
		wantErr bool
	}{
		{"valid completed", "completed", false},
		{"valid archived", "archived", false},
		{"valid someday", "someday", false},
		{"invalid status", "active", true},
		{"invalid arbitrary", "nonsense", true},
		{"invalid planning", "planning", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := id.ResolveStatusPath(tt.dungeon)
			isDungeon := strings.HasPrefix(resolved, "dungeon/")
			if isDungeon == tt.wantErr {
				t.Errorf("dungeon=%q: resolved=%q, isDungeon=%v, wantErr=%v",
					tt.dungeon, resolved, isDungeon, tt.wantErr)
			}
		})
	}
}

func writeGateFestival(t *testing.T, dir, festivalID, status string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: \"1.0\"\nmetadata:\n  id: " + festivalID +
		"\n  status_history:\n    - status: " + status + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeGateChain(t *testing.T, path string) {
	t.Helper()
	yaml := `chain_version: "1.0"
metadata:
  id: CH0001
  name: onboarding-readiness
  created_at: 2026-01-01T00:00:00Z
  status: active
festivals:
  - ref: audit
    id: FA0009
    name: audit-remediation
  - ref: onboarding
    id: FA0010
    name: onboarding-parity
edges:
  - from: audit
    to: onboarding
    type: hard
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupChainGateCampaign builds a campaign with a CH0001 chain (hard edge
// FA0009 -> FA0010), FA0010 in ready, and FA0009 at upstreamStatus in the given
// upstreamDir relative to festivals/. Returns the FA0010 festival dir.
func setupChainGateCampaign(t *testing.T, upstreamRelDir, upstreamStatus string) string {
	t.Helper()
	root := t.TempDir()
	festivals := filepath.Join(root, "festivals")
	if err := os.MkdirAll(filepath.Join(festivals, ".festival"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(festivals, "chains"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeGateChain(t, filepath.Join(festivals, "chains", "onboarding-readiness-CH0001.yaml"))

	fa10 := filepath.Join(festivals, "ready", "onboarding-parity-FA0010")
	writeGateFestival(t, fa10, "FA0010", "ready")
	writeGateFestival(t, filepath.Join(festivals, upstreamRelDir, "audit-remediation-FA0009"), "FA0009", upstreamStatus)

	t.Chdir(festivals)
	return fa10
}

func TestCheckChainDependencies_DungeonCompletedUpstreamDoesNotBlock(t *testing.T) {
	fa10 := setupChainGateCampaign(t, filepath.Join("dungeon", "completed", "2026-06-04"), "completed")

	festival := &show.FestivalInfo{Name: "onboarding-parity", Path: fa10, Status: "ready"}
	blocked, msg := checkChainDependencies(t.Context(), festival)
	if blocked {
		t.Fatalf("expected no block when upstream FA0009 is completed in the dungeon, got: %s", msg)
	}
}

func TestCheckChainDependencies_IncompleteUpstreamStillBlocks(t *testing.T) {
	fa10 := setupChainGateCampaign(t, "active", "active")

	festival := &show.FestivalInfo{Name: "onboarding-parity", Path: fa10, Status: "ready"}
	blocked, msg := checkChainDependencies(t.Context(), festival)
	if !blocked {
		t.Fatal("expected a block when upstream FA0009 is still active (not completed)")
	}
	if !strings.Contains(msg, "FA0009") {
		t.Fatalf("block message should name the incomplete upstream, got: %s", msg)
	}
}

func TestCheckChainDependencies_AnchorsOnFestivalNotCwd(t *testing.T) {
	fa10 := setupChainGateCampaign(t, "active", "active")
	root := filepath.Dir(filepath.Dir(filepath.Dir(fa10)))
	t.Chdir(root)

	festival := &show.FestivalInfo{Name: "onboarding-parity", Path: fa10, Status: "ready"}
	blocked, msg := checkChainDependencies(t.Context(), festival)
	if !blocked {
		t.Fatal("expected chain gate to block when checked from outside festivals/")
	}
	if !strings.Contains(msg, "FA0009") {
		t.Fatalf("block message should name the incomplete upstream, got: %s", msg)
	}
}

func TestRunPromote_SelectorReadyToActiveBlockedByChainFromCampaignRoot(t *testing.T) {
	root := t.TempDir()
	festivals := filepath.Join(root, "festivals")
	for _, dir := range []string{
		filepath.Join(root, ".campaign"),
		filepath.Join(festivals, ".festival"),
		filepath.Join(festivals, "chains"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeGateChain(t, filepath.Join(festivals, "chains", "onboarding-readiness-CH0001.yaml"))
	fa10 := filepath.Join(festivals, "ready", "onboarding-parity-FA0010")
	writeGateFestival(t, fa10, "FA0010", "ready")
	writeGateFestival(t, filepath.Join(festivals, "active", "audit-remediation-FA0009"), "FA0009", "active")
	t.Setenv("CAMP_ROOT", root)
	t.Chdir(root)

	if err := runPromote(t.Context(), &promoteOptions{noCommit: true}, "FA0010"); err != nil {
		t.Fatalf("runPromote: %v", err)
	}

	if _, err := os.Stat(fa10); err != nil {
		t.Fatalf("FA0010 should remain in ready (chain gate must block), got stat err: %v", err)
	}
	promoted := filepath.Join(festivals, "active", "onboarding-parity-FA0010")
	if _, err := os.Stat(promoted); !os.IsNotExist(err) {
		t.Fatal("FA0010 must not be promoted to active while hard upstream FA0009 is incomplete")
	}
}

func writePromoteFestival(t *testing.T, dir, festivalID, status string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: \"1.0\"\nmetadata:\n  id: " + festivalID +
		"\n  status_history:\n    - status: " + status + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\nTest goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupPromoteCampaign(t *testing.T) (root, festivalsDir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	festivalsDir = filepath.Join(root, "festivals")
	for _, status := range []string{"planning", "ready", "active", "ritual"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writePromoteFestival(t, filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"), "FE0001", "planning")
	writePromoteFestival(t, filepath.Join(festivalsDir, "ready", "beta-feature-FE0002"), "FE0002", "ready")
	writePromoteFestival(t, filepath.Join(festivalsDir, "active", "gamma-feature-FE0003"), "FE0003", "active")
	writePromoteFestival(t, filepath.Join(festivalsDir, "ritual", "delta-ritual"), "", "ritual")
	t.Setenv("CAMP_ROOT", root)
	return root, festivalsDir
}

func TestResolveFestivalForPromote_Selector(t *testing.T) {
	root, _ := setupPromoteCampaign(t)

	festival, fromPicker, err := resolveFestivalForPromote(t.Context(), "", root, "alpha-feature-FE0001", false)
	if err != nil {
		t.Fatalf("resolveFestivalForPromote: %v", err)
	}
	if fromPicker {
		t.Error("explicit selector must not be reported as a picker selection")
	}
	if festival.Name != "alpha-feature-FE0001" {
		t.Errorf("got festival %q, want alpha-feature-FE0001", festival.Name)
	}
	if festival.Status != "planning" {
		t.Errorf("got status %q, want planning", festival.Status)
	}
}

func TestResolveFestivalForPromote_CwdContext(t *testing.T) {
	_, festivalsDir := setupPromoteCampaign(t)
	activeDir := filepath.Join(festivalsDir, "active", "gamma-feature-FE0003")

	festival, fromPicker, err := resolveFestivalForPromote(t.Context(), festivalsDir, activeDir, "", false)
	if err != nil {
		t.Fatalf("resolveFestivalForPromote: %v", err)
	}
	if fromPicker {
		t.Error("cwd context must not be reported as a picker selection")
	}
	if festival.Name != "gamma-feature-FE0003" || festival.Status != "active" {
		t.Errorf("got %q/%q, want gamma-feature-FE0003/active", festival.Name, festival.Status)
	}
}

func TestResolveFestivalForPromote_NoContextNonInteractive(t *testing.T) {
	root, festivalsDir := setupPromoteCampaign(t)

	_, fromPicker, err := resolveFestivalForPromote(t.Context(), festivalsDir, root, "", false)
	if err == nil {
		t.Fatal("expected an error when no festival context is available and the picker is disabled")
	}
	if fromPicker {
		t.Error("no festival should be reported as a picker selection")
	}

	var structured *ferrors.Error
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured error, got %T: %v", err, err)
	}
	if structured.Code != ferrors.ErrCodeNotFound {
		t.Fatalf("code = %q, want %q (agents branch on ErrCodeNotFound)", structured.Code, ferrors.ErrCodeNotFound)
	}
	if !ferrors.Is(err, ferrors.ErrCodeNotFound) {
		t.Fatalf("errors.Is(%q) = false", ferrors.ErrCodeNotFound)
	}
	if structured.Fields["resource"] != "festival" {
		t.Fatalf("resource field = %#v, want festival", structured.Fields["resource"])
	}
	if structured.Hint == "" {
		t.Fatal("hint must be set so operators know how to recover")
	}
	if !strings.Contains(structured.Hint, "interactive terminal") {
		t.Fatalf("hint should mention interactive terminal, got %q", structured.Hint)
	}
}

func TestPromotePickerNarrowsToStatusDirectoryLikeWatch(t *testing.T) {
	cwd := "/campaign/festivals/ready"
	festivalsDir := "/campaign/festivals"
	r := promoteTargetResolver(cwd, festivalsDir, true)
	got := r.PickerOptions(cwd, festivalsDir)
	if !reflect.DeepEqual(got.PreferredStatuses, []string{"ready"}) {
		t.Fatalf("promote picker PreferredStatuses = %#v, want [ready] (must match fest watch)", got.PreferredStatuses)
	}
	if !reflect.DeepEqual(got.FallbackStatuses, shared.WorkingFestivalPickerStatuses) {
		t.Fatalf("promote picker FallbackStatuses = %#v, want working statuses", got.FallbackStatuses)
	}

	fromRoot := promoteTargetResolver("/campaign", festivalsDir, true).PickerOptions("/campaign", festivalsDir)
	if !reflect.DeepEqual(fromRoot.PreferredStatuses, shared.WorkingFestivalPickerStatuses) {
		t.Fatalf("promote picker from campaign root PreferredStatuses = %#v, want working statuses", fromRoot.PreferredStatuses)
	}
}

func TestCompletePromoteTargetExcludesRitual(t *testing.T) {
	root, _ := setupPromoteCampaign(t)

	candidates, err := shared.ListFestivalPickCandidates(t.Context(), root, promotePickerOptions())
	if err != nil {
		t.Fatalf("ListFestivalPickCandidates: %v", err)
	}
	completions := shared.OrderedSelectorNames(candidates, "")

	joined := strings.Join(completions, " ")
	for _, want := range []string{"alpha-feature-FE0001", "beta-feature-FE0002", "gamma-feature-FE0003"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected completion to include %q, got %v", want, completions)
		}
	}
	if strings.Contains(joined, "delta-ritual") {
		t.Errorf("ritual festival must not appear in promote completions, got %v", completions)
	}
}

func TestRunPromote_SelectorPromotesPlanningToReady(t *testing.T) {
	_, festivalsDir := setupPromoteCampaign(t)

	if err := runPromote(t.Context(), &promoteOptions{noCommit: true}, "alpha-feature-FE0001"); err != nil {
		t.Fatalf("runPromote: %v", err)
	}

	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, err := os.Stat(movedPath); err != nil {
		t.Fatalf("expected festival at %s after promotion: %v", movedPath, err)
	}
	oldPath := filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected planning copy removed, stat err=%v", err)
	}
}

func feedStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
}

func TestPromoteResolved_ConfirmedSharesTheCLIFlow(t *testing.T) {
	_, festivalsDir := setupPromoteCampaign(t)

	festival, _, err := resolveFestivalForPromote(t.Context(), "", filepath.Dir(festivalsDir), "alpha-feature-FE0001", false)
	if err != nil {
		t.Fatalf("resolveFestivalForPromote: %v", err)
	}

	feedStdin(t, "y\n")
	newPath, err := PromoteResolved(t.Context(), festival)
	if err != nil {
		t.Fatalf("PromoteResolved: %v", err)
	}
	if newPath == "" {
		t.Fatal("confirmed promotion should return the post-move path")
	}

	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, err := os.Stat(movedPath); err != nil {
		t.Fatalf("expected festival moved to %s: %v", movedPath, err)
	}
	if _, err := os.Stat(filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")); !os.IsNotExist(err) {
		t.Fatalf("expected planning copy removed, stat err=%v", err)
	}
}

func TestPromoteResolved_DeclinedDoesNotMove(t *testing.T) {
	_, festivalsDir := setupPromoteCampaign(t)

	festival, _, err := resolveFestivalForPromote(t.Context(), "", filepath.Dir(festivalsDir), "alpha-feature-FE0001", false)
	if err != nil {
		t.Fatalf("resolveFestivalForPromote: %v", err)
	}

	feedStdin(t, "n\n")
	newPath, err := PromoteResolved(t.Context(), festival)
	if err != nil {
		t.Fatalf("PromoteResolved: %v", err)
	}
	if newPath != "" {
		t.Fatalf("declined promotion must not return a path, got %q", newPath)
	}
	if _, err := os.Stat(filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")); err != nil {
		t.Fatalf("festival must remain in planning after decline: %v", err)
	}
}

func TestEnsureWorkspaceContext_ResolvesForEmbeddedPromotion(t *testing.T) {
	var gotPath string
	ctx, err := ensureWorkspaceContext(t.Context(), "/campaign/festivals/active/example-FE0001", func(_ context.Context, path string) (workspace.WorkspaceInfo, error) {
		gotPath = path
		return workspace.WorkspaceInfo{
			Root:          "/campaign",
			FestivalsPath: "/campaign/festivals",
			Type:          workspace.WorkspaceTypeCampaign,
		}, nil
	})
	if err != nil {
		t.Fatalf("ensureWorkspaceContext: %v", err)
	}
	if gotPath != "/campaign/festivals/active/example-FE0001" {
		t.Fatalf("resolver path = %q, want resolved festival path", gotPath)
	}

	got, ok := scope.WorkspaceFrom(ctx)
	if !ok || got == nil {
		t.Fatal("embedded promotion context is missing workspace metadata")
	}
	if got.Root != "/campaign" || got.FestivalsPath != "/campaign/festivals" || got.Type != scope.WorkspaceTypeCampaign {
		t.Fatalf("workspace context = %#v", got)
	}
}

func TestEnsureWorkspaceContext_PreservesCommandResolvedWorkspace(t *testing.T) {
	want := &scope.WorkspaceInfo{
		Root:          "/existing",
		FestivalsPath: "/existing/festivals",
		Type:          scope.WorkspaceTypeStandalone,
	}
	ctx := scope.WithWorkspace(t.Context(), want)

	gotCtx, err := ensureWorkspaceContext(ctx, "/ignored", func(context.Context, string) (workspace.WorkspaceInfo, error) {
		t.Fatal("resolver must not run when command scope already supplied a workspace")
		return workspace.WorkspaceInfo{}, nil
	})
	if err != nil {
		t.Fatalf("ensureWorkspaceContext: %v", err)
	}
	got, ok := scope.WorkspaceFrom(gotCtx)
	if !ok || got != want {
		t.Fatalf("existing workspace context was replaced: got %#v, want %#v", got, want)
	}
}

func TestEnsureWorkspaceContext_PropagatesResolutionFailure(t *testing.T) {
	wantErr := errors.New("workspace unavailable")
	_, err := ensureWorkspaceContext(t.Context(), "/missing/festival", func(context.Context, string) (workspace.WorkspaceInfo, error) {
		return workspace.WorkspaceInfo{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ensureWorkspaceContext error = %v, want %v", err, wantErr)
	}
}

func capturePromoteStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fnErr := fn()
	_ = w.Close()
	<-done
	return buf.String(), fnErr
}

func assertJSONFailureBody(t *testing.T, out string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("stdout must be parseable JSON, got %q (err: %v)", out, err)
	}
	if body["success"] != false {
		t.Fatalf("JSON body should carry success=false, got %v", body["success"])
	}
	if body["error"] == nil || body["error"] == "" {
		t.Fatalf("JSON body should carry a non-empty error, got %v", body["error"])
	}
}

func TestRunPromote_ResolveFailureJSONExitsNonZeroWithBody(t *testing.T) {
	root, _ := setupPromoteCampaign(t)
	t.Chdir(root)

	out, err := capturePromoteStdout(t, func() error {
		return runPromote(t.Context(), &promoteOptions{json: true, noCommit: true}, "no-such-festival-FE9999")
	})

	if !errors.Is(err, ferrors.ErrAlreadyPrinted) {
		t.Fatalf("resolve failure under --json must return ErrAlreadyPrinted (non-zero exit), got %v", err)
	}
	assertJSONFailureBody(t, out)
}

func TestPromoteCore_InvalidDungeonJSONExitsNonZeroWithBody(t *testing.T) {
	festival := &show.FestivalInfo{Name: "alpha-feature", Status: "planning", Path: t.TempDir()}

	out, err := capturePromoteStdout(t, func() error {
		_, e := promoteCore(t.Context(), festival, false, &promoteOptions{dungeon: "bogus", json: true})
		return e
	})

	if !errors.Is(err, ferrors.ErrAlreadyPrinted) {
		t.Fatalf("invalid --dungeon under --json must return ErrAlreadyPrinted (non-zero exit), got %v", err)
	}
	assertJSONFailureBody(t, out)
}

// A completed festival is not a promote target: promote's picker options carry no
// FallbackStatuses, so an all-dungeon campaign offers nothing rather than
// suggesting work that is already finished.
func TestPromotePickerOptionsExcludeDungeonWhenNoWorkingFestival(t *testing.T) {
	festivals := t.TempDir()
	dir := filepath.Join(festivals, ".dungeon", "completed", "2026-08-19", "cans-CA0001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates := shared.CollectFestivalPickCandidates(festivals, promotePickerOptions())
	if len(candidates) != 0 {
		t.Fatalf("promote completion offered a dungeoned festival: %#v", candidates)
	}
}

// setupPromoteCampaignWithAutoCommitPolicy creates a campaign with the workspace
// config agent.require_auto_commit set to the given value. Returns the root and
// festivals dir.
func setupPromoteCampaignWithAutoCommitPolicy(t *testing.T, requireAutoCommit bool) (root, festivalsDir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	festivalsDir = filepath.Join(root, "festivals")
	// Create .festival/ directory with workspace config
	dotFestivalDir := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestivalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgData := "version: \"1.0\"\nagent:\n  require_auto_commit: " + boolStr(requireAutoCommit) + "\n"
	if err := os.WriteFile(filepath.Join(dotFestivalDir, "config.yaml"), []byte(cfgData), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"planning", "ready", "active"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writePromoteFestival(t, filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"), "FE0001", "planning")
	t.Setenv("CAMP_ROOT", root)
	return root, festivalsDir
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestPromoteCore_NoCommitRejectedWhenPolicyRequiresAutoCommit verifies that
// --no-commit is rejected with an error when agent.require_auto_commit is true.
// This is the core regression test for fest#216.
func TestPromoteCore_NoCommitRejectedWhenPolicyRequiresAutoCommit(t *testing.T) {
	_, festivalsDir := setupPromoteCampaignWithAutoCommitPolicy(t, true)

	festival := &show.FestivalInfo{
		Name:   "alpha-feature-FE0001",
		Path:   filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"),
		Status: "planning",
	}

	_, err := promoteCore(t.Context(), festival, false, &promoteOptions{noCommit: true})
	if err == nil {
		t.Fatal("expected error when --no-commit is used with require_auto_commit policy, got nil")
	}
	if !strings.Contains(err.Error(), "auto-commit is required by policy") {
		t.Fatalf("error should mention auto-commit policy, got: %v", err)
	}
	if strings.Contains(err.Error(), "to disable this guard") {
		t.Fatalf("hint must not tell operators to set require_auto_commit to disable the guard, got: %v", err)
	}
	if !strings.Contains(err.Error(), "remove --no-commit") {
		t.Fatalf("hint should tell operators to remove --no-commit, got: %v", err)
	}

	// Verify the festival was NOT moved (the check happens before any state change)
	oldPath := filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if _, statErr := os.Stat(oldPath); statErr != nil {
		t.Fatalf("festival should remain in planning (not moved), stat err: %v", statErr)
	}
	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, statErr := os.Stat(movedPath); !os.IsNotExist(statErr) {
		t.Fatal("festival must not have been promoted when --no-commit was rejected")
	}
}

// TestPromoteCore_NoCommitRejectedJSONExitsNonZero verifies that --no-commit
// rejection under --json produces a non-zero exit with a JSON error body.
func TestPromoteCore_NoCommitRejectedJSONExitsNonZero(t *testing.T) {
	_, festivalsDir := setupPromoteCampaignWithAutoCommitPolicy(t, true)

	festival := &show.FestivalInfo{
		Name:   "alpha-feature-FE0001",
		Path:   filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"),
		Status: "planning",
	}

	out, err := capturePromoteStdout(t, func() error {
		_, e := promoteCore(t.Context(), festival, false, &promoteOptions{noCommit: true, json: true})
		return e
	})

	if !errors.Is(err, ferrors.ErrAlreadyPrinted) {
		t.Fatalf("--no-commit rejection under --json must return ErrAlreadyPrinted, got %v", err)
	}
	assertJSONFailureBody(t, out)
	if !strings.Contains(out, "remove --no-commit") {
		t.Fatalf("JSON hint should tell operators to remove --no-commit, got: %q", out)
	}
	if strings.Contains(out, "to disable this guard") {
		t.Fatalf("JSON hint must not tell operators to set require_auto_commit to disable the guard, got: %q", out)
	}
}

// TestPromoteCore_NoCommitAllowedWhenPolicyNotSet verifies that --no-commit
// still works (backward compat) when require_auto_commit is not enabled.
func TestPromoteCore_NoCommitAllowedWhenPolicyNotSet(t *testing.T) {
	_, festivalsDir := setupPromoteCampaignWithAutoCommitPolicy(t, false)

	festival := &show.FestivalInfo{
		Name:   "alpha-feature-FE0001",
		Path:   filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"),
		Status: "planning",
	}

	_, err := promoteCore(t.Context(), festival, false, &promoteOptions{noCommit: true})
	if err != nil {
		t.Fatalf("--no-commit should succeed when policy does not require auto-commit, got: %v", err)
	}

	// Festival should have been moved to ready
	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, statErr := os.Stat(movedPath); statErr != nil {
		t.Fatalf("festival should have been promoted to ready: %v", statErr)
	}
}

// TestPromoteCore_NoCommitAbortedWhenPolicyConfigUnreadable verifies that a
// malformed workspace config.yaml does not fail-open to honoring --no-commit.
func TestPromoteCore_NoCommitAbortedWhenPolicyConfigUnreadable(t *testing.T) {
	_, festivalsDir := setupPromoteCampaignWithAutoCommitPolicy(t, true)
	cfgPath := filepath.Join(festivalsDir, ".festival", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1.0\"\nagent: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	festival := &show.FestivalInfo{
		Name:   "alpha-feature-FE0001",
		Path:   filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001"),
		Status: "planning",
	}

	_, err := promoteCore(t.Context(), festival, false, &promoteOptions{noCommit: true})
	if err == nil {
		t.Fatal("expected error when auto-commit policy config is unreadable, got nil")
	}
	if !strings.Contains(err.Error(), "loading auto-commit policy") {
		t.Fatalf("error should mention policy load, got: %v", err)
	}

	oldPath := filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if _, statErr := os.Stat(oldPath); statErr != nil {
		t.Fatalf("festival should remain in planning when policy load fails, stat err: %v", statErr)
	}
}
