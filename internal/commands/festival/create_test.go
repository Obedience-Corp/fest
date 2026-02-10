package festival

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/festival"
)

// TestCreateSequence_DirectoryCreation tests that creating a sequence
// results in a directory being created (not a file)
func TestCreateSequence_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up phase directory structure
	phaseDir := filepath.Join(tmpDir, "001_PLANNING")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatalf("failed to create phase dir: %v", err)
	}

	// Use renumberer directly (what create sequence uses internally)
	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	err := ren.InsertSequence(context.Background(), phaseDir, 0, "requirements")
	if err != nil {
		t.Fatalf("InsertSequence failed: %v", err)
	}

	// Verify directory was created (not a file)
	seqPath := filepath.Join(phaseDir, "01_requirements")
	info, err := os.Stat(seqPath)
	if err != nil {
		t.Fatalf("expected sequence directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected 01_requirements to be a directory, got file")
	}
}

// TestCreateTask_FileCreation tests that creating a task
// creates the parent directory but doesn't write file content
func TestCreateTask_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up sequence directory structure
	seqDir := filepath.Join(tmpDir, "01_requirements")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatalf("failed to create sequence dir: %v", err)
	}

	// Use renumberer directly (what create task uses internally)
	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	err := ren.InsertTask(context.Background(), seqDir, 0, "define_requirements")
	if err != nil {
		t.Fatalf("InsertTask failed: %v", err)
	}

	// The renumberer ensures parent exists but doesn't create .md file content
	// (the create command is responsible for writing template content)
	taskPath := filepath.Join(seqDir, "01_define_requirements.md")

	// Check parent directory exists
	if _, err := os.Stat(seqDir); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}

	// Task file may or may not exist (renumberer behavior)
	// but if it exists, it should be empty (no template content)
	if info, err := os.Stat(taskPath); err == nil {
		if info.IsDir() {
			t.Error("expected task to be a file, not a directory")
		}
		// If file exists, verify it's empty (no content written by renumberer)
		content, _ := os.ReadFile(taskPath)
		if len(content) > 0 {
			t.Errorf("renumberer should not write content to task file, got: %q", content)
		}
	}
}

// TestCreateSequence_NoIsDirectoryError tests that creating a sequence
// doesn't cause "is a directory" errors when the directory already exists
func TestCreateSequence_NoIsDirectoryError(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up phase directory with existing sequence
	phaseDir := filepath.Join(tmpDir, "001_PLANNING")
	existingSeq := filepath.Join(phaseDir, "01_existing")
	if err := os.MkdirAll(existingSeq, 0755); err != nil {
		t.Fatalf("failed to create existing sequence: %v", err)
	}

	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	// Insert new sequence at beginning (should shift existing)
	err := ren.InsertSequence(context.Background(), phaseDir, 0, "new_sequence")
	if err != nil {
		t.Fatalf("InsertSequence failed with error: %v", err)
	}

	// Verify both sequences exist as directories
	newSeq := filepath.Join(phaseDir, "01_new_sequence")
	shiftedSeq := filepath.Join(phaseDir, "02_existing")

	if info, err := os.Stat(newSeq); err != nil || !info.IsDir() {
		t.Error("expected 01_new_sequence directory")
	}
	if info, err := os.Stat(shiftedSeq); err != nil || !info.IsDir() {
		t.Error("expected 02_existing directory (renumbered)")
	}
}

// TestCreateTask_NoIsDirectoryError tests that creating a task
// with existing tasks doesn't cause errors
func TestCreateTask_NoIsDirectoryError(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up sequence directory with existing task
	seqDir := tmpDir
	if err := os.WriteFile(filepath.Join(seqDir, "01_existing.md"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create existing task: %v", err)
	}

	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	// Insert new task at beginning (should shift existing)
	err := ren.InsertTask(context.Background(), seqDir, 0, "new_task")
	if err != nil {
		t.Fatalf("InsertTask failed with error: %v", err)
	}

	// Verify renumbering happened correctly
	shiftedTask := filepath.Join(seqDir, "02_existing.md")
	if _, err := os.Stat(shiftedTask); err != nil {
		t.Error("expected 02_existing.md (renumbered from 01)")
	}

	// Verify content was preserved in renamed file
	content, err := os.ReadFile(shiftedTask)
	if err != nil {
		t.Fatalf("failed to read shifted task: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("shifted task content = %q, want %q", content, "content")
	}
}

// TestCreatePhase_DirectoryCreation tests that creating a phase
// results in a directory being created
func TestCreatePhase_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	err := ren.InsertPhase(context.Background(), tmpDir, 0, "PLANNING")
	if err != nil {
		t.Fatalf("InsertPhase failed: %v", err)
	}

	// Verify directory was created
	phasePath := filepath.Join(tmpDir, "001_PLANNING")
	info, err := os.Stat(phasePath)
	if err != nil {
		t.Fatalf("expected phase directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected 001_PLANNING to be a directory, got file")
	}
}

// TestCreate_ChainedOperations tests multiple create operations in sequence
func TestCreate_ChainedOperations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival structure progressively
	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	// Create phases
	ctx := context.Background()
	if err := ren.InsertPhase(ctx, tmpDir, 0, "PLANNING"); err != nil {
		t.Fatalf("InsertPhase PLANNING failed: %v", err)
	}
	if err := ren.InsertPhase(ctx, tmpDir, 1, "IMPLEMENT"); err != nil {
		t.Fatalf("InsertPhase IMPLEMENT failed: %v", err)
	}

	// Create sequences in first phase
	phaseDir := filepath.Join(tmpDir, "001_PLANNING")
	if err := ren.InsertSequence(ctx, phaseDir, 0, "requirements"); err != nil {
		t.Fatalf("InsertSequence requirements failed: %v", err)
	}
	if err := ren.InsertSequence(ctx, phaseDir, 1, "design"); err != nil {
		t.Fatalf("InsertSequence design failed: %v", err)
	}

	// Create tasks in first sequence
	seqDir := filepath.Join(phaseDir, "01_requirements")
	if err := ren.InsertTask(ctx, seqDir, 0, "gather_info"); err != nil {
		t.Fatalf("InsertTask gather_info failed: %v", err)
	}
	if err := ren.InsertTask(ctx, seqDir, 1, "analyze"); err != nil {
		t.Fatalf("InsertTask analyze failed: %v", err)
	}

	// Verify complete structure
	expectedDirs := []string{
		"001_PLANNING",
		"002_IMPLEMENT",
		"001_PLANNING/01_requirements",
		"001_PLANNING/02_design",
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpDir, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

// TestCreateOptions_DefaultAfterZero verifies that after=0 creates at position 1
func TestCreateOptions_DefaultAfterZero(t *testing.T) {
	tmpDir := t.TempDir()

	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	// Create with after=0 (default)
	if err := ren.InsertPhase(context.Background(), tmpDir, 0, "FIRST"); err != nil {
		t.Fatalf("InsertPhase failed: %v", err)
	}

	// Should create at position 1 (001_)
	if _, err := os.Stat(filepath.Join(tmpDir, "001_FIRST")); err != nil {
		t.Error("expected 001_FIRST to exist")
	}
}

// TestCreateOptions_InsertInMiddle verifies inserting in the middle of existing elements
func TestCreateOptions_InsertInMiddle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial phases
	os.MkdirAll(filepath.Join(tmpDir, "001_FIRST"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "002_THIRD"), 0755)

	ren := festival.NewRenumberer(festival.RenumberOptions{
		AutoApprove: true,
		Quiet:       true,
	})

	// Insert after position 1
	if err := ren.InsertPhase(context.Background(), tmpDir, 1, "SECOND"); err != nil {
		t.Fatalf("InsertPhase failed: %v", err)
	}

	// Verify structure
	expected := []string{"001_FIRST", "002_SECOND", "003_THIRD"}
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("expected %s to exist", name)
		}
	}
}

// TestCreateFestival_GatesDirectory tests that festival creation creates gates/ directory at root
// with gate templates organized by phase type
func TestCreateFestival_GatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the festivals directory structure with templates
	festivalsDir := filepath.Join(tmpDir, "festivals")
	festivalMetaDir := filepath.Join(festivalsDir, ".festival")
	templatesDir := filepath.Join(festivalMetaDir, "templates")
	phasesTemplatesDir := filepath.Join(templatesDir, "phases")

	// Create phase-type subdirectories with gates/ subdirectory containing gate templates
	// Only implementation phases have quality gates
	phaseTypes := map[string][]string{
		"implementation": {"QUALITY_GATE_TESTING.md", "QUALITY_GATE_REVIEW.md", "QUALITY_GATE_ITERATE.md", "QUALITY_GATE_FEST_COMMIT.md"},
	}

	for phaseType, gates := range phaseTypes {
		// Create phase directory (for structure, but we only copy gates)
		phaseDir := filepath.Join(phasesTemplatesDir, phaseType)

		// Create gates subdirectory with gate templates
		gatesDir := filepath.Join(phaseDir, "gates")
		if err := os.MkdirAll(gatesDir, 0755); err != nil {
			t.Fatalf("failed to create gates dir for %s: %v", phaseType, err)
		}
		for _, gate := range gates {
			content := "# " + gate + "\n\nGate template content for " + phaseType + "."
			if err := os.WriteFile(filepath.Join(gatesDir, gate), []byte(content), 0644); err != nil {
				t.Fatalf("failed to create template %s/gates/%s: %v", phaseType, gate, err)
			}
		}
	}

	// Also create core festival templates to satisfy festival creation
	festivalTemplatesDir := filepath.Join(templatesDir, "festival")
	if err := os.MkdirAll(festivalTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create festival templates dir: %v", err)
	}
	coreTemplates := []string{"OVERVIEW.md", "GOAL.md"}
	for _, tmpl := range coreTemplates {
		content := "# {{.festival_name}}\n"
		if err := os.WriteFile(filepath.Join(festivalTemplatesDir, tmpl), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create template festival/%s: %v", tmpl, err)
		}
	}

	// Change working directory temporarily
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(festivalsDir)

	// Run create festival
	opts := &CreateFestivalOptions{
		Name:        "test-festival",
		Goal:        "Test goal",
		SkipMarkers: true,
		Dest:        "active",
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Find the created festival directory (now includes ID suffix)
	activeDir := filepath.Join(festivalsDir, "active")
	entries, err := os.ReadDir(activeDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry in active/: %v", err)
	}
	festivalDir := filepath.Join(activeDir, entries[0].Name())

	// Verify gates/ directory exists at festival root
	gatesDir := filepath.Join(festivalDir, "gates")
	info, err := os.Stat(gatesDir)
	if err != nil {
		t.Fatalf("expected gates directory to exist at festival root: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected gates to be a directory")
	}

	// Verify phase-type subdirectories within gates/ with their templates
	for phaseType, gates := range phaseTypes {
		phaseGatesDir := filepath.Join(gatesDir, phaseType)
		if _, err := os.Stat(phaseGatesDir); err != nil {
			t.Errorf("expected gates/%s directory to exist: %v", phaseType, err)
			continue
		}

		// Verify gate templates exist
		for _, gate := range gates {
			gatePath := filepath.Join(phaseGatesDir, gate)
			if _, err := os.Stat(gatePath); err != nil {
				t.Errorf("expected gate template gates/%s/%s to exist: %v", phaseType, gate, err)
			}
		}
	}
}

// TestCreateFestival_FestYAMLGenerated tests that fest.yaml is generated with gates config
func TestCreateFestival_FestYAMLGenerated(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the festivals directory structure with templates
	festivalsDir := filepath.Join(tmpDir, "festivals")
	festivalMetaDir := filepath.Join(festivalsDir, ".festival")
	templatesDir := filepath.Join(festivalMetaDir, "templates")
	phasesTemplatesDir := filepath.Join(templatesDir, "phases")

	// Create implementation phase with gates subdirectory
	implGatesDir := filepath.Join(phasesTemplatesDir, "implementation", "gates")
	if err := os.MkdirAll(implGatesDir, 0755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	// Create minimal implementation gate templates with correct names
	for _, tmpl := range []string{"QUALITY_GATE_TESTING.md", "QUALITY_GATE_REVIEW.md", "QUALITY_GATE_ITERATE.md", "QUALITY_GATE_FEST_COMMIT.md"} {
		if err := os.WriteFile(filepath.Join(implGatesDir, tmpl), []byte("# Gate"), 0644); err != nil {
			t.Fatalf("failed to create template: %v", err)
		}
	}

	// Also create core templates
	festivalTemplatesDir := filepath.Join(templatesDir, "festival")
	if err := os.MkdirAll(festivalTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create festival templates dir: %v", err)
	}
	for _, tmpl := range []string{"OVERVIEW.md", "GOAL.md"} {
		if err := os.WriteFile(filepath.Join(festivalTemplatesDir, tmpl), []byte("# Template"), 0644); err != nil {
			t.Fatalf("failed to create template: %v", err)
		}
	}

	// Change working directory temporarily
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(festivalsDir)

	// Run create festival
	opts := &CreateFestivalOptions{
		Name:        "gates-test",
		Goal:        "Test gates configuration",
		SkipMarkers: true,
		Dest:        "active",
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Find the created festival directory (now includes ID suffix)
	activeDir := filepath.Join(festivalsDir, "active")
	entries, err := os.ReadDir(activeDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry in active/: %v", err)
	}
	festivalDir := filepath.Join(activeDir, entries[0].Name())
	festYAMLPath := filepath.Join(festivalDir, "fest.yaml")

	if _, err := os.Stat(festYAMLPath); err != nil {
		t.Fatalf("expected fest.yaml to exist: %v", err)
	}

	// Read and verify content has gates/implementation/ prefix
	content, err := os.ReadFile(festYAMLPath)
	if err != nil {
		t.Fatalf("failed to read fest.yaml: %v", err)
	}
	contentStr := string(content)

	// Check that gates/implementation/ prefix is used in template paths
	if !contains(contentStr, "gates/implementation/QUALITY_GATE_TESTING") {
		t.Error("fest.yaml should contain gates/implementation/QUALITY_GATE_TESTING")
	}
	if !contains(contentStr, "gates/implementation/QUALITY_GATE_REVIEW") {
		t.Error("fest.yaml should contain gates/implementation/QUALITY_GATE_REVIEW")
	}
	if !contains(contentStr, "gates/implementation/QUALITY_GATE_ITERATE") {
		t.Error("fest.yaml should contain gates/implementation/QUALITY_GATE_ITERATE")
	}
	if !contains(contentStr, "gates/implementation/QUALITY_GATE_FEST_COMMIT") {
		t.Error("fest.yaml should contain gates/implementation/QUALITY_GATE_FEST_COMMIT")
	}

	// Verify quality_gates.enabled is true
	if !contains(contentStr, "enabled: true") {
		t.Error("fest.yaml should have quality_gates.enabled: true")
	}
}

// TestCreateFestival_GatesConfigHasCorrectStructure verifies the generated config
func TestCreateFestival_GatesConfigHasCorrectStructure(t *testing.T) {
	cfg := config.DefaultFestivalConfig()

	// Verify quality gates are enabled
	if !cfg.QualityGates.Enabled {
		t.Error("quality gates should be enabled by default")
	}

	// Verify only implementation gates are defined
	if len(cfg.QualityGates.Implementation) != 4 {
		t.Errorf("expected 4 implementation gates, got %d", len(cfg.QualityGates.Implementation))
	}

	// Verify implementation gates use correct template paths
	expectedTemplates := map[string]bool{
		"gates/implementation/QUALITY_GATE_TESTING": false,
		"gates/implementation/QUALITY_GATE_REVIEW":  false,
		"gates/implementation/QUALITY_GATE_ITERATE": false,
		"gates/implementation/QUALITY_GATE_FEST_COMMIT":  false,
	}

	for _, task := range cfg.QualityGates.Implementation {
		if _, ok := expectedTemplates[task.Template]; !ok {
			t.Errorf("unexpected template path: %s", task.Template)
		} else {
			expectedTemplates[task.Template] = true
		}
		if !task.Enabled {
			t.Errorf("expected task %s to be enabled", task.ID)
		}
	}

	for tmpl, found := range expectedTemplates {
		if !found {
			t.Errorf("expected template %s not found in config", tmpl)
		}
	}
}

// TestCreateFestival_WithTypeStandard tests festival creation with --type standard
func TestCreateFestival_WithTypeStandard(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure with templates
	festivalsDir := filepath.Join(tmpDir, "festivals")
	setupFestivalTemplates(t, festivalsDir)

	// Change working directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(festivalsDir)

	// Run create festival with --type standard
	opts := &CreateFestivalOptions{
		Name:        "standard-test",
		Goal:        "Test standard type",
		Type:        "standard",
		SkipMarkers: true,
		Dest:        "active",
		JSONOutput:  false,
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Find created festival directory
	activeDir := filepath.Join(festivalsDir, "active")
	entries, err := os.ReadDir(activeDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry in active/: %v", err)
	}
	festivalDir := filepath.Join(activeDir, entries[0].Name())

	// Verify INGEST and PLAN phases were auto-created
	ingestPhase := filepath.Join(festivalDir, "001_INGEST")
	planPhase := filepath.Join(festivalDir, "002_PLAN")

	if _, err := os.Stat(ingestPhase); err != nil {
		t.Errorf("expected INGEST phase to be auto-created: %v", err)
	}
	if _, err := os.Stat(planPhase); err != nil {
		t.Errorf("expected PLAN phase to be auto-created: %v", err)
	}

	// Verify fest.yaml contains festival type
	festYAMLPath := filepath.Join(festivalDir, "fest.yaml")
	content, err := os.ReadFile(festYAMLPath)
	if err != nil {
		t.Fatalf("failed to read fest.yaml: %v", err)
	}
	if !contains(string(content), "festival_type: standard") {
		t.Error("fest.yaml should contain festival_type: standard")
	}
}

// TestCreateFestival_WithoutType tests backward compatibility (no auto-scaffolding)
func TestCreateFestival_WithoutType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure with templates
	festivalsDir := filepath.Join(tmpDir, "festivals")
	setupFestivalTemplates(t, festivalsDir)

	// Change working directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(festivalsDir)

	// Run create festival WITHOUT --type
	opts := &CreateFestivalOptions{
		Name:        "no-type-test",
		Goal:        "Test without type",
		SkipMarkers: true,
		Dest:        "active",
		JSONOutput:  false,
	}

	err := RunCreateFestival(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	// Find created festival directory
	activeDir := filepath.Join(festivalsDir, "active")
	entries, err := os.ReadDir(activeDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry in active/: %v", err)
	}
	festivalDir := filepath.Join(activeDir, entries[0].Name())

	// Verify NO phases were auto-created
	phases, err := os.ReadDir(festivalDir)
	if err != nil {
		t.Fatalf("failed to read festival dir: %v", err)
	}

	// Count phase directories (should be 0)
	// Phase directories start with 3 digits followed by underscore
	phaseCount := 0
	for _, entry := range phases {
		name := entry.Name()
		// Check if it's a phase directory: starts with 3 digits + underscore
		if entry.IsDir() && len(name) >= 4 && name[0] >= '0' && name[0] <= '9' && name[3] == '_' {
			phaseCount++
		}
	}

	if phaseCount != 0 {
		t.Errorf("expected no auto-created phases, found %d", phaseCount)
	}

	// Verify fest.yaml does NOT contain festival_type
	festYAMLPath := filepath.Join(festivalDir, "fest.yaml")
	content, err := os.ReadFile(festYAMLPath)
	if err != nil {
		t.Fatalf("failed to read fest.yaml: %v", err)
	}
	if contains(string(content), "festival_type:") {
		t.Error("fest.yaml should NOT contain festival_type when not specified")
	}
}

// TestCreateFestival_WithUnknownType tests error handling for unknown types
func TestCreateFestival_WithUnknownType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals directory structure with templates
	festivalsDir := filepath.Join(tmpDir, "festivals")
	setupFestivalTemplates(t, festivalsDir)

	// Change working directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(festivalsDir)

	// Run create festival with unknown type
	opts := &CreateFestivalOptions{
		Name:        "bad-type-test",
		Goal:        "Test bad type",
		Type:        "unknown-type",
		SkipMarkers: true,
		Dest:        "active",
		JSONOutput:  false,
	}

	err := RunCreateFestival(context.Background(), opts)
	if err == nil {
		t.Error("expected error for unknown festival type")
	}
}

// setupFestivalTemplates creates a minimal template structure for testing
func setupFestivalTemplates(t *testing.T, festivalsDir string) {
	t.Helper()

	festivalMetaDir := filepath.Join(festivalsDir, ".festival")
	templatesDir := filepath.Join(festivalMetaDir, "templates")

	// Create core festival templates
	festivalTemplatesDir := filepath.Join(templatesDir, "festival")
	if err := os.MkdirAll(festivalTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create festival templates dir: %v", err)
	}
	for _, tmpl := range []string{"OVERVIEW.md", "GOAL.md", "RULES.md", "TODO.md"} {
		content := "# {{.festival_name}}\n"
		if err := os.WriteFile(filepath.Join(festivalTemplatesDir, tmpl), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create template festival/%s: %v", tmpl, err)
		}
	}

	// Create phase templates for different phase types
	phaseTypes := []string{"planning", "implementation", "research", "ingest"}
	for _, phaseType := range phaseTypes {
		phaseDir := filepath.Join(templatesDir, "phases", phaseType)
		if err := os.MkdirAll(phaseDir, 0755); err != nil {
			t.Fatalf("failed to create phase dir %s: %v", phaseType, err)
		}
		goalContent := "# Phase: {{.phase_name}}\n"
		if err := os.WriteFile(filepath.Join(phaseDir, "GOAL.md"), []byte(goalContent), 0644); err != nil {
			t.Fatalf("failed to create phase goal template: %v", err)
		}
	}

	// Create gates for implementation phase
	implGatesDir := filepath.Join(templatesDir, "phases", "implementation", "gates")
	if err := os.MkdirAll(implGatesDir, 0755); err != nil {
		t.Fatalf("failed to create implementation gates dir: %v", err)
	}
	for _, gate := range []string{"QUALITY_GATE_TESTING.md", "QUALITY_GATE_REVIEW.md", "QUALITY_GATE_ITERATE.md", "QUALITY_GATE_FEST_COMMIT.md"} {
		content := "# Gate\n"
		if err := os.WriteFile(filepath.Join(implGatesDir, gate), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create gate template: %v", err)
		}
	}
}

// contains checks if substr is in s (simple substring check)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
