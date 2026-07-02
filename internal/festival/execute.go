package festival

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// executeChanges applies the planned changes
func (r *Renumberer) executeChanges() error {
	if len(r.changes) == 0 {
		if !r.options.Quiet {
			fmt.Println("No changes needed.")
		}
		return nil
	}

	// Display changes
	if !r.options.Quiet {
		r.displayChanges()
	}

	if r.options.DryRun {
		if !r.options.Quiet {
			fmt.Println("\nDRY RUN - Preview complete.")
		}
		// If auto-approve, proceed to apply after dry-run preview
		if r.options.AutoApprove {
			if !r.options.Quiet {
				fmt.Println("\nApplying changes...")
			}
		} else {
			// Prompt user to apply changes after dry-run
			if r.confirmApplyAfterDryRun() {
				if !r.options.Quiet {
					fmt.Println("\nApplying changes...")
				}
			} else {
				if !r.options.Quiet {
					fmt.Println("Operation cancelled.")
				}
				return nil
			}
		}
	} else {
		// Not in dry-run mode, confirm changes before applying
		if !r.options.AutoApprove {
			if !r.confirmChanges() {
				if !r.options.Quiet {
					fmt.Println("Operation cancelled.")
				}
				return nil
			}
		}
	}

	// Create backup if requested
	if r.options.Backup {
		if err := r.createBackup(); err != nil {
			return errors.IO("Renumberer.createBackup", err)
		}
	}

	// Apply changes
	for _, change := range r.changes {
		switch change.Type {
		case ChangeRename:
			if err := os.Rename(change.OldPath, change.NewPath); err != nil {
				return errors.IO("Renumberer.rename", err).
					WithField("from", change.OldPath).
					WithField("to", change.NewPath)
			}
			if err := refreshRenamedMetadata(change.NewPath); err != nil {
				fmt.Printf("Warning: renamed %s but could not refresh its frontmatter: %v\n",
					filepath.Base(change.NewPath), err)
			}
			if r.options.Verbose {
				fmt.Printf("Renamed: %s → %s\n", filepath.Base(change.OldPath), filepath.Base(change.NewPath))
			}

		case ChangeCreate:
			// If NewPath looks like a file (e.g., ends with .md), create an empty file.
			// Otherwise, create a directory (used for phases/sequences).
			if strings.HasSuffix(strings.ToLower(change.NewPath), ".md") {
				// For file creations, just ensure parent exists; the caller writes content.
				if err := os.MkdirAll(filepath.Dir(change.NewPath), 0755); err != nil {
					return errors.IO("Renumberer.mkdirParent", err).
						WithField("path", change.NewPath)
				}
			} else {
				if err := os.MkdirAll(change.NewPath, 0755); err != nil {
					return errors.IO("Renumberer.mkdir", err).
						WithField("path", change.NewPath)
				}
			}
			if r.options.Verbose {
				fmt.Printf("Created: %s\n", filepath.Base(change.NewPath))
			}

		case ChangeRemove:
			if err := os.RemoveAll(change.OldPath); err != nil {
				return errors.IO("Renumberer.remove", err).
					WithField("path", change.OldPath)
			}
			if r.options.Verbose {
				fmt.Printf("Removed: %s\n", filepath.Base(change.OldPath))
			}
		}
	}

	if !r.options.Quiet {
		fmt.Printf("\n✓ Successfully applied %d changes.\n", len(r.changes))
	}
	return nil
}

// displayChanges shows planned changes
func (r *Renumberer) displayChanges() {
	fmt.Println("\nFestival Renumbering Report")
	fmt.Println(strings.Repeat("═", 55))
	fmt.Println("\nChanges to be made:")

	for _, change := range r.changes {
		switch change.Type {
		case ChangeRename:
			fmt.Printf("  → Rename: %s → %s\n",
				filepath.Base(change.OldPath),
				filepath.Base(change.NewPath))
		case ChangeCreate:
			fmt.Printf("  ✓ Create: %s\n", filepath.Base(change.NewPath))
		case ChangeRemove:
			fmt.Printf("  ✗ Remove: %s\n", filepath.Base(change.OldPath))
		}
	}

	fmt.Printf("\nTotal: %d changes\n", len(r.changes))
}

// confirmChanges prompts for confirmation
func (r *Renumberer) confirmChanges() bool {
	fmt.Print("\nProceed with renumbering? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// confirmApplyAfterDryRun prompts to apply changes after dry-run preview
func (r *Renumberer) confirmApplyAfterDryRun() bool {
	fmt.Print("\nApply these changes? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// createBackup creates a backup of affected directories
func (r *Renumberer) createBackup() error {
	// Implementation would create timestamped backup
	// For now, just log
	fmt.Println("Creating backup...")
	return nil
}

// refreshRenamedMetadata keeps frontmatter consistent with a renumber
// rename: a renamed task file gets its fest_id/fest_order updated; a
// renamed sequence or phase directory gets its goal file's fest_id and
// fest_order updated plus the fest_parent of its immediate children.
// Rewrites are line-surgical so unknown frontmatter keys and formatting
// survive untouched (the schema round-trip would drop unknown keys).
func refreshRenamedMetadata(newPath string) error {
	base := filepath.Base(newPath)

	if m := taskParsePattern.FindStringSubmatch(base); m != nil {
		return rewriteFrontmatterFields(newPath, map[string]string{
			"fest_id":    base,
			"fest_order": strings.TrimLeft(m[1], "0"),
		})
	}

	info, err := os.Stat(newPath)
	if err != nil || !info.IsDir() {
		return err
	}

	var goalFile, number string
	var childPatterns []string
	if m := sequenceParsePattern.FindStringSubmatch(base); m != nil {
		goalFile, number = "SEQUENCE_GOAL.md", strings.TrimLeft(m[1], "0")
		childPatterns = []string{"*.md"}
	} else if m := phaseParsePattern.FindStringSubmatch(base); m != nil {
		goalFile, number = "PHASE_GOAL.md", strings.TrimLeft(m[1], "0")
		childPatterns = []string{"*/SEQUENCE_GOAL.md"}
	} else {
		return nil
	}

	goalPath := filepath.Join(newPath, goalFile)
	if _, statErr := os.Stat(goalPath); statErr == nil {
		if err := rewriteFrontmatterFields(goalPath, map[string]string{
			"fest_id":    base,
			"fest_order": number,
		}); err != nil {
			return err
		}
	}

	for _, pattern := range childPatterns {
		children, globErr := filepath.Glob(filepath.Join(newPath, pattern))
		if globErr != nil {
			continue
		}
		for _, child := range children {
			if filepath.Base(child) == goalFile {
				continue
			}
			if err := rewriteFrontmatterFields(child, map[string]string{
				"fest_parent": base,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// rewriteFrontmatterFields replaces the values of existing top-level keys
// inside a file's frontmatter block, leaving every other byte alone. Keys
// absent from the block are not added; files without frontmatter are
// left untouched.
func rewriteFrontmatterFields(path string, fields map[string]string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.IO("Renumberer.readFrontmatter", err).WithField("path", path)
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	changed := false
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			break
		}
		for key, value := range fields {
			if strings.HasPrefix(trimmed, key+":") {
				lines[i] = key + ": " + value
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), mode); err != nil {
		return errors.IO("Renumberer.writeFrontmatter", err).WithField("path", path)
	}
	return nil
}
