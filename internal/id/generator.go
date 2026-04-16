// Package id provides festival ID generation and parsing utilities.
// Festival IDs follow the format XX0001 where XX is a 2-letter prefix
// derived from the festival name and 0001 is a 4-digit counter.
package id

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// Common words to skip when extracting initials
var commonWords = map[string]bool{
	"the": true, "a": true, "an": true,
	"of": true, "for": true, "and": true,
	"to": true, "in": true, "on": true,
}

// idPattern matches festival IDs in directory names (e.g., -GC0001)
var idPattern = regexp.MustCompile(`-([A-Z]{2})(\d{4,})$`)

// StatusDirectories are ALL directories that can contain festivals (full lifecycle)
var StatusDirectories = []string{"planning", "ready", "active", "ritual", "dungeon/completed", "dungeon/archived", "dungeon/someday"}

// PrimaryStatusDirs are non-terminal directories surfaced in navigation,
// fuzzy search, `fest list` defaults, and shell completion. Kept in sync
// with WorkingStatusDirectories so fgo fuzzy matching can reach ritual
// festivals alongside active/ready/planning.
var PrimaryStatusDirs = []string{"active", "ready", "planning", "ritual"}

// WorkingStatusDirectories are non-terminal directories (excludes dungeon/*).
var WorkingStatusDirectories = []string{"planning", "ready", "active", "ritual"}

// ResolveStatusPath maps user-facing status names to filesystem directory paths.
// Dungeon sub-statuses ("completed", "archived", "someday") resolve to their
// full paths ("dungeon/completed", etc.). All other statuses pass through unchanged.
func ResolveStatusPath(status string) string {
	switch status {
	case "completed":
		return "dungeon/completed"
	case "archived":
		return "dungeon/archived"
	case "someday":
		return "dungeon/someday"
	default:
		return status
	}
}

// ExtractInitials extracts a 2-letter uppercase prefix from a festival name.
// For multi-word names: first letter of first two significant words.
// For single-word names: first two letters.
// If extraction fails, returns "XX" as a fallback.
func ExtractInitials(name string) string {
	// Normalize: replace hyphens with spaces, lowercase
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", " "))

	// Split into words
	words := strings.Fields(normalized)

	// Filter out common words and non-alphabetic entries
	var significant []string
	for _, word := range words {
		// Skip common words
		if commonWords[word] {
			continue
		}
		// Keep only if starts with a letter
		if len(word) > 0 && unicode.IsLetter(rune(word[0])) {
			significant = append(significant, word)
		}
	}

	// Build initials
	var initials strings.Builder

	switch len(significant) {
	case 0:
		// No significant words found
		return "XX"
	case 1:
		// Single word: take first two letters
		word := significant[0]
		for _, r := range word {
			if unicode.IsLetter(r) {
				initials.WriteRune(unicode.ToUpper(r))
				if initials.Len() >= 2 {
					break
				}
			}
		}
	default:
		// Multiple words: first letter of first two
		for i := 0; i < 2 && i < len(significant); i++ {
			for _, r := range significant[i] {
				if unicode.IsLetter(r) {
					initials.WriteRune(unicode.ToUpper(r))
					break
				}
			}
		}
	}

	// Pad if needed
	result := initials.String()
	if len(result) < 2 {
		result = result + strings.Repeat("X", 2-len(result))
	}

	return result[:2]
}

// FormatID creates a festival ID string from prefix and counter.
// Format: XX0001 (2 letters + 4 digits, zero-padded)
func FormatID(prefix string, counter int) string {
	if counter > 9999 {
		// Overflow: don't zero-pad beyond 4 digits
		return fmt.Sprintf("%s%d", prefix, counter)
	}
	return fmt.Sprintf("%s%04d", prefix, counter)
}

// ParseID extracts the prefix and counter from a festival ID.
// Returns an error if the ID format is invalid.
func ParseID(id string) (prefix string, counter int, err error) {
	if len(id) < 6 {
		return "", 0, errors.Validation("invalid ID format: too short").
			WithField("id", id).
			WithField("minLength", 6)
	}

	prefix = id[:2]
	if !isValidPrefix(prefix) {
		return "", 0, errors.Validation("invalid ID format: prefix must be two uppercase letters").
			WithField("prefix", prefix)
	}

	counterStr := id[2:]
	if len(counterStr) < 4 {
		return "", 0, errors.Validation("invalid ID format: counter too short").
			WithField("counter", counterStr)
	}

	counter, err = strconv.Atoi(counterStr)
	if err != nil {
		return "", 0, errors.Validation("invalid ID format: counter must be numeric").
			WithField("counter", counterStr)
	}

	return prefix, counter, nil
}

// isValidPrefix checks if a prefix is two uppercase letters
func isValidPrefix(prefix string) bool {
	if len(prefix) != 2 {
		return false
	}
	for _, r := range prefix {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ExtractIDFromDirName extracts the festival ID from a directory name.
// Directory names follow the pattern: name-XX0001
func ExtractIDFromDirName(dirName string) (string, error) {
	if dirName == "" {
		return "", errors.Validation("empty directory name")
	}

	matches := idPattern.FindStringSubmatch(dirName)
	if matches == nil {
		return "", errors.NotFound("no valid ID found in directory name").
			WithField("dirName", dirName)
	}

	// matches[0] is full match, matches[1] is prefix, matches[2] is counter
	return matches[1] + matches[2], nil
}

// FindNextCounter scans all festival directories to find the next available
// counter for the given prefix. Returns the next counter value (max + 1).
func FindNextCounter(ctx context.Context, festivalsRoot string, prefix string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	maxCounter := 0

	// Scan each status directory
	for _, status := range StatusDirectories {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		statusPath := filepath.Join(festivalsRoot, status)

		// Skip if directory doesn't exist
		if _, err := os.Stat(statusPath); os.IsNotExist(err) {
			continue
		}

		// Walk the directory tree to find all festivals
		err := filepath.WalkDir(statusPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip directories we can't read
			}

			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			if !d.IsDir() {
				return nil
			}

			// Try to extract ID from directory name
			id, err := ExtractIDFromDirName(d.Name())
			if err != nil {
				return nil // Not a festival directory with ID
			}

			// Parse the ID
			idPrefix, counter, err := ParseID(id)
			if err != nil {
				return nil
			}

			// Only count matching prefixes
			if idPrefix == prefix && counter > maxCounter {
				maxCounter = counter
			}

			return nil
		})

		if err != nil {
			// Continue scanning other directories even if one fails
			continue
		}
	}

	return maxCounter + 1, nil
}

// GenerateID creates a unique festival ID for the given name.
// It extracts initials from the name and finds the next available counter
// by scanning all festival directories.
func GenerateID(ctx context.Context, name string, festivalsRoot string) (string, error) {
	initials := ExtractInitials(name)

	counter, err := FindNextCounter(ctx, festivalsRoot, initials)
	if err != nil {
		return "", errors.Wrap(err, "finding next counter").
			WithField("initials", initials)
	}

	return FormatID(initials, counter), nil
}

// RitualPrefix is prepended to ritual festival IDs.
const RitualPrefix = "RI-"

// ritualRunPattern matches hex run counters appended to ritual directory names (e.g., -0001, -00FF)
var ritualRunPattern = regexp.MustCompile(`-([0-9A-Fa-f]{4})$`)

// GenerateRitualID creates a ritual festival ID with RI- prefix.
// Format: RI-XX0001 where XX is derived from the festival name.
func GenerateRitualID(ctx context.Context, name string, festivalsRoot string) (string, error) {
	baseID, err := GenerateID(ctx, name, festivalsRoot)
	if err != nil {
		return "", err
	}
	return RitualPrefix + baseID, nil
}

// IsRitualID checks if an ID has the RI- prefix.
func IsRitualID(id string) bool {
	return strings.HasPrefix(id, RitualPrefix)
}

// FormatHexCounter formats a run number as a 4-digit zero-padded uppercase hex string.
// Examples: 1 -> "0001", 10 -> "000A", 255 -> "00FF", 65535 -> "FFFF"
func FormatHexCounter(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 0xFFFF {
		return fmt.Sprintf("%X", n) // No padding for overflow
	}
	return fmt.Sprintf("%04X", n)
}

// ParseHexCounter extracts the hex run counter from a ritual run directory name.
// The counter is the last 4 hex characters after the final hyphen.
// Returns 0 and an error if no valid counter is found.
func ParseHexCounter(dirName string) (int, error) {
	matches := ritualRunPattern.FindStringSubmatch(dirName)
	if matches == nil {
		return 0, errors.NotFound("no hex counter found").WithField("dirName", dirName)
	}
	n, err := strconv.ParseInt(matches[1], 16, 64)
	if err != nil {
		return 0, errors.Wrap(err, "parsing hex counter").WithField("counter", matches[1])
	}
	return int(n), nil
}

// FindNextRitualRun scans active/ and dungeon/ for the highest existing
// run counter for the given ritual directory name prefix, and returns the next one.
// For dungeon statuses, also scans inside date subdirectories.
func FindNextRitualRun(ctx context.Context, festivalsRoot, ritualDirName string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	maxRun := 0

	// Scan directories where ritual runs could be
	scanDirs := []string{"active", "dungeon/completed", "dungeon/archived"}
	for _, status := range scanDirs {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		statusPath := filepath.Join(festivalsRoot, status)

		// Use WalkDir to recurse into date subdirectories
		err := filepath.WalkDir(statusPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip inaccessible directories
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if !d.IsDir() {
				return nil
			}

			name := d.Name()
			if !strings.HasPrefix(name, ritualDirName+"-") {
				return nil
			}
			run, parseErr := ParseHexCounter(name)
			if parseErr != nil {
				return nil
			}
			if run > maxRun {
				maxRun = run
			}
			return nil
		})
		if err != nil {
			continue
		}
	}

	return maxRun + 1, nil
}

// taskRefPattern matches task references like FEST-123456 or [FEST-123456]
var taskRefPattern = regexp.MustCompile(`FEST-(\d{6})`)

// Validate checks if a string is a valid task reference format.
// Valid format: FEST-xxxxxx where x is a digit
func Validate(ref string) bool {
	return taskRefPattern.MatchString(ref)
}

// ExtractFromMessage extracts all task references from a commit message.
// Looks for patterns like FEST-123456 or [FEST-123456]
func ExtractFromMessage(message string) []string {
	matches := taskRefPattern.FindAllString(message, -1)
	if matches == nil {
		return []string{}
	}
	return matches
}
