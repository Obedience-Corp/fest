package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

var selectorDateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}(-\d{2})?$`)

// FestivalSelectorCandidate represents a festival available for explicit selector resolution.
type FestivalSelectorCandidate struct {
	Name string
	ID   string
	Path string
}

type selectorMatch struct {
	Name  string
	Path  string
	Score int
}

// ResolveFestivalSelector resolves a selector to a festival path.
//
// Resolution order:
//  1. Exact festival ID match (e.g. GS0001)
//  2. Exact festival directory name match
//  3. Fuzzy fallback against known festivals
//
// This resolver is intentionally campaign-scoped.
func ResolveFestivalSelector(ctx context.Context, cwd, selector string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		return "", errors.Validation("festival selector cannot be empty")
	}

	candidates, err := ListFestivalSelectorCandidates(ctx, cwd)
	if err != nil {
		return "", err
	}

	// 1) Exact ID match first.
	for _, c := range candidates {
		if c.ID != "" && strings.EqualFold(c.ID, trimmed) {
			return c.Path, nil
		}
	}

	// 2) Exact name match.
	for _, c := range candidates {
		if strings.EqualFold(c.Name, trimmed) {
			return c.Path, nil
		}
	}

	// 3) Fuzzy fallback.
	matches := fuzzyMatchSelectorCandidates(trimmed, candidates)
	if len(matches) == 0 {
		return "", errors.NotFound("festival").
			WithField("selector", selector).
			WithHint("Run 'fest show all' to see available festivals")
	}

	if len(matches) == 1 || isUnambiguousSelectorMatch(matches) {
		return matches[0].Path, nil
	}

	names := topMatchNames(matches, 5)
	return "", errors.Validation(fmt.Sprintf("festival selector %q is ambiguous", selector)).
		WithField("selector", selector).
		WithHint(fmt.Sprintf("Try a more specific selector. Top matches: %s", strings.Join(names, ", ")))
}

// CompleteFestivalSelector returns shell completion candidates for a selector.
// Only includes festivals in working statuses (planning, ready, active, ritual).
func CompleteFestivalSelector(ctx context.Context, cwd, toComplete string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	festivalsDir, err := findCampaignFestivalsDir(ctx, cwd)
	if err != nil {
		return nil, err
	}
	candidates := collectSelectorCandidates(festivalsDir, id.WorkingStatusDirectories)

	if strings.TrimSpace(toComplete) == "" {
		result := make([]string, 0, len(candidates))
		for _, c := range candidates {
			result = append(result, c.Name)
		}
		sort.Strings(result)
		return result, nil
	}

	matches := fuzzyMatchSelectorCandidates(toComplete, candidates)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, m.Name)
	}
	return result, nil
}

// ListFestivalSelectorCandidates lists all festivals available for selector matching.
// This is campaign-scoped by design.
func ListFestivalSelectorCandidates(ctx context.Context, cwd string) ([]FestivalSelectorCandidate, error) {
	festivalsDir, err := findCampaignFestivalsDir(ctx, cwd)
	if err != nil {
		return nil, err
	}
	return collectSelectorCandidates(festivalsDir, id.StatusDirectories), nil
}

func findCampaignFestivalsDir(ctx context.Context, cwd string) (string, error) {
	campaignRoot, err := workspace.DetectCampaign(ctx, cwd)
	if err != nil {
		return "", errors.NotFound("campaign workspace").
			WithHint("Run this command from inside a campaign workspace")
	}

	festivalsDir := filepath.Join(campaignRoot, workspace.FestivalsDir)
	info, statErr := os.Stat(festivalsDir)
	if statErr != nil || !info.IsDir() {
		return "", errors.NotFound("festivals directory").
			WithField("path", festivalsDir).
			WithHint("Campaign workspace is missing festivals/ directory")
	}

	return festivalsDir, nil
}

func collectSelectorCandidates(festivalsDir string, statuses []string) []FestivalSelectorCandidate {
	var candidates []FestivalSelectorCandidate

	for _, status := range statuses {
		statusDir := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			path := filepath.Join(statusDir, name)

			if isValidFestival(path) {
				candidates = append(candidates, FestivalSelectorCandidate{
					Name: name,
					ID:   extractSelectorID(name),
					Path: path,
				})
				continue
			}

			// For dungeon statuses, recurse into date directories.
			if !selectorDateDirPattern.MatchString(name) {
				continue
			}

			dateEntries, dateErr := os.ReadDir(path)
			if dateErr != nil {
				continue
			}
			for _, de := range dateEntries {
				if !de.IsDir() {
					continue
				}
				festivalName := de.Name()
				festivalPath := filepath.Join(path, festivalName)
				if !isValidFestival(festivalPath) {
					continue
				}
				candidates = append(candidates, FestivalSelectorCandidate{
					Name: festivalName,
					ID:   extractSelectorID(festivalName),
					Path: festivalPath,
				})
			}
		}
	}

	return dedupeSelectorCandidates(candidates)
}

func dedupeSelectorCandidates(candidates []FestivalSelectorCandidate) []FestivalSelectorCandidate {
	seen := make(map[string]bool, len(candidates))
	result := make([]FestivalSelectorCandidate, 0, len(candidates))
	for _, c := range candidates {
		if seen[c.Path] {
			continue
		}
		seen[c.Path] = true
		result = append(result, c)
	}
	return result
}

func extractSelectorID(name string) string {
	extracted, err := id.ExtractLogicalIDFromDirName(name)
	if err != nil {
		return ""
	}
	return extracted
}

func fuzzyMatchSelectorCandidates(query string, candidates []FestivalSelectorCandidate) []selectorMatch {
	byPath := make(map[string]selectorMatch, len(candidates))
	for _, c := range candidates {
		// Score against name and ID and keep the best score.
		scoreName, _ := navigation.Score(query, c.Name)
		score := scoreName
		if c.ID != "" {
			scoreID, _ := navigation.Score(query, c.ID)
			if scoreID > score {
				score = scoreID
			}
		}
		if score <= 0 {
			continue
		}

		existing, ok := byPath[c.Path]
		if !ok || score > existing.Score {
			byPath[c.Path] = selectorMatch{
				Name:  c.Name,
				Path:  c.Path,
				Score: score,
			}
		}
	}

	matches := make([]selectorMatch, 0, len(byPath))
	for _, m := range byPath {
		matches = append(matches, m)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Name < matches[j].Name
	})

	return matches
}

func isUnambiguousSelectorMatch(matches []selectorMatch) bool {
	if len(matches) <= 1 {
		return true
	}
	threshold := float64(matches[0].Score) * 0.8
	return float64(matches[1].Score) < threshold
}

func topMatchNames(matches []selectorMatch, max int) []string {
	if max <= 0 || max > len(matches) {
		max = len(matches)
	}
	result := make([]string, 0, max)
	for _, m := range matches[:max] {
		result = append(result, m.Name)
	}
	return result
}
