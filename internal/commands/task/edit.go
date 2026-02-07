package task

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [task]",
		Short: "Open the current task in your editor",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: runEdit,
	}
}

func runEdit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	taskID, taskFilePath, err := resolveTask(ctx, festivalPath, arg)
	if err != nil {
		return err
	}

	editor := resolveEditor(ctx)

	fmt.Printf("%s %s\n", ui.Label("Opening"), ui.Value(taskID, ui.TaskColor))

	editorCmd := exec.CommandContext(ctx, editor, taskFilePath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return errors.Wrap(err, "running editor")
	}

	return nil
}

// resolveEditor returns the configured editor from config, $EDITOR, or "vim" as fallback.
func resolveEditor(ctx context.Context) string {
	cfg, err := config.Load(ctx, "")
	if err == nil && cfg.Behavior.Editor != "" {
		return cfg.Behavior.Editor
	}

	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	return "vim"
}
