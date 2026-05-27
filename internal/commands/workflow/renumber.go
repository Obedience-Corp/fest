package workflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

const defaultWorkflowFile = "WORKFLOW.md"

type renumberPlan struct {
	headings []renumberHeading
	changes  int
}

type renumberHeading struct {
	line      int
	numStart  int
	numEnd    int
	oldNumber int
	newNumber int
	suffix    string
}

func newRenumberCmd() *cobra.Command {
	var (
		dryRun     bool
		skipDryRun bool
		backup     bool
	)

	cmd := &cobra.Command{
		Use:   "renumber [path]",
		Short: "Renumber step headings in a WORKFLOW.md to a contiguous 1-indexed sequence",
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		Long: `Renumber the "## Step N:" headings inside a WORKFLOW.md so they form a
contiguous 1-indexed sequence (Step 1, Step 2, ..., Step K) in document order.

Only the heading lines are touched. Step body text, the Workflow State Tracking
table, and any other markdown content are left exactly as written.

The default target is ./WORKFLOW.md. Pass an explicit path to target another file.

Like other renumber commands, --dry-run is the default so you can preview the
plan; pass --skip-dry-run to write changes immediately.

Known v1 limitations:
  - Cross-references inside step bodies (for example "see Step 3") are not
    rewritten. Update those references by hand after renumbering if needed.

Examples:
  fest workflow renumber                              # preview ./WORKFLOW.md
  fest workflow renumber ./phases/001_PLAN/WORKFLOW.md
  fest workflow renumber --skip-dry-run               # apply changes
  fest workflow renumber --skip-dry-run --backup      # write .bak alongside`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipDryRun {
				dryRun = false
			}

			path := defaultWorkflowFile
			if len(args) > 0 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return errors.Wrap(err, "resolving path").WithField("path", path)
			}

			return runWorkflowRenumber(cmd.OutOrStdout(), absPath, dryRun, backup)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "preview changes without writing")
	cmd.Flags().BoolVar(&skipDryRun, "skip-dry-run", false, "skip preview and apply changes immediately")
	cmd.Flags().BoolVar(&backup, "backup", false, "write a .bak copy of the original before applying")

	return cmd
}

func runWorkflowRenumber(out io.Writer, path string, dryRun, backup bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NotFound(fmt.Sprintf("WORKFLOW.md at %s", path)).
				WithField("path", path).
				WithHint("Pass an explicit path or cd into a directory containing WORKFLOW.md")
		}
		return errors.IO("reading workflow file", err).WithField("path", path)
	}

	plan, planErr := buildRenumberPlan(string(content))
	if planErr != nil {
		return planErr.WithField("path", path)
	}

	if len(plan.headings) == 0 {
		return errors.Validation("no '## Step N:' headings found, nothing to renumber").
			WithField("path", path).
			WithHint("Confirm the file contains step headings in the expected format")
	}

	if plan.changes == 0 {
		_, _ = fmt.Fprintf(out, "nothing to renumber: %d step(s) already in contiguous 1-indexed order\n", len(plan.headings))
		return nil
	}

	printPlan(out, path, plan, dryRun)

	if dryRun {
		_, _ = fmt.Fprintln(out, "\n(dry-run) no changes written. Pass --skip-dry-run to apply.")
		return nil
	}

	if backup {
		if err := os.WriteFile(path+".bak", content, 0o644); err != nil {
			return errors.IO("writing backup", err).WithField("path", path+".bak")
		}
	}

	updated := applyRenumber(string(content), plan)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return errors.IO("writing workflow file", err).WithField("path", path)
	}

	_, _ = fmt.Fprintf(out, "\nrenumbered %d heading(s) in %s\n", plan.changes, path)
	return nil
}

func buildRenumberPlan(content string) (renumberPlan, *errors.Error) {
	re := wf.StepHeaderRegexp()
	matches := re.FindAllStringSubmatchIndex(content, -1)

	plan := renumberPlan{}
	if len(matches) == 0 {
		return plan, nil
	}

	seen := map[int][]int{}
	plan.headings = make([]renumberHeading, 0, len(matches))

	for i, m := range matches {
		numStr := content[m[2]:m[3]]
		num, convErr := strconv.Atoi(numStr)
		if convErr != nil {
			return renumberPlan{}, errors.Parse("invalid step number", convErr).
				WithField("value", numStr)
		}

		line := lineNumber(content, m[0])
		suffix := content[m[3]:m[1]]

		h := renumberHeading{
			line:      line,
			numStart:  m[2],
			numEnd:    m[3],
			oldNumber: num,
			newNumber: i + 1,
			suffix:    suffix,
		}
		plan.headings = append(plan.headings, h)
		seen[num] = append(seen[num], line)

		if h.oldNumber != h.newNumber {
			plan.changes++
		}
	}

	var dupes []duplicate
	for num, lines := range seen {
		if len(lines) > 1 {
			dupes = append(dupes, duplicate{number: num, lines: lines})
		}
	}
	if len(dupes) > 0 {
		return renumberPlan{}, formatDuplicateError(dupes)
	}

	return plan, nil
}

type duplicate struct {
	number int
	lines  []int
}

func formatDuplicateError(dupes []duplicate) *errors.Error {
	msg := "duplicate step numbers detected; refusing to renumber"
	e := errors.Validation(msg).WithHint("Resolve duplicates manually before renumbering. Auto-merging is ambiguous.")
	for _, d := range dupes {
		key := fmt.Sprintf("Step %d", d.number)
		e = e.WithField(key, formatLines(d.lines))
	}
	return e
}

func formatLines(lines []int) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("line %d", l)
	}
	return out
}

func lineNumber(content string, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	n := 1
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			n++
		}
	}
	return n
}

func applyRenumber(content string, plan renumberPlan) string {
	if len(plan.headings) == 0 {
		return content
	}
	out := make([]byte, 0, len(content)+len(plan.headings)*2)
	cursor := 0
	for _, h := range plan.headings {
		out = append(out, content[cursor:h.numStart]...)
		out = append(out, strconv.Itoa(h.newNumber)...)
		cursor = h.numEnd
	}
	out = append(out, content[cursor:]...)
	return string(out)
}

func printPlan(out io.Writer, path string, plan renumberPlan, dryRun bool) {
	label := "Plan"
	if dryRun {
		label = "Dry-run plan"
	}
	_, _ = fmt.Fprintf(out, "%s for %s:\n", label, path)
	for _, h := range plan.headings {
		marker := "  "
		if h.oldNumber != h.newNumber {
			marker = "* "
		}
		_, _ = fmt.Fprintf(out, "%sline %d: Step %d -> Step %d%s\n", marker, h.line, h.oldNumber, h.newNumber, h.suffix)
	}
}
