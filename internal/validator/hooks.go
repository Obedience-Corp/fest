package validator

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	guidancewf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

// CodeHooksUndeclaredBinding warns when a step binds a hook name not declared.
const CodeHooksUndeclaredBinding = "hooks_undeclared_binding"

// CodeHooksConfigError warns when hook configuration cannot be loaded or resolved.
const CodeHooksConfigError = "hooks_config_error"

// CodeHooksStartBindingPlacement warns when hooks.start bindings appear on a
// document where the task_start verb never fires.
const CodeHooksStartBindingPlacement = "hooks_start_binding_placement"

// IssuesForUndeclaredBindings builds non-blocking warnings for undeclared bound names.
func IssuesForUndeclaredBindings(path string, planned []hooks.PlannedHook) []Issue {
	var issues []Issue
	for _, p := range planned {
		if p.Skip != hooks.SkipUndeclared {
			continue
		}
		issues = append(issues, Issue{
			Level:   LevelWarning,
			Code:    CodeHooksUndeclaredBinding,
			Path:    path,
			Message: "hook binding references undeclared name " + p.Name + " (skipped)",
			Fix:     "Declare the hook under hooks.definitions or remove the binding",
		})
	}
	return issues
}

// ScanUndeclaredBindings walks the festival's workflow docs and step/goal
// frontmatter and warns for hook bindings that reference undeclared names
// (spec 03: undeclared bindings skip with a warning in fest validate).
func ScanUndeclaredBindings(ctx context.Context, festivalPath string, eff *hooks.Effective) []Issue {
	if eff == nil {
		return nil
	}
	var issues []Issue
	_ = filepath.WalkDir(festivalPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return fs.SkipAll
		}
		if d.IsDir() {
			name := d.Name()
			if path != festivalPath && (strings.HasPrefix(name, ".") || name == "results" || name == "feedback") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(festivalPath, path)
		if relErr != nil {
			rel = path
		}
		switch d.Name() {
		case "GATES.md", "WORKFLOW.md":
			issues = append(issues, scanWorkflowDocBindings(ctx, path, rel, d.Name(), eff)...)
		default:
			issues = append(issues, scanFrontmatterBindings(path, rel, eff)...)
		}
		return nil
	})
	return issues
}

func scanWorkflowDocBindings(ctx context.Context, path, rel, docName string, eff *hooks.Effective) []Issue {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	steps, err := guidancewf.NewParser().ParseContent(ctx, string(content))
	if err != nil {
		return nil
	}
	level := hooks.LevelPhase
	if docName == "GATES.md" {
		level = hooks.LevelGate
	}
	var issues []Issue
	for _, step := range steps {
		if len(step.Hooks.Pre) == 0 && len(step.Hooks.Post) == 0 {
			continue
		}
		planned := eff.PlanBindings(level, step.Hooks.Pre, step.Hooks.Post)
		issues = append(issues, IssuesForUndeclaredBindings(rel, planned)...)
	}
	return issues
}

func scanFrontmatterBindings(path, rel string, eff *hooks.Effective) []Issue {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return nil
	}
	startPre, startPost := fm.Hooks.StartBindings()
	if len(fm.Hooks.Pre) == 0 && len(fm.Hooks.Post) == 0 &&
		len(startPre) == 0 && len(startPost) == 0 {
		return nil
	}
	level := hooks.LevelTask
	goalDoc := false
	switch filepath.Base(path) {
	case "SEQUENCE_GOAL.md":
		level = hooks.LevelSequence
		goalDoc = true
	case "PHASE_GOAL.md", "FESTIVAL_GOAL.md":
		level = hooks.LevelPhase
		goalDoc = true
	}
	var issues []Issue
	if len(fm.Hooks.Pre) > 0 || len(fm.Hooks.Post) > 0 {
		planned := eff.PlanBindings(level, fm.Hooks.Pre, fm.Hooks.Post)
		issues = append(issues, IssuesForUndeclaredBindings(rel, planned)...)
	}
	if len(startPre) > 0 || len(startPost) > 0 {
		if goalDoc {
			issues = append(issues, Issue{
				Level:   LevelWarning,
				Code:    CodeHooksStartBindingPlacement,
				Path:    rel,
				Message: "hooks.start bindings only run on task documents (task_start never fires here)",
				Fix:     "Move the start bindings to a task document or remove them",
			})
		} else {
			planned := eff.PlanBindings(hooks.LevelTask, startPre, startPost)
			issues = append(issues, IssuesForUndeclaredBindings(rel, planned)...)
		}
	}
	return issues
}
