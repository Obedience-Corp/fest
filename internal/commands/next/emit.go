// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
)

// emitNextResult renders a resolved next-step result in whichever output mode
// the flags selected. Every festival-backed path through fest next ends here,
// so a task, a planning step, and a complete festival all honor the same
// output contracts.
func emitNextResult(ctx context.Context, festivalPath string, result *selection.NextTaskResult, opts RenderOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if opts.ProjectDir {
		if result.WorkingDirAbsolute == "" {
			return errors.NotFound("no fest_working_dir set for current sequence")
		}
		fmt.Println(result.WorkingDirAbsolute)
		return nil
	}

	if opts.Path {
		if result.Task == nil {
			return errors.NotFound("no task available")
		}
		relPath := filepath.Join(result.Task.PhaseName, result.Task.SequenceName, result.Task.Name+".md")
		fmt.Println(relPath)
		return nil
	}

	if opts.CD {
		output := selection.FormatCD(result)
		if output == "" {
			return errors.NotFound("no task available to navigate to")
		}
		fmt.Println(output)
		return nil
	}

	if opts.Short {
		fmt.Println(selection.FormatShort(result))
		return nil
	}

	if opts.JSON {
		output, err := selection.FormatJSON(result)
		if err != nil {
			return errors.Parse("formatting JSON", err)
		}
		fmt.Println(output)
		return nil
	}

	if opts.Verbose {
		fmt.Print(selection.FormatVerbose(result, opts.showInlineContext()))
	} else {
		fmt.Print(selection.FormatText(result, opts.showInlineContext()))
	}
	printChainContext(ctx, festivalPath, result.FestivalComplete)
	printFeedbackReminder(ctx, festivalPath)
	return nil
}
