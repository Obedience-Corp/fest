package notes

import (
	"strings"
	"testing"
)

func TestParseTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag       string
		mode      ReleaseMode
		iteration int
	}{
		{tag: "v0.2.3", mode: ModeStable},
		{tag: "v0.2.3-rc.2", mode: ModeRC, iteration: 2},
		{tag: "v0.2.3-dev.4", mode: ModeDev, iteration: 4},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			info, err := ParseTag(tt.tag)
			if err != nil {
				t.Fatalf("ParseTag() error = %v", err)
			}
			if info.Mode != tt.mode {
				t.Fatalf("ParseTag().Mode = %q, want %q", info.Mode, tt.mode)
			}
			if info.Iteration != tt.iteration {
				t.Fatalf("ParseTag().Iteration = %d, want %d", info.Iteration, tt.iteration)
			}
		})
	}
}

func TestFindPreviousTag(t *testing.T) {
	t.Parallel()

	tags := []string{
		"v0.3.0-dev.2",
		"v0.3.0-dev.1",
		"v0.2.1-rc.2",
		"v0.2.1-rc.1",
		"v0.2.0",
		"v0.1.5",
	}

	stable, _ := ParseTag("v0.2.0")
	if got := FindPreviousTag(stable, tags); got != "v0.1.5" {
		t.Fatalf("FindPreviousTag(stable) = %q, want %q", got, "v0.1.5")
	}

	rc, _ := ParseTag("v0.2.1-rc.2")
	if got := FindPreviousTag(rc, tags); got != "v0.2.1-rc.1" {
		t.Fatalf("FindPreviousTag(rc) = %q, want %q", got, "v0.2.1-rc.1")
	}

	dev, _ := ParseTag("v0.3.0-dev.1")
	if got := FindPreviousTag(dev, tags); got != "v0.2.0" {
		t.Fatalf("FindPreviousTag(dev) = %q, want %q", got, "v0.2.0")
	}
}

func TestParseCommitSubject(t *testing.T) {
	t.Parallel()

	change, ok := ParseCommitSubject("[OBEY-123] fix(docs): correct onboarding guidance (#146)")
	if !ok {
		t.Fatal("ParseCommitSubject() returned ok=false")
	}
	if change.Text != "Correct onboarding guidance" {
		t.Fatalf("ParseCommitSubject().Text = %q, want %q", change.Text, "Correct onboarding guidance")
	}
	if change.PRNumber != 146 {
		t.Fatalf("ParseCommitSubject().PRNumber = %d, want 146", change.PRNumber)
	}
	if change.Category != CategoryDocs {
		t.Fatalf("ParseCommitSubject().Category = %q, want %q", change.Category, CategoryDocs)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("v0.2.1")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	rendered, err := Render("Obedience-Corp/fest", tag, "v0.1.1", []Change{
		{Text: "Wire plugin dispatch into command execution", PRNumber: 147, Category: CategoryFeature},
		{Text: "Use yellow for ready fgo completions", PRNumber: 148, Category: CategoryFix},
		{Text: "Correct fest onboarding guidance", PRNumber: 146, Category: CategoryDocs},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	wantSubstrings := []string{
		"## Release v0.2.1",
		"Compare: https://github.com/Obedience-Corp/fest/compare/v0.1.1...v0.2.1",
		"## Highlights",
		"## Features",
		"## Fixes",
		"## Documentation",
		"Wire plugin dispatch into command execution ([#147](https://github.com/Obedience-Corp/fest/pull/147))",
	}

	for _, want := range wantSubstrings {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing substring %q\n%s", want, rendered)
		}
	}
}

func TestRenderDeduplicatesChangesByText(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("v0.2.1")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	rendered, err := Render("Obedience-Corp/fest", tag, "v0.1.1", []Change{
		{Text: "Wire plugin dispatch into command execution", Category: CategoryFeature},
		{Text: "Wire plugin dispatch into command execution", PRNumber: 147, Category: CategoryFeature},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "Wire plugin dispatch into command execution ([#147](https://github.com/Obedience-Corp/fest/pull/147))"
	if strings.Count(rendered, want) != 1 {
		t.Fatalf("Render() should keep one PR-linked entry, got:\n%s", rendered)
	}
	if strings.Count(rendered, "Wire plugin dispatch into command execution") != 1 {
		t.Fatalf("Render() should suppress duplicate section entries when only one user-facing change exists, got:\n%s", rendered)
	}
}
