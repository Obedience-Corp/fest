package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSequence(t *testing.T, phasePath, name, workingDir string) {
	t.Helper()
	dir := filepath.Join(phasePath, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\nfest_type: sequence\nfest_id: " + name + "\n"
	if workingDir != "" {
		fm += "fest_working_dir: " + workingDir + "\n"
	}
	fm += "---\n\n# Sequence Goal\n"
	if err := os.WriteFile(filepath.Join(dir, "SEQUENCE_GOAL.md"), []byte(fm), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCollectPhaseWorkingDirs(t *testing.T) {
	phase := t.TempDir()

	writeSequence(t, phase, "01_impl", "projects/camp")
	writeSequence(t, phase, "02_other_repo", "projects/fest")
	// A design sequence declares none; the judge already sees the festival.
	writeSequence(t, phase, "03_design", "")
	// Hidden dirs and loose files must not be mistaken for sequences.
	if err := os.MkdirAll(filepath.Join(phase, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phase, "PHASE_GOAL.md"), []byte("# goal\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, skipped := collectPhaseWorkingDirs(phase)
	if len(got) != 2 {
		t.Fatalf("got %d working dirs, want 2: %+v", len(got), got)
	}
	if len(skipped) != 0 {
		t.Fatalf("declaring none is not a skip, got %+v", skipped)
	}
	// A phase spanning two repos must report both, tagged by sequence, or a
	// judge cannot tell which deliverable belongs where.
	if got[0].Sequence != "01_impl" || got[0].Path != "projects/camp" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Sequence != "02_other_repo" || got[1].Path != "projects/fest" {
		t.Errorf("second = %+v", got[1])
	}
}

// Paths must stay campaign-relative. An absolute path per working dir would
// carry the operator's home directory and username into the judge prompt, the
// provider's logs, and every ledger entry and transcript that records the run.
func TestCollectPhaseWorkingDirsEmitsRelativePathsOnly(t *testing.T) {
	phase := t.TempDir()
	writeSequence(t, phase, "01_impl", "projects/camp")

	got, _ := collectPhaseWorkingDirs(phase)
	if len(got) != 1 {
		t.Fatalf("got %d working dirs, want 1", len(got))
	}
	if filepath.IsAbs(got[0].Path) {
		t.Fatalf("path %q is absolute; working dirs must stay campaign-relative", got[0].Path)
	}

	// The JSON contract must not carry an absolute path field either.
	raw, err := json.Marshal(got[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("absolute_path")) {
		t.Fatalf("working dir JSON still exposes absolute_path: %s", raw)
	}
}

func TestCollectPhaseWorkingDirsIsBestEffort(t *testing.T) {
	// A gate must never fail because a sequence is malformed; partial context
	// beats a checkpoint that cannot run.
	phase := t.TempDir()
	writeSequence(t, phase, "01_good", "projects/camp")

	bad := filepath.Join(phase, "02_bad")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "SEQUENCE_GOAL.md"), []byte("not: [valid: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Values normalization must reject rather than hand to the judge to open.
	for name, value := range map[string]string{
		"03_traversing": "../../etc",
		"04_absolute":   "/etc",
		"05_home":       "~/secrets",
	} {
		writeSequence(t, phase, name, value)
	}

	got, skipped := collectPhaseWorkingDirs(phase)
	if len(got) != 1 || got[0].Path != "projects/camp" {
		t.Errorf("malformed sequences must be skipped, got %+v", got)
	}
	// A declaration that was present but rejected is not the same as none, and
	// must be reported rather than vanishing.
	if len(skipped) != 3 {
		t.Fatalf("got %d reported skips, want 3: %+v", len(skipped), skipped)
	}
	for _, skip := range skipped {
		if skip.Value == "" || skip.Reason == "" {
			t.Errorf("skip must carry the offending value and a reason: %+v", skip)
		}
	}
}

// The failure that matters is a phase where every declaration was rejected: the
// judge then sees no working dirs at all, which is indistinguishable from a
// design phase unless it is said out loud.
func TestReportWorkingDirSkipsWarnsLoudlyWhenNothingKept(t *testing.T) {
	var out bytes.Buffer
	reportWorkingDirSkips(&out, []workingDirSkip{
		{Sequence: "01_impl", Value: "/etc", Reason: "must be relative"},
	}, 0)

	got := out.String()
	if !strings.Contains(got, "01_impl") || !strings.Contains(got, "/etc") {
		t.Errorf("warning must name the sequence and the value: %q", got)
	}
	if !strings.Contains(got, "no working directories") {
		t.Errorf("warning must say the judge got nothing: %q", got)
	}

	// When something was kept, the phase is only partially degraded.
	out.Reset()
	reportWorkingDirSkips(&out, []workingDirSkip{
		{Sequence: "01_impl", Value: "/etc", Reason: "must be relative"},
	}, 1)
	if strings.Contains(out.String(), "no working directories") {
		t.Errorf("must not claim the judge got nothing when a dir was kept: %q", out.String())
	}
}

func TestCollectPhaseWorkingDirsUnreadablePhase(t *testing.T) {
	got, skipped := collectPhaseWorkingDirs(filepath.Join(t.TempDir(), "missing"))
	if got != nil || skipped != nil {
		t.Errorf("an unreadable phase must yield nothing, got %+v / %+v", got, skipped)
	}
}

func TestCampaignRootFor(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	festival := filepath.Join(root, "festivals", "active", "demo-D0001")
	if err := os.MkdirAll(festival, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := campaignRootFor(ctx, festival); got != root {
		t.Errorf("campaignRootFor = %q, want %q", got, root)
	}
	// No campaign marker anywhere: relative paths still work, so this must
	// return empty rather than guessing.
	if got := campaignRootFor(ctx, t.TempDir()); got != "" {
		t.Errorf("want empty for a non-campaign path, got %q", got)
	}
}

// The judge's campaign root must be the one the rest of fest resolves, or the
// relative working dirs are relative to a different tree than the judge opens.
func TestCampaignRootForHonorsCampRoot(t *testing.T) {
	ctx := context.Background()
	override := t.TempDir()
	if err := os.MkdirAll(filepath.Join(override, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A festival that would otherwise resolve to its own enclosing campaign.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	festival := filepath.Join(other, "festivals", "active", "demo-D0001")
	if err := os.MkdirAll(festival, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CAMP_ROOT", override)
	if got := campaignRootFor(ctx, festival); got != override {
		t.Errorf("campaignRootFor ignored CAMP_ROOT: got %q, want %q", got, override)
	}
}

// The request shape is the contract the obey judge pins against.
func TestApprovalJudgeRequestJSONKeys(t *testing.T) {
	raw, err := json.Marshal(approvalJudgeRequest{
		SchemaVersion: approvalJudgeSchemaVersion,
		CampaignRoot:  "/campaigns/demo",
		WorkingDirs:   []judgeWorkingDir{{Sequence: "01_impl", Path: "projects/camp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"campaign_root"`, `"working_dirs"`, `"sequence"`, `"path"`} {
		if !bytes.Contains(raw, []byte(key)) {
			t.Errorf("request JSON missing %s: %s", key, raw)
		}
	}

	// Both fields are additive; an older judge must see the request unchanged
	// when a phase declares no working dirs.
	raw, err = json.Marshal(approvalJudgeRequest{SchemaVersion: approvalJudgeSchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"campaign_root", "working_dirs"} {
		if bytes.Contains(raw, []byte(key)) {
			t.Errorf("empty %s must be omitted: %s", key, raw)
		}
	}
}
