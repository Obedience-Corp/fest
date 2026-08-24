package ritual

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

var (
	checkedListItem = regexp.MustCompile(`^([\s]*[-*]\s*)\[(x|X)\]`)
	emojiProgress   = regexp.MustCompile(`\[(✅|🚧|❌)\]`)
)

var runtimeProgressStatuses = map[string]struct{}{
	"completed":   {},
	"in_progress": {},
	"blocked":     {},
	"passed":      {},
	"failed":      {},
	"active":      {},
	"ready":       {},
}

// resetProgressArtifacts strips in-progress and completed execution state from
// a converted ritual copy so fest ritual run starts from a clean template.
func resetProgressArtifacts(ctx context.Context, destPath string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	return filepath.WalkDir(destPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if walkErr := ctx.Err(); walkErr != nil {
			return errors.Wrap(walkErr, "context cancelled")
		}

		if d.IsDir() {
			switch d.Name() {
			case ".fest", ".workflow":
				if removeErr := os.RemoveAll(path); removeErr != nil {
					return errors.IO("removing progress directory", removeErr).
						WithField("path", path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		return resetMarkdownFile(path)
	})
}

func resetMarkdownFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.IO("reading markdown for progress reset", err).
			WithField("path", path)
	}

	updated := resetMarkdownCompletion(content)
	if bytes.Equal(content, updated) {
		return nil
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, updated, mode); err != nil {
		return errors.IO("writing reset markdown", err).
			WithField("path", path)
	}
	return nil
}

func resetMarkdownCompletion(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lines[i] = resetProgressLine(line)
	}
	return resetFrontmatterProgressStatus([]byte(strings.Join(lines, "\n")))
}

func resetProgressLine(line string) string {
	line = checkedListItem.ReplaceAllString(line, `${1}[ ]`)
	return emojiProgress.ReplaceAllString(line, `[ ]`)
}

func resetFrontmatterProgressStatus(content []byte) []byte {
	typ, status, ok := peekFrontmatterTypeStatus(content)
	if !ok || !isRuntimeStatus(status) {
		return content
	}
	next := string(frontmatter.DefaultStatus(typ))
	if strings.EqualFold(status, next) {
		return content
	}
	updated, changed := frontmatter.UpdateFields(content, map[string]string{
		"fest_status": next,
	})
	if !changed {
		return content
	}
	return updated
}

func peekFrontmatterTypeStatus(content []byte) (frontmatter.Type, string, bool) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	var typ frontmatter.Type
	var status string
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			break
		}
		key, val, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "fest_type":
			typ = frontmatter.Type(strings.TrimSpace(val))
		case "fest_status":
			status = strings.TrimSpace(val)
		}
	}
	if status == "" {
		return "", "", false
	}
	return typ, status, true
}

func isRuntimeStatus(status string) bool {
	_, ok := runtimeProgressStatuses[strings.ToLower(status)]
	return ok
}
