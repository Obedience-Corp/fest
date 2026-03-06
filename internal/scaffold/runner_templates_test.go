package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerCreatesOverview(t *testing.T) {
	ctx := context.Background()
	parser := NewPlanParser()
	plan, err := parser.Parse(ctx, fc0003Structure)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "overview-test")
	runner := NewRunner(RunnerOptions{FestivalDir: destDir})

	_, err = runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify FESTIVAL_OVERVIEW.md exists
	overviewPath := filepath.Join(destDir, "FESTIVAL_OVERVIEW.md")
	overviewContent, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatalf("reading FESTIVAL_OVERVIEW.md: %v", err)
	}
	content := string(overviewContent)

	// Should have frontmatter
	if !strings.HasPrefix(content, "---") {
		t.Error("FESTIVAL_OVERVIEW.md should start with frontmatter")
	}
	if !strings.Contains(content, "fest_type: festival") {
		t.Error("FESTIVAL_OVERVIEW.md should contain festival frontmatter")
	}

	// Should contain plan phase names
	if !strings.Contains(content, "INGEST") {
		t.Error("FESTIVAL_OVERVIEW.md should contain INGEST phase")
	}
	if !strings.Contains(content, "IMPLEMENT") {
		t.Error("FESTIVAL_OVERVIEW.md should contain IMPLEMENT phase")
	}
	if !strings.Contains(content, "REVIEW") {
		t.Error("FESTIVAL_OVERVIEW.md should contain REVIEW phase")
	}

	// Should have standard sections
	if !strings.Contains(content, "Problem Statement") {
		t.Error("FESTIVAL_OVERVIEW.md should contain Problem Statement section")
	}
	if !strings.Contains(content, "Planned Phases") {
		t.Error("FESTIVAL_OVERVIEW.md should contain Planned Phases section")
	}
}

func TestRunnerOverviewDryRun(t *testing.T) {
	ctx := context.Background()
	plan := &ParsedPlan{
		FestivalName: "dry-overview",
		Goal:         "Test goal",
		Phases:       []ParsedPhase{{Number: 1, Name: "BUILD", Type: "implementation"}},
	}

	destDir := filepath.Join(t.TempDir(), "dry-overview")
	runner := NewRunner(RunnerOptions{
		FestivalDir: destDir,
		DryRun:      true,
	})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// FESTIVAL_OVERVIEW.md should be in FilesCreated
	found := false
	for _, f := range result.FilesCreated {
		if strings.HasSuffix(f, "FESTIVAL_OVERVIEW.md") {
			found = true
			break
		}
	}
	if !found {
		t.Error("FESTIVAL_OVERVIEW.md should be listed in FilesCreated during dry run")
	}

	// But should not exist on disk
	if _, err := os.Stat(filepath.Join(destDir, "FESTIVAL_OVERVIEW.md")); !os.IsNotExist(err) {
		t.Error("dry run should not create FESTIVAL_OVERVIEW.md on disk")
	}
}

func TestRunnerWithTemplateRoot(t *testing.T) {
	ctx := context.Background()

	// Set up a temp directory with minimal templates
	tmplRoot := filepath.Join(t.TempDir(), "templates")
	_ = os.MkdirAll(filepath.Join(tmplRoot, "festival"), 0755)
	_ = os.MkdirAll(filepath.Join(tmplRoot, "phases", "implementation"), 0755)
	_ = os.MkdirAll(filepath.Join(tmplRoot, "sequences"), 0755)
	_ = os.MkdirAll(filepath.Join(tmplRoot, "tasks"), 0755)

	// Write minimal templates (no YAML frontmatter, just content)
	_ = os.WriteFile(filepath.Join(tmplRoot, "festival", "GOAL.md"),
		[]byte("# Goal: {{ .festival_name }}\n\nTemplate-rendered goal.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmplRoot, "festival", "OVERVIEW.md"),
		[]byte("# Overview: {{ .festival_name }}\n\nTemplate-rendered overview.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmplRoot, "phases", "implementation", "GOAL.md"),
		[]byte("# Phase: {{ .phase_name }}\n\nPhase type: {{ .phase_type }}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmplRoot, "sequences", "GOAL.md"),
		[]byte("# Sequence: {{ .sequence_name }}\n\nTemplate-rendered sequence.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmplRoot, "tasks", "TASK.md"),
		[]byte("# Task: {{ .task_name }}\n\nTemplate-rendered task.\n"), 0644)

	plan := &ParsedPlan{
		FestivalName: "template-test",
		Goal:         "Test templates",
		Phases: []ParsedPhase{
			{
				Number: 1, Name: "BUILD", Type: "implementation",
				Sequences: []ParsedSequence{
					{
						Number: 1, Name: "core",
						Tasks: []ParsedTask{{Number: 1, Name: "setup"}},
					},
				},
			},
		},
	}

	destDir := filepath.Join(t.TempDir(), "tmpl-fest")
	runner := NewRunner(RunnerOptions{
		FestivalDir:  destDir,
		TemplateRoot: tmplRoot,
	})

	_, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify FESTIVAL_GOAL.md used the template
	goalContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goalContent), "Template-rendered goal") {
		t.Error("FESTIVAL_GOAL.md should use template content")
	}
	if !strings.Contains(string(goalContent), "template-test") {
		t.Error("FESTIVAL_GOAL.md should have festival name rendered")
	}

	// Verify FESTIVAL_OVERVIEW.md used the template
	overviewContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_OVERVIEW.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overviewContent), "Template-rendered overview") {
		t.Error("FESTIVAL_OVERVIEW.md should use template content")
	}

	// Verify PHASE_GOAL.md used the phase-type-specific template
	phaseContent, err := os.ReadFile(filepath.Join(destDir, "001_BUILD/PHASE_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(phaseContent), "Phase type: implementation") {
		t.Error("PHASE_GOAL.md should use phase-type template with rendered variables")
	}

	// Verify SEQUENCE_GOAL.md used the template
	seqContent, err := os.ReadFile(filepath.Join(destDir, "001_BUILD/01_core/SEQUENCE_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seqContent), "Template-rendered sequence") {
		t.Error("SEQUENCE_GOAL.md should use template content")
	}

	// Verify task file used the template
	taskFiles, _ := filepath.Glob(filepath.Join(destDir, "001_BUILD/01_core/01_setup.md"))
	if len(taskFiles) != 1 {
		t.Fatal("expected task file to exist")
	}
	taskContent, err := os.ReadFile(taskFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(taskContent), "Template-rendered task") {
		t.Error("task file should use template content")
	}
}

func TestRunnerTemplateFallback(t *testing.T) {
	ctx := context.Background()

	// Provide a TemplateRoot with only partial templates
	tmplRoot := filepath.Join(t.TempDir(), "partial-templates")
	_ = os.MkdirAll(filepath.Join(tmplRoot, "festival"), 0755)
	// Only write GOAL.md, NOT OVERVIEW.md
	_ = os.WriteFile(filepath.Join(tmplRoot, "festival", "GOAL.md"),
		[]byte("# Custom Goal\n\nFrom template.\n"), 0644)

	plan := &ParsedPlan{
		FestivalName: "fallback-test",
		Goal:         "Test fallback",
		Phases:       []ParsedPhase{{Number: 1, Name: "WORK", Type: "implementation"}},
	}

	destDir := filepath.Join(t.TempDir(), "fallback-fest")
	runner := NewRunner(RunnerOptions{
		FestivalDir:  destDir,
		TemplateRoot: tmplRoot,
	})

	_, err := runner.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// FESTIVAL_GOAL.md should use template
	goalContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goalContent), "From template") {
		t.Error("FESTIVAL_GOAL.md should use template when available")
	}

	// FESTIVAL_OVERVIEW.md should fall back to generated content
	overviewContent, err := os.ReadFile(filepath.Join(destDir, "FESTIVAL_OVERVIEW.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overviewContent), "Problem Statement") {
		t.Error("FESTIVAL_OVERVIEW.md should use fallback content when template missing")
	}
	if !strings.Contains(string(overviewContent), "WORK") {
		t.Error("FESTIVAL_OVERVIEW.md fallback should include phase names")
	}

	// PHASE_GOAL.md should fall back (no phases/implementation/GOAL.md in partial templates)
	phaseContent, err := os.ReadFile(filepath.Join(destDir, "001_WORK/PHASE_GOAL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(phaseContent), "Phase Goal") {
		t.Error("PHASE_GOAL.md should use fallback when template missing")
	}
}

func TestStripTemplateFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantSub string // substring that must appear
	}{
		{
			name:    "no frontmatter",
			input:   "# Hello\n\nContent here.\n",
			want:    "# Hello\n\nContent here.\n",
			wantSub: "Hello",
		},
		{
			name:    "with frontmatter",
			input:   "---\nid: test\ntags: []\n---\n# Content\n\nBody.\n",
			wantSub: "# Content",
		},
		{
			name:    "unclosed frontmatter",
			input:   "---\nid: test\n# Content\n",
			want:    "---\nid: test\n# Content\n",
			wantSub: "---",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripTemplateFrontmatter(tc.input)
			if tc.want != "" && got != tc.want {
				t.Errorf("stripTemplateFrontmatter() = %q, want %q", got, tc.want)
			}
			if tc.wantSub != "" && !strings.Contains(got, tc.wantSub) {
				t.Errorf("stripTemplateFrontmatter() = %q, want substring %q", got, tc.wantSub)
			}
		})
	}
}
