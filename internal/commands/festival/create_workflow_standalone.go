package festival

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"golang.org/x/term"
)

// runStandaloneCreateWorkflow implements `fest create workflow <name>` outside
// a festival context (D009). It writes <cwd>/WORKFLOW.md from the same
// WorkflowInput shape festival mode uses, then optionally initializes
// <cwd>/.workflow/workflow.yaml via localstore.Init.
//
// Festival-only flags (--position, --path, --festival) are rejected here so
// users get an explicit error rather than confusing silent ignores.
func runStandaloneCreateWorkflow(ctx context.Context, opts *CreateWorkflowOptions, res *standalone.Result, cwd string) error {
	if err := rejectFestivalOnlyFlags(opts); err != nil {
		return emitWorkflowError(opts, err)
	}

	if opts.Name == "" {
		return emitWorkflowError(opts,
			errors.Validation("workflow name required").
				WithHint("usage: fest create workflow <name>"))
	}

	input, err := buildStandaloneWorkflowInput(ctx, opts)
	if err != nil {
		return emitWorkflowError(opts, err)
	}
	if err := validateWorkflowInput(input); err != nil {
		return emitWorkflowError(opts, err)
	}

	target := filepath.Join(cwd, "WORKFLOW.md")
	if _, err := os.Stat(target); err == nil {
		return emitWorkflowError(opts,
			errors.Validation("WORKFLOW.md already exists").
				WithField("path", target).
				WithHint("remove it first or pick another directory"))
	}

	workflowID := "wf-" + workflow.SanitizeBasenameAsSlug(opts.Name)
	if err := workflow.ValidateWorkflowID(workflowID); err != nil {
		return emitWorkflowError(opts, err)
	}

	content := renderWorkflowContent(input, workflowID, "after")
	if err := atomicWriteWorkflowFile(target, []byte(content), 0o644); err != nil {
		return emitWorkflowError(opts, err)
	}

	if !opts.JSONOutput {
		fmt.Printf("created %s\n", target)
	}

	if !opts.NoInit {
		store := localstore.Open(filepath.Join(cwd, ".workflow"), target)
		if err := store.Init(ctx, localstore.InitOptions{WorkflowID: workflowID}); err != nil {
			return emitWorkflowError(opts, errors.Wrap(err, "initialize .workflow/"))
		}
		if !opts.JSONOutput {
			fmt.Printf("initialized .workflow/workflow.yaml (workflow_id=%s)\n", workflowID)
		}
	} else if !opts.JSONOutput {
		fmt.Println("skipped .workflow/ init (--no-init); run `fest workflow init` when ready")
	}

	if !opts.JSONOutput {
		fmt.Println("next: fest next")
	}
	_ = res
	return nil
}

// rejectFestivalOnlyFlags returns a validation error when the caller passed a
// flag that only makes sense inside a festival. Closes D009 task 06.
func rejectFestivalOnlyFlags(opts *CreateWorkflowOptions) error {
	if opts.Festival != "" {
		return errors.Validation("--festival is not valid in standalone mode").
			WithHint("run inside a festival to use --festival, or omit the flag")
	}
	if opts.Path != "" && opts.Path != "." {
		return errors.Validation("--path is not valid in standalone mode").
			WithField("path", opts.Path).
			WithHint("standalone mode writes to the current directory")
	}
	return nil
}

// buildStandaloneWorkflowInput accepts inline --steps / --steps-file or
// prompts on a TTY when neither is provided. Non-TTY callers without flags
// get an explicit error rather than a silent skeleton (D009 task 05).
func buildStandaloneWorkflowInput(ctx context.Context, opts *CreateWorkflowOptions) (*WorkflowInput, error) {
	if opts.Steps != "" || opts.StepsFile != "" {
		input, err := parseWorkflowInput(opts)
		if err != nil {
			return nil, err
		}
		if input.Title == "" {
			input.Title = opts.Name
		}
		return input, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.Validation("steps required in non-interactive mode").
			WithHint("pass --steps '{...}' or --steps-file path.json")
	}
	return promptForStandaloneInput(ctx, os.Stdin, os.Stdout, opts.Name)
}

// promptForStandaloneInput walks the user through title + intent + step
// prompts. ctx cancellation aborts cleanly with no partial files since the
// caller has not started writing yet. Each prompt loop checks ctx.Err()
// before reading.
func promptForStandaloneInput(ctx context.Context, in io.Reader, out io.Writer, name string) (*WorkflowInput, error) {
	r := bufio.NewReader(in)

	title, err := promptLine(ctx, r, out, fmt.Sprintf("title [%s]: ", name))
	if err != nil {
		return nil, err
	}
	if title == "" {
		title = name
	}
	intent, err := promptLine(ctx, r, out, "intent (one line): ")
	if err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintln(out, "steps (Name|Goal per line, empty line ends):"); err != nil {
		return nil, errors.IO("write prompt", err)
	}
	var steps []WorkflowStepInput
	for {
		s, err := promptLine(ctx, r, out, "  - ")
		if err != nil {
			return nil, err
		}
		if s == "" {
			break
		}
		parts := strings.SplitN(s, "|", 2)
		step := WorkflowStepInput{Name: strings.TrimSpace(parts[0])}
		if len(parts) == 2 {
			step.Goal = strings.TrimSpace(parts[1])
		} else {
			step.Goal = "Describe this step."
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, errors.Validation("at least one step required").
			WithHint("rerun and enter steps interactively or pass --steps")
	}
	return &WorkflowInput{
		Title:       title,
		Description: intent,
		Steps:       steps,
	}, nil
}

// promptLine prints prompt to out, reads a line from r, and trims trailing
// whitespace. Honors ctx by checking before the (blocking) read; users can
// Ctrl+C to interrupt the read at the OS level.
func promptLine(ctx context.Context, r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", errors.IO("write prompt", err)
	}
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", errors.IO("read prompt", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// atomicWriteWorkflowFile writes data via tmp+rename so a crash mid-write
// leaves the original (or no file) instead of a half-formed WORKFLOW.md.
func atomicWriteWorkflowFile(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return errors.IO("write tmp WORKFLOW.md", err).WithField("path", tmp)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return errors.IO("rename WORKFLOW.md", err).WithField("path", path)
	}
	return nil
}
