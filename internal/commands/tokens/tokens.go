package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/lancekrogers/tcount/tokenizer"
	"github.com/spf13/cobra"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

type tokensOptions struct {
	all   bool
	model string
	json  bool
}

// NewTokensCommand creates the tokens command: planning-corpus token counts
// via tcount, for a festival or the whole festivals workspace.
func NewTokensCommand() *cobra.Command {
	opts := &tokensOptions{}

	cmd := &cobra.Command{
		Use:   "tokens [path]",
		Short: "Count planning-corpus tokens for a festival or the whole workspace",
		Long: `Count tokens in festival planning documents using tcount.

With no arguments, counts the festival containing the current directory.
With --all, counts the entire festivals workspace (every status).
With a path, counts that file or directory directly.

Counting matches the tcount CLI: recursive, .gitignore-aware, and the
reported number is tcount's primary method for the selected model.`,
		Example: `  fest tokens                    # Current festival's planning corpus
  fest tokens --all              # Entire festivals workspace
  fest tokens 001_PLAN           # One phase directory
  fest tokens --all --json       # Machine-readable totals`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			return runTokens(cmd.Context(), target, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.all, "all", false, "count the entire festivals workspace")
	cmd.Flags().StringVar(&opts.model, "model", "", "model whose tokenizer to use (tcount default when empty)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	return cmd
}

func runTokens(ctx context.Context, target string, opts *tokensOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := resolveTarget(target, opts.all)
	if err != nil {
		return err
	}

	counter, err := tokenizer.NewCounter(tokenizer.CounterOptions{})
	if err != nil {
		return errors.Wrap(err, "initializing tokenizer")
	}

	info, err := os.Stat(path)
	if err != nil {
		return errors.IO("reading count target", err).WithField("path", path)
	}
	var res *tokenizer.CountResult
	if info.IsDir() {
		res, err = counter.CountDirectory(ctx, path, opts.model, false)
	} else {
		res, err = counter.CountFile(ctx, path, opts.model, false)
	}
	if err != nil {
		return errors.Wrap(err, "counting tokens").WithField("path", path)
	}
	if len(res.Methods) == 0 {
		return errors.New("tokenizer returned no methods").WithField("path", path)
	}

	// Mirror the tcount CLI --tokens contract: the primary method is the number.
	primary := res.Methods[0]
	if opts.json {
		return outputJSON(map[string]interface{}{
			"path":         path,
			"tokens":       primary.Tokens,
			"method":       primary.Name,
			"display_name": primary.DisplayName,
			"is_exact":     primary.IsExact,
			"file_count":   res.FileCount,
			"characters":   res.Characters,
			"words":        res.Words,
			"lines":        res.Lines,
		})
	}

	files := ""
	if res.FileCount > 0 {
		files = fmt.Sprintf(" across %s files", groupDigits(res.FileCount))
	}
	exactness := "approximate"
	if primary.IsExact {
		exactness = "exact"
	}
	fmt.Printf("%s tokens%s · %s (%s)\n%s\n",
		groupDigits(primary.Tokens), files, primary.DisplayName, exactness, path)
	return nil
}

// resolveTarget picks what to count: an explicit path, the whole festivals
// workspace, or the festival containing the current directory.
func resolveTarget(target string, all bool) (string, error) {
	if target != "" && all {
		return "", errors.Validation("pass a path or --all, not both")
	}
	if target != "" {
		return target, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.IO("getting current directory", err)
	}
	if all {
		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil || festivalsDir == "" {
			return "", errors.NotFound("festivals workspace").
				WithHint("run from a project with a festivals/ directory or pass a path")
		}
		return festivalsDir, nil
	}
	festivalRoot, err := template.FindFestivalRoot(cwd)
	if err != nil || festivalRoot == "" {
		return "", errors.NotFound("festival").WithOp("tokens").
			WithHint("run inside a festival, pass a path, or use --all for the whole workspace")
	}
	return festivalRoot, nil
}

// groupDigits renders n with thousands separators (1234567 -> 1,234,567).
func groupDigits(n int) string {
	s := strconv.Itoa(n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func outputJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
