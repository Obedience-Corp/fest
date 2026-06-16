package promote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/id"
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
	if !strings.Contains(strings.ToLower(err.Error()), "festival") {
		t.Errorf("error should mention festival, got: %v", err)
	}
}

func TestCompletePromoteTargetExcludesRitual(t *testing.T) {
	root, _ := setupPromoteCampaign(t)

	completions, err := shared.CompleteFestivalPickSelectors(t.Context(), root, "", promotePickerOptions())
	if err != nil {
		t.Fatalf("CompleteFestivalPickSelectors: %v", err)
	}

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
