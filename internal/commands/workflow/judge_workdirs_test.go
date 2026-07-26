package workflow

import (
	"os"
	"path/filepath"
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
	campaign := "/campaigns/demo"

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

	got := collectPhaseWorkingDirs(phase, campaign)
	if len(got) != 2 {
		t.Fatalf("got %d working dirs, want 2: %+v", len(got), got)
	}
	// A phase spanning two repos must report both, tagged by sequence, or a
	// judge cannot tell which deliverable belongs where.
	if got[0].Sequence != "01_impl" || got[0].Path != "projects/camp" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Sequence != "02_other_repo" || got[1].Path != "projects/fest" {
		t.Errorf("second = %+v", got[1])
	}
	if got[0].AbsolutePath != filepath.Join(campaign, "projects/camp") {
		t.Errorf("absolute path = %q", got[0].AbsolutePath)
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

	escaping := filepath.Join(phase, "03_escaping")
	if err := os.MkdirAll(escaping, 0o755); err != nil {
		t.Fatal(err)
	}
	// An absolute or traversing working dir is rejected by normalization rather
	// than handed to the judge as a path to open.
	if err := os.WriteFile(filepath.Join(escaping, "SEQUENCE_GOAL.md"),
		[]byte("---\nfest_type: sequence\nfest_working_dir: ../../etc\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := collectPhaseWorkingDirs(phase, "/campaigns/demo")
	if len(got) != 1 || got[0].Path != "projects/camp" {
		t.Errorf("malformed sequences must be skipped, got %+v", got)
	}
}

func TestCollectPhaseWorkingDirsUnreadablePhase(t *testing.T) {
	if got := collectPhaseWorkingDirs(filepath.Join(t.TempDir(), "missing"), ""); got != nil {
		t.Errorf("an unreadable phase must yield nothing, got %+v", got)
	}
}

func TestCampaignRootFor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	festival := filepath.Join(root, "festivals", "active", "demo-D0001")
	if err := os.MkdirAll(festival, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := campaignRootFor(festival); got != root {
		t.Errorf("campaignRootFor = %q, want %q", got, root)
	}
	// No campaign marker anywhere: relative paths still work, so this must
	// return empty rather than guessing.
	if got := campaignRootFor(t.TempDir()); got != "" {
		t.Errorf("want empty for a non-campaign path, got %q", got)
	}
}
