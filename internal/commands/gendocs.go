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

// fenceExamples finds "Examples:" blocks in markdown and wraps the
// indented content that follows in a code fence so that # comments
// don't render as headings.
func fenceExamples(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "Examples:" {
			out = append(out, line)
			i++
			// Collect indented example lines until we hit a code fence or non-indented content
			var exampleLines []string
			for i < len(lines) {
				l := lines[i]
				// Stop at code fence (usage block) or section heading
				if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "###") {
					break
				}
				exampleLines = append(exampleLines, l)
				i++
			}
			// Trim trailing blank lines from examples
			for len(exampleLines) > 0 && strings.TrimSpace(exampleLines[len(exampleLines)-1]) == "" {
				exampleLines = exampleLines[:len(exampleLines)-1]
			}
			if len(exampleLines) > 0 {
				out = append(out, "```")
				out = append(out, exampleLines...)
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

func combineSingleFile(dir, name string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("# %s CLI Reference\n", name))

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
