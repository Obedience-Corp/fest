package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestHooksConceptDocCitesRealCLI(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(hooksDocRepoRoot(t), "docs", "concepts", "hooks.md"))
	if err != nil {
		t.Fatal(err)
	}
	cites := inlineFestCitations(string(body))
	if len(cites) == 0 {
		t.Fatal("docs/concepts/hooks.md has no `fest ...` command citations")
	}
	for _, cite := range cites {
		if err := validateCitedFestCommand(rootCmd, cite); err != nil {
			t.Errorf("%q: %v", cite, err)
		}
	}
}

func TestValidateCitedFestCommand_RejectsInventedForms(t *testing.T) {
	tests := []struct {
		cite    string
		wantErr bool
	}{
		{cite: "fest task in-progress", wantErr: true},
		{cite: "fest task update --progress N", wantErr: true},
		{cite: "fest task update 50%", wantErr: false},
		{cite: "fest task update", wantErr: false},
		{cite: "fest status set --task ... in_progress", wantErr: false},
		{cite: "fest status set ... in_progress", wantErr: false},
		{cite: "fest progress --in-progress", wantErr: false},
		{cite: "fest task completed --yes --json", wantErr: false},
		{cite: "fest hooks list --json", wantErr: false},
		{cite: "fest workflow approve --auto", wantErr: false},
		{cite: "fest status set ... completed", wantErr: false},
	}
	for _, tt := range tests {
		err := validateCitedFestCommand(rootCmd, tt.cite)
		if tt.wantErr && err == nil {
			t.Errorf("%q: want error, got nil", tt.cite)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("%q: unexpected error: %v", tt.cite, err)
		}
	}
}

func validateCitedFestCommand(root *cobra.Command, cite string) error {
	tokens := strings.Fields(cite)
	if len(tokens) == 0 || tokens[0] != root.Name() {
		return fmt.Errorf("citation must start with %q", root.Name())
	}
	cmd, rest := consumeCommandPath(root, tokens[1:])
	return consumeCitedArgs(cmd, rest)
}

func consumeCommandPath(cmd *cobra.Command, tokens []string) (*cobra.Command, []string) {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if isCitePlaceholder(tok) {
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") {
			break
		}
		child := visibleCiteChild(cmd, tok)
		if child == nil {
			break
		}
		cmd = child
		i++
	}
	return cmd, tokens[i:]
}

func consumeCitedArgs(cmd *cobra.Command, tokens []string) error {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if isCitePlaceholder(tok) {
			continue
		}
		if strings.HasPrefix(tok, "-") {
			flag, err := citedFlag(cmd, tok)
			if err != nil {
				return err
			}
			if flagTakesNextArg(tok, flag) && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++
			}
			continue
		}
		if looksLikeSubcommand(tok) && hasVisibleCiteChildren(cmd) {
			return fmt.Errorf("unknown subcommand %q of %s", tok, cmd.CommandPath())
		}
	}
	return nil
}

func citedFlag(cmd *cobra.Command, tok string) (*pflag.Flag, error) {
	var flag *pflag.Flag
	switch {
	case strings.HasPrefix(tok, "--"):
		name := strings.TrimPrefix(tok, "--")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if name == "" {
			return nil, fmt.Errorf("invalid flag %q on %s", tok, cmd.CommandPath())
		}
		flag = cmd.Flag(name)
	case len(tok) >= 2 && tok[0] == '-' && tok[1] != '-':
		flag = cmd.Flags().ShorthandLookup(string(tok[1]))
		if flag == nil {
			flag = cmd.PersistentFlags().ShorthandLookup(string(tok[1]))
		}
	}
	if flag == nil {
		return nil, fmt.Errorf("unknown flag %q on %s", tok, cmd.CommandPath())
	}
	return flag, nil
}

func flagTakesNextArg(tok string, flag *pflag.Flag) bool {
	if flag.Value.Type() == "bool" || strings.Contains(tok, "=") {
		return false
	}
	return true
}

func visibleCiteChild(cmd *cobra.Command, name string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if shouldSkipCommandSurfaceEntry(child) {
			continue
		}
		if child.Name() == name {
			return child
		}
		for _, alias := range child.Aliases {
			if alias == name {
				return child
			}
		}
	}
	return nil
}

func hasVisibleCiteChildren(cmd *cobra.Command) bool {
	for _, child := range cmd.Commands() {
		if !shouldSkipCommandSurfaceEntry(child) {
			return true
		}
	}
	return false
}

func isCitePlaceholder(tok string) bool {
	switch tok {
	case "...", "…", "N":
		return true
	}
	if len(tok) >= 2 {
		if (tok[0] == '<' && tok[len(tok)-1] == '>') || (tok[0] == '{' && tok[len(tok)-1] == '}') {
			return true
		}
	}
	return false
}

func looksLikeSubcommand(tok string) bool {
	if tok == "" || tok[0] < 'a' || tok[0] > 'z' {
		return false
	}
	for _, r := range tok {
		if r != '-' && r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func inlineFestCitations(md string) []string {
	body := stripFencedCode(md)
	var out []string
	for i := 0; i < len(body); {
		start := strings.IndexByte(body[i:], '`')
		if start < 0 {
			break
		}
		start += i
		rest := body[start+1:]
		end := strings.IndexByte(rest, '`')
		if end < 0 {
			break
		}
		inner := rest[:end]
		if inner == "fest" || strings.HasPrefix(inner, "fest ") {
			out = append(out, inner)
		}
		i = start + 1 + end + 1
	}
	return out
}

func stripFencedCode(md string) string {
	var b strings.Builder
	rest := md
	for {
		start := strings.Index(rest, "```")
		if start < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:start])
		after := rest[start+3:]
		end := strings.Index(after, "```")
		if end < 0 {
			return b.String()
		}
		rest = after[end+3:]
	}
}

func hooksDocRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the test directory")
		}
		dir = parent
	}
}
