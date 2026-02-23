// Package commit provides the fest commit command for git integration.
package commit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// Commit reference format prefix: OBEY-FE (Obey Festival component)
const commitRefPrefix = "OBEY-FE"

var (
	message   string
	taskRef   string
	noTag     bool
	jsonOut   bool
	autoStage bool
)

// NewCommitCommand creates the fest commit command
func NewCommitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Create git commit with task reference",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Create a git commit with the festival/task ID embedded in the message.

The fest commit command wraps git commit and automatically:
  1. Stages all changes (git add -A) unless --stage=false
  2. Prepends the festival reference to the commit message

Reference format: [OBEY-FE-{id}]
  - OBEY: Obey workflow tool prefix
  - FE: Festival component identifier
  - {id}: Task ref (FEST-xxxxxx) or festival ID (e.g., CS0001)

Detection priority:
  1. Explicit --task flag value
  2. Task fest_ref from current directory (if inside festival task)
  3. Festival ID from fest.yaml metadata

Examples:
  fest commit -m "Implement feature"
  # In linked project → [OBEY-FE-CS0001] Implement feature
  # In festival task  → [OBEY-FE-FEST-a3b2c1] Implement feature

  fest commit --task FEST-b4c5d6 -m "Related work"
  # → [OBEY-FE-FEST-b4c5d6] Related work

  fest commit --no-tag -m "No reference"
  # → No reference

  fest commit --stage=false -m "Only commit staged"
  # Skip auto-staging, commit only what's already staged`,
		RunE: runCommit,
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message")
	cmd.Flags().StringVar(&taskRef, "task", "", "task reference ID to use (e.g., FEST-a3b2c1)")
	cmd.Flags().BoolVar(&noTag, "no-tag", false, "don't prepend task reference")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output result as JSON")
	cmd.Flags().BoolVar(&autoStage, "stage", true, "auto-stage all changes before commit")

	cmd.MarkFlagRequired("message")

	return cmd
}

// CommitResult represents the result of a commit operation
type CommitResult struct {
	Success bool   `json:"success"`
	Hash    string `json:"hash,omitempty"`
	Message string `json:"message"`
	TaskRef string `json:"task_ref,omitempty"`
	Error   string `json:"error,omitempty"`
}

func runCommit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	result := &CommitResult{}

	// Check if we're in a git repository
	inRepo, err := isGitRepo(ctx)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}
	if !inRepo {
		result.Success = false
		result.Error = "not in a git repository"
		return outputResult(result)
	}

	// Auto-stage changes if enabled (default: true)
	if autoStage {
		if err := stageAllChanges(ctx); err != nil {
			result.Success = false
			result.Error = err.Error()
			return outputResult(result)
		}
	}

	// Get task reference
	var ref string
	if !noTag {
		if taskRef != "" {
			// Validate provided ref
			if !id.Validate(taskRef) {
				result.Success = false
				result.Error = fmt.Sprintf("invalid task reference format: %s (expected FEST-xxxxxx)", taskRef)
				return outputResult(result)
			}
			ref = formatCommitRef(taskRef)
		} else {
			// Try to detect from current location (task ref within festival)
			detectedRef, err := detectCurrentTaskRef(ctx)
			if err == nil && detectedRef != "" {
				ref = formatCommitRef(detectedRef)
			} else {
				// Fall back to festival ID from fest.yaml
				festivalID, err := detectFestivalID(ctx)
				if err == nil && festivalID != "" {
					ref = formatCommitRef(festivalID)
				}
			}
		}
	}

	// Build commit message
	commitMessage := message
	if ref != "" {
		commitMessage = fmt.Sprintf("[%s] %s", ref, message)
	}

	// Execute git commit
	hash, err := executeGitCommit(ctx, commitMessage)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}

	result.Success = true
	result.Hash = hash
	result.Message = commitMessage
	result.TaskRef = ref

	return outputResult(result)
}

func outputResult(result *CommitResult) error {
	if jsonOut {
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		if result.Success {
			fmt.Println(ui.H1("Commit"))
			fmt.Printf("%s %s\n", ui.Label("Hash"), ui.Value(result.Hash))
			fmt.Printf("%s %s\n", ui.Label("Message"), highlightTaskRefs(result.Message))
			if result.TaskRef != "" {
				fmt.Printf("%s %s\n", ui.Label("Task"), ui.Value(result.TaskRef, ui.TaskColor))
			}
		} else {
			return errors.New(result.Error)
		}
	}
	return nil
}

func isGitRepo(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, errors.Wrap(ctx.Err(), "context cancelled")
		}
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.Wrap(err, "checking git repository")
	}
	return true, nil
}

func executeGitCommit(ctx context.Context, message string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "git commit failed")
	}

	// Get the commit hash
	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	var out bytes.Buffer
	hashCmd.Stdout = &out
	if err := hashCmd.Run(); err != nil {
		return "", errors.Wrap(err, "failed to get commit hash")
	}

	return strings.TrimSpace(out.String()), nil
}

// stageAllChanges runs git add -A to stage all changes
func stageAllChanges(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "git add failed")
	}
	return nil
}

// formatCommitRef formats an ID with the standard commit reference prefix.
// Format: [OBEY-FE-{id}] where FE = Festival component
func formatCommitRef(id string) string {
	return fmt.Sprintf("%s-%s", commitRefPrefix, id)
}

// detectFestivalID retrieves the festival ID from fest.yaml metadata
func detectFestivalID(ctx context.Context) (string, error) {
	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok || festivalPath == "" {
		return "", errors.NotFound("festival")
	}

	cfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return "", errors.Wrap(err, "loading festival config")
	}

	if cfg.Metadata.ID == "" {
		return "", errors.NotFound("festival ID in metadata")
	}

	return cfg.Metadata.ID, nil
}

func detectCurrentTaskRef(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Get festival path from scope context (resolved by PersistentPreRunE)
	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok || festivalPath == "" {
		return "", errors.NotFound("festival")
	}

	// Check if we're in a task directory
	relPath, err := filepath.Rel(festivalPath, cwd)
	if err != nil {
		return "", err
	}

	// If the path starts with ".." we're outside the festival directory
	// This means we can't detect a task ref - let caller fall back to festival ID
	if strings.HasPrefix(relPath, "..") {
		return "", errors.NotFound("task reference - outside festival directory")
	}

	// Walk up looking for a task file
	parts := strings.Split(relPath, string(os.PathSeparator))
	if len(parts) >= 3 {
		// We might be in sequence/task level
		seqDir := filepath.Join(festivalPath, parts[0], parts[1])
		entries, err := os.ReadDir(seqDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				if strings.HasPrefix(strings.ToUpper(entry.Name()), "SEQUENCE_") {
					continue
				}

				// Try to parse frontmatter from this task
				taskPath := filepath.Join(seqDir, entry.Name())
				content, err := os.ReadFile(taskPath)
				if err != nil {
					continue
				}

				fm, _, err := frontmatter.Parse(content)
				if err != nil || fm == nil {
					continue
				}

				if fm.Ref != "" {
					return fm.Ref, nil
				}
			}
		}
	}

	return "", errors.NotFound("task reference")
}
