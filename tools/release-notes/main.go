package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/tools/release-notes/internal/notes"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("release-notes", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	repo := fs.String("repo", "", "GitHub repository in owner/name form")
	tag := fs.String("tag", "", "tag to render notes for")
	outputPath := fs.String("output", "dist/release-notes.md", "output markdown path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *repo == "" {
		return fmt.Errorf("--repo is required")
	}
	if *tag == "" {
		return fmt.Errorf("--tag is required")
	}

	current, err := notes.ParseTag(*tag)
	if err != nil {
		return err
	}

	targetCommit, err := tagCommit(*tag)
	if err != nil {
		return fmt.Errorf("resolve tag commit: %w", err)
	}

	tags, err := gitLines("tag", "-l", "v*", "--sort=-version:refname")
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	previousTag, err := resolvePreviousTag(current, targetCommit, tags)
	if err != nil {
		return err
	}

	subjects, err := commitSubjects(*tag, previousTag)
	if err != nil {
		return err
	}

	var changes []notes.Change
	seen := map[string]struct{}{}
	for _, subject := range subjects {
		change, ok := notes.ParseCommitSubject(subject)
		if !ok {
			continue
		}
		key := strings.ToLower(change.Text)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		changes = append(changes, change)
	}

	rendered, err := notes.Render(*repo, current, previousTag, changes)
	if err != nil {
		return err
	}

	absOutput := *outputPath
	if !filepath.IsAbs(absOutput) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		absOutput = filepath.Join(cwd, absOutput)
	}

	if err := os.MkdirAll(filepath.Dir(absOutput), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(absOutput, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("Wrote release notes to %s\n", absOutput)
	if previousTag != "" {
		fmt.Printf("Compared %s...%s\n", previousTag, *tag)
	}
	return nil
}

func commitSubjects(tag, previousTag string) ([]string, error) {
	args := []string{"log", "--format=%s"}
	if previousTag != "" {
		args = append(args, previousTag+".."+tag)
	} else {
		args = append(args, tag)
	}
	return gitLines(args...)
}

func resolvePreviousTag(current notes.TagInfo, targetCommit string, tags []string) (string, error) {
	search := current
	seen := map[string]struct{}{current.Raw: {}}

	for {
		candidate := notes.FindPreviousTag(search, tags)
		if candidate == "" {
			return "", nil
		}
		if _, exists := seen[candidate]; exists {
			return "", nil
		}
		seen[candidate] = struct{}{}

		candidateCommit, err := tagCommit(candidate)
		if err != nil {
			return "", fmt.Errorf("resolve previous tag commit %s: %w", candidate, err)
		}
		if candidateCommit != targetCommit {
			return candidate, nil
		}

		search, err = notes.ParseTag(candidate)
		if err != nil {
			return "", err
		}
	}
}

func tagCommit(tag string) (string, error) {
	lines, err := gitLines("rev-list", "-n", "1", tag)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("tag %s has no commit", tag)
	}
	return strings.TrimSpace(lines[0]), nil
}

func gitLines(args ...string) ([]string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
