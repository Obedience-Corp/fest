package show

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/spf13/cobra"
)

// NewRulesCommand creates the fest rules command.
func NewRulesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rules",
		Short: "Display festival rules for the current festival",
		Long:  "Reads and displays the FESTIVAL_RULES.md file for the current festival.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRules(cmd)
		},
	}
}

func runRules(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival")
	}

	rulesPath := filepath.Join(festivalPath, "FESTIVAL_RULES.md")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NotFound("FESTIVAL_RULES.md").WithOp("rules").
				WithField("festival", filepath.Base(festivalPath))
		}
		return errors.IO("reading FESTIVAL_RULES.md", err)
	}

	body := stripRulesFrontmatter(string(content))
	if strings.TrimSpace(body) == "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "FESTIVAL_RULES.md exists but has no content.")
		return nil
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), body)
	return nil
}

// stripRulesFrontmatter removes YAML frontmatter delimited by --- from content.
func stripRulesFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return content
	}
	return strings.TrimLeft(content[end+6:], "\n")
}
