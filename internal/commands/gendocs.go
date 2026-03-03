package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	gendocsOutput string
	gendocsFormat string
	gendocsSingle bool
)

func init() {
	gendocsCmd := &cobra.Command{
		Use:    "gendocs",
		Short:  "Generate CLI reference documentation",
		Long:   "Generate markdown or YAML reference documentation for all fest commands.",
		Hidden: true,
		Annotations: map[string]string{
			"scope": "global",
		},
		RunE: runGendocs,
	}
	gendocsCmd.Flags().StringVar(&gendocsOutput, "output", "docs", "output directory")
	gendocsCmd.Flags().StringVar(&gendocsFormat, "format", "markdown", "output format: markdown or yaml")
	gendocsCmd.Flags().BoolVar(&gendocsSingle, "single", false, "generate a single combined reference file")
	rootCmd.AddCommand(gendocsCmd)
}

func runGendocs(cmd *cobra.Command, args []string) error {
	if err := os.MkdirAll(gendocsOutput, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	stripANSIFromTree(rootCmd)
	disableAutoGenTag(rootCmd)

	switch gendocsFormat {
	case "markdown":
		if err := doc.GenMarkdownTree(rootCmd, gendocsOutput); err != nil {
			return fmt.Errorf("generate markdown: %w", err)
		}
		if err := fenceExamplesInDir(gendocsOutput); err != nil {
			return fmt.Errorf("post-process examples: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Markdown docs written to %s\n", gendocsOutput)
	case "yaml":
		if err := doc.GenYamlTree(rootCmd, gendocsOutput); err != nil {
			return fmt.Errorf("generate yaml: %w", err)
		}
		fmt.Fprintf(os.Stderr, "YAML docs written to %s\n", gendocsOutput)
	default:
		return fmt.Errorf("unknown format %q (use markdown or yaml)", gendocsFormat)
	}

	if gendocsSingle && gendocsFormat == "markdown" {
		if err := combineSingleFile(gendocsOutput, "fest"); err != nil {
			return fmt.Errorf("generate single file: %w", err)
		}
	}

	return nil
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func stripANSIFromTree(cmd *cobra.Command) {
	cmd.Short = stripANSI(cmd.Short)
	cmd.Long = stripANSI(cmd.Long)
	cmd.Example = stripANSI(cmd.Example)

	for _, g := range cmd.Groups() {
		g.Title = stripANSI(g.Title)
	}

	for _, child := range cmd.Commands() {
		stripANSIFromTree(child)
	}
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, child := range cmd.Commands() {
		disableAutoGenTag(child)
	}
}

// fenceExamplesInDir wraps unformatted Examples: blocks in code fences.
// Cobra's GenMarkdownTree renders the Long description as plain markdown,
// so lines like "# comment" inside examples become headings.
func fenceExamplesInDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result := fenceExamples(string(data))
		if result != string(data) {
			if err := os.WriteFile(path, []byte(result), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// fenceExamples wraps unfenced blocks in the Synopsis that contain shell
// commands or # comments. Cobra's GenMarkdownTree renders Long as plain
// markdown, so "# comment" lines become headings and "--flag" gets
// mangled by Hugo's typographer.
//
// Strategy: scan for lines that look like shell content (indented # comments
// or indented command invocations) outside of existing code fences, and
// wrap contiguous runs of such lines in code fences.
func fenceExamples(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inFence := false

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Track existing code fences
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			out = append(out, line)
			i++
			continue
		}

		// Skip content inside existing fences
		if inFence {
			out = append(out, line)
			i++
			continue
		}

		// Skip markdown headings (### Synopsis, ### Options, etc.)
		if strings.HasPrefix(line, "#") {
			out = append(out, line)
			i++
			continue
		}

		// Detect indented shell-like content: lines starting with whitespace
		// followed by # comment or a known command prefix
		if looksLikeShellBlock(line) {
			// Collect the full block of shell-like + context lines
			var block []string
			for i < len(lines) {
				l := lines[i]
				trimmed := strings.TrimSpace(l)
				// Stop at code fence, heading, or completely unindented non-empty text
				// that doesn't look like part of the block
				if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(l, "###") {
					break
				}
				// Empty lines are allowed within blocks
				if trimmed == "" {
					block = append(block, l)
					i++
					continue
				}
				// Indented content continues the block
				if strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "\t") {
					block = append(block, l)
					i++
					continue
				}
				// Unindented ALL-CAPS label followed by colon (e.g. "NEXT STEPS:")
				// is a section header within the synopsis - keep it outside the fence
				if isAllCapsLabel(trimmed) {
					break
				}
				// Other unindented text breaks the block
				break
			}
			// Trim trailing blank lines
			for len(block) > 0 && strings.TrimSpace(block[len(block)-1]) == "" {
				block = block[:len(block)-1]
			}
			if len(block) > 0 {
				out = append(out, "```bash")
				out = append(out, block...)
				out = append(out, "```")
				out = append(out, "")
			}
			continue
		}

		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

// looksLikeShellBlock returns true if the line appears to be indented
// shell content (a # comment or command invocation).
func looksLikeShellBlock(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// Must be indented (starts with spaces or tab)
	if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") {
		return false
	}
	// Indented # comment
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	// Indented command invocation (fest, camp, or common shell patterns)
	for _, prefix := range []string{"fest ", "camp ", "eval ", "source "} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// isAllCapsLabel checks if a line is an all-caps section label like
// "EXAMPLES:", "NEXT STEPS:", "TEMPLATE VARIABLES:", etc.
func isAllCapsLabel(s string) bool {
	s = strings.TrimSuffix(s, ":")
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r != ' ' && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func combineSingleFile(dir, name string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("---\ntitle: \"%s Reference\"\nweight: 1\n---\n\n# %s CLI Reference\n", name, name))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if entry.Name() == name+"-reference.md" || entry.Name() == "_index.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		parts = append(parts, string(data))
	}

	combined := strings.Join(parts, "\n---\n\n")
	outPath := filepath.Join(dir, name+"-reference.md")
	if err := os.WriteFile(outPath, []byte(combined), 0644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Combined reference written to %s\n", outPath)
	return nil
}
