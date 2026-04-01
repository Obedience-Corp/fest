package notes

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type ReleaseMode string

const (
	ModeStable ReleaseMode = "stable"
	ModeRC     ReleaseMode = "rc"
	ModeDev    ReleaseMode = "dev"
)

type ChangeCategory string

const (
	CategoryFeature     ChangeCategory = "feature"
	CategoryFix         ChangeCategory = "fix"
	CategoryDocs        ChangeCategory = "docs"
	CategoryMaintenance ChangeCategory = "maintenance"
	CategoryOther       ChangeCategory = "other"
)

type TagInfo struct {
	Raw       string
	Mode      ReleaseMode
	Major     int
	Minor     int
	Patch     int
	Iteration int
}

type Change struct {
	Text     string
	PRNumber int
	Category ChangeCategory
}

var (
	stableTagRe   = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
	rcTagRe       = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)-rc\.(\d+)$`)
	devTagRe      = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)-dev\.(\d+)$`)
	subjectRe     = regexp.MustCompile(`^(feat|fix|docs|refactor|chore|test|build|ci|perf)(?:\(([^)]+)\))?:\s*(.+)$`)
	prSuffixRe    = regexp.MustCompile(`\s+\(#(\d+)\)$`)
	bracketLeadRe = regexp.MustCompile(`^\[[^]]+\]\s*`)
)

func ParseTag(raw string) (TagInfo, error) {
	if match := stableTagRe.FindStringSubmatch(raw); match != nil {
		return TagInfo{
			Raw:   raw,
			Mode:  ModeStable,
			Major: mustAtoi(match[1]),
			Minor: mustAtoi(match[2]),
			Patch: mustAtoi(match[3]),
		}, nil
	}

	if match := rcTagRe.FindStringSubmatch(raw); match != nil {
		return TagInfo{
			Raw:       raw,
			Mode:      ModeRC,
			Major:     mustAtoi(match[1]),
			Minor:     mustAtoi(match[2]),
			Patch:     mustAtoi(match[3]),
			Iteration: mustAtoi(match[4]),
		}, nil
	}

	if match := devTagRe.FindStringSubmatch(raw); match != nil {
		return TagInfo{
			Raw:       raw,
			Mode:      ModeDev,
			Major:     mustAtoi(match[1]),
			Minor:     mustAtoi(match[2]),
			Patch:     mustAtoi(match[3]),
			Iteration: mustAtoi(match[4]),
		}, nil
	}

	return TagInfo{}, fmt.Errorf("invalid release tag %q", raw)
}

func FindPreviousTag(target TagInfo, tags []string) string {
	switch target.Mode {
	case ModeStable:
		return findPreviousStable(target, tags)
	case ModeRC:
		if prev := findPreviousSameLine(target, tags, ModeRC); prev != "" {
			return prev
		}
		return findPreviousStable(target, tags)
	case ModeDev:
		if prev := findPreviousSameLine(target, tags, ModeDev); prev != "" {
			return prev
		}
		return findPreviousStable(target, tags)
	default:
		return ""
	}
}

func ParseCommitSubject(subject string) (Change, bool) {
	subject = strings.TrimSpace(subject)
	for {
		trimmed := bracketLeadRe.ReplaceAllString(subject, "")
		if trimmed == subject {
			break
		}
		subject = strings.TrimSpace(trimmed)
	}

	if subject == "" || strings.HasPrefix(subject, "Merge ") {
		return Change{}, false
	}

	prNumber := 0
	if match := prSuffixRe.FindStringSubmatch(subject); match != nil {
		prNumber = mustAtoi(match[1])
		subject = strings.TrimSpace(prSuffixRe.ReplaceAllString(subject, ""))
	}

	if match := subjectRe.FindStringSubmatch(subject); match != nil {
		kind := match[1]
		scope := strings.ToLower(match[2])
		text := normalizeText(match[3])

		return Change{
			Text:     sentenceCase(text),
			PRNumber: prNumber,
			Category: classify(kind, scope, text),
		}, text != ""
	}

	text := normalizeText(subject)
	if text == "" {
		return Change{}, false
	}

	return Change{
		Text:     sentenceCase(text),
		PRNumber: prNumber,
		Category: classify("", "", text),
	}, true
}

func Render(repo string, current TagInfo, previousTag string, changes []Change) (string, error) {
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}

	changes = dedupeChanges(changes)

	var b strings.Builder
	writeLine := func(s string) {
		b.WriteString(s)
		b.WriteByte('\n')
	}

	writeLine("## Release " + current.Raw)
	writeLine("")
	writeLine(fmt.Sprintf("This %s release includes the following changes.", channelLabel(current.Mode)))
	writeLine("")

	if previousTag != "" {
		writeLine("Summary:")
		for _, line := range summaryLines(changes) {
			writeLine("- " + line)
		}
		writeLine("")
		writeLine(fmt.Sprintf("Compare: https://github.com/%s/compare/%s...%s", repo, previousTag, current.Raw))
		writeLine("")
	} else {
		writeLine("This is the first release covered by the structured release-notes generator.")
		writeLine("")
	}

	highlights := highlightChanges(changes)
	if len(highlights) > 0 && !(len(highlights) == 1 && countUserFacing(changes) == 1) {
		writeLine("## Highlights")
		writeLine("")
		for _, change := range highlights {
			writeLine("- " + renderChange(repo, change))
		}
		writeLine("")
	}

	sections := []struct {
		title    string
		category ChangeCategory
	}{
		{title: "Features", category: CategoryFeature},
		{title: "Fixes", category: CategoryFix},
		{title: "Documentation", category: CategoryDocs},
		{title: "Maintenance", category: CategoryMaintenance},
		{title: "Other Changes", category: CategoryOther},
	}

	for _, section := range sections {
		entries := filterChanges(changes, section.category)
		if len(entries) == 0 {
			continue
		}
		writeLine("## " + section.title)
		writeLine("")
		for _, change := range entries {
			writeLine("- " + renderChange(repo, change))
		}
		writeLine("")
	}

	return strings.TrimSpace(b.String()) + "\n", nil
}

func findPreviousStable(target TagInfo, tags []string) string {
	for _, raw := range tags {
		if raw == target.Raw {
			continue
		}
		candidate, err := ParseTag(raw)
		if err != nil || candidate.Mode != ModeStable {
			continue
		}
		if compareVersion(candidate, target) < 0 {
			return raw
		}
	}
	return ""
}

func findPreviousSameLine(target TagInfo, tags []string, mode ReleaseMode) string {
	for _, raw := range tags {
		if raw == target.Raw {
			continue
		}
		candidate, err := ParseTag(raw)
		if err != nil || candidate.Mode != mode {
			continue
		}
		if compareVersion(candidate, target) == 0 && candidate.Iteration < target.Iteration {
			return raw
		}
	}
	return ""
}

func compareVersion(left, right TagInfo) int {
	switch {
	case left.Major != right.Major:
		if left.Major < right.Major {
			return -1
		}
		return 1
	case left.Minor != right.Minor:
		if left.Minor < right.Minor {
			return -1
		}
		return 1
	case left.Patch != right.Patch:
		if left.Patch < right.Patch {
			return -1
		}
		return 1
	default:
		return 0
	}
}

func classify(kind, scope, text string) ChangeCategory {
	switch {
	case kind == "feat":
		return CategoryFeature
	case kind == "fix" && scope == "docs":
		return CategoryDocs
	case kind == "fix" || kind == "perf":
		return CategoryFix
	case kind == "docs":
		return CategoryDocs
	case kind == "refactor" || kind == "chore" || kind == "test" || kind == "build" || kind == "ci":
		return CategoryMaintenance
	}

	lower := strings.ToLower(text)
	if strings.Contains(lower, "doc") || strings.Contains(lower, "readme") {
		return CategoryDocs
	}
	return CategoryOther
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func sentenceCase(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func summaryLines(changes []Change) []string {
	counts := map[ChangeCategory]int{}
	for _, change := range changes {
		counts[change.Category]++
	}

	var lines []string
	appendSummary := func(category ChangeCategory, singular, plural string) {
		if counts[category] == 0 {
			return
		}
		label := plural
		if counts[category] == 1 {
			label = singular
		}
		lines = append(lines, fmt.Sprintf("%d %s", counts[category], label))
	}

	appendSummary(CategoryFeature, "feature", "features")
	appendSummary(CategoryFix, "fix", "fixes")
	appendSummary(CategoryDocs, "documentation update", "documentation updates")
	appendSummary(CategoryMaintenance, "maintenance change", "maintenance changes")
	appendSummary(CategoryOther, "other change", "other changes")

	if len(lines) == 0 {
		return []string{"No user-facing changes were detected in the tagged git range"}
	}

	return lines
}

func highlightChanges(changes []Change) []Change {
	var highlights []Change
	for _, change := range changes {
		if change.Category == CategoryMaintenance || change.Category == CategoryOther {
			continue
		}
		highlights = append(highlights, change)
		if len(highlights) == 3 {
			return highlights
		}
	}

	if len(highlights) > 0 {
		return highlights
	}

	if len(changes) > 3 {
		return changes[:3]
	}
	return changes
}

func filterChanges(changes []Change, category ChangeCategory) []Change {
	var filtered []Change
	for _, change := range changes {
		if change.Category == category {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func dedupeChanges(changes []Change) []Change {
	if len(changes) < 2 {
		return changes
	}

	var deduped []Change
	indexByKey := map[string]int{}

	for _, change := range changes {
		key := normalizedChangeKey(change)
		if idx, ok := indexByKey[key]; ok {
			deduped[idx] = preferChange(deduped[idx], change)
			continue
		}

		indexByKey[key] = len(deduped)
		deduped = append(deduped, change)
	}

	return deduped
}

func normalizedChangeKey(change Change) string {
	return strings.ToLower(strings.TrimSpace(change.Text))
}

func preferChange(current, candidate Change) Change {
	switch {
	case current.PRNumber == 0 && candidate.PRNumber != 0:
		return candidate
	case current.PRNumber != 0 && candidate.PRNumber == 0:
		return current
	case categoryRank(candidate.Category) < categoryRank(current.Category):
		return candidate
	default:
		return current
	}
}

func categoryRank(category ChangeCategory) int {
	switch category {
	case CategoryFeature:
		return 0
	case CategoryFix:
		return 1
	case CategoryDocs:
		return 2
	case CategoryMaintenance:
		return 3
	default:
		return 4
	}
}

func countUserFacing(changes []Change) int {
	count := 0
	for _, change := range changes {
		if change.Category == CategoryFeature || change.Category == CategoryFix || change.Category == CategoryDocs {
			count++
		}
	}
	return count
}

func renderChange(repo string, change Change) string {
	if change.PRNumber == 0 {
		return change.Text
	}
	return fmt.Sprintf("%s ([#%d](https://github.com/%s/pull/%d))", change.Text, change.PRNumber, repo, change.PRNumber)
}

func channelLabel(mode ReleaseMode) string {
	switch mode {
	case ModeStable:
		return "stable"
	case ModeRC:
		return "release candidate"
	case ModeDev:
		return "development"
	default:
		return "unknown"
	}
}

func mustAtoi(s string) int {
	var n int
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
