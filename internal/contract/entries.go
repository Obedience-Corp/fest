// Package contract defines the fest CLI's entries for the central watcher contract.
// These entries declare every file and directory that fest manages, so the obey daemon
// knows what to watch via .campaign/watchers.yaml.
package contract

import (
	"github.com/obediencecorp/obey-shared/contract"
)

// FestEntries returns all contract entries owned by the fest CLI.
// These entries are written to .campaign/watchers.yaml via contract.WriteEntries().
// All paths are relative to the campaign root.
//
// Status directory entries must match the canonical list in internal/id/generator.go
// (StatusDirectories). If fest adds or removes a status directory, both must update.
func FestEntries() []contract.Entry {
	return []contract.Entry{
		// ============================================================
		// Methodology State Files (festivals/.festival/)
		// ============================================================

		// Authoritative festival lifecycle event log (append-only JSONL).
		// Records every create, move, archive, and status change.
		{
			ID:     "festival-events",
			Path:   "festivals/.festival/.state/festival_events.jsonl",
			Type:   contract.TypeFestivalLifecycle,
			Format: contract.FormatJSONL,
			Watch:  contract.WatchAppend,
			Owner:  contract.OwnerFest,
		},

		// Festival type definitions (phase structures, auto-phases).
		{
			ID:     "festival-types",
			Path:   "festivals/.festival/festival_types.yaml",
			Type:   contract.TypeFestivalTypes,
			Format: contract.FormatYAML,
			Watch:  contract.WatchFile,
			Owner:  contract.OwnerFest,
		},

		// Festival ID registry backup.
		{
			ID:     "festival-id-registry",
			Path:   "festivals/.festival/.state/id_registry.yaml.backup",
			Type:   contract.TypeFestivalRegistry,
			Format: contract.FormatYAML,
			Watch:  contract.WatchFile,
			Owner:  contract.OwnerFest,
		},

		// Methodology file integrity checksums.
		{
			ID:     "festival-checksums",
			Path:   "festivals/.festival/.state/.fest-checksums.json",
			Type:   contract.TypeFestivalIntegrity,
			Format: contract.FormatJSON,
			Watch:  contract.WatchFile,
			Owner:  contract.OwnerFest,
		},

		// Festival-to-project navigation mapping (written by fest link).
		{
			ID:     "festival-navigation",
			Path:   ".campaign/fest/navigation.yaml",
			Type:   contract.TypeFestivalNavigation,
			Format: contract.FormatYAML,
			Watch:  contract.WatchFile,
			Owner:  contract.OwnerFest,
		},

		// ============================================================
		// Festival Status Directories
		// ============================================================

		{
			ID:     "festivals-planning",
			Path:   "festivals/planning/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "planning",
		},
		{
			ID:     "festivals-active",
			Path:   "festivals/active/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "active",
		},
		{
			ID:     "festivals-ready",
			Path:   "festivals/ready/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "ready",
		},
		{
			ID:     "festivals-ritual",
			Path:   "festivals/ritual/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "ritual",
		},

		// Dungeon subdirectories
		{
			ID:     "festivals-dungeon-archived",
			Path:   "festivals/dungeon/archived/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "archived",
		},
		{
			ID:     "festivals-dungeon-completed",
			Path:   "festivals/dungeon/completed/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "completed",
		},
		{
			ID:     "festivals-dungeon-someday",
			Path:   "festivals/dungeon/someday/",
			Type:   contract.TypeFestivalStatusDir,
			Format: contract.FormatDirectory,
			Watch:  contract.WatchDirectory,
			Owner:  contract.OwnerFest,
			Status: "someday",
		},

		// ============================================================
		// Per-Festival Template Entries
		//
		// These use {festival} as a placeholder. The daemon expands them
		// into concrete watchers when it discovers a festival via the
		// status_dir entries above.
		// ============================================================

		// Festival metadata (fest.yaml) -- contains project_path, status_history,
		// type_config, quality_gates. Most information-dense per-festival file.
		{
			ID:       "festival-metadata",
			Path:     "{festival}/fest.yaml",
			Type:     contract.TypeFestivalMetadata,
			Format:   contract.FormatYAML,
			Watch:    contract.WatchFile,
			Owner:    contract.OwnerFest,
			Template: true,
			FallbackPaths: []string{
				"{festival}/festival.yaml",
			},
		},

		// Task progress event log (append-only JSONL of task completions).
		{
			ID:     "festival-progress",
			Path:   "{festival}/.fest/progress_events.jsonl",
			Type:   contract.TypeFestivalProgress,
			Format: contract.FormatJSONL,
			Watch:  contract.WatchAppend,
			Owner:  contract.OwnerFest,
			Template: true,
			FallbackPaths: []string{
				"{festival}/.fest/progress.yaml",
				"{festival}/.fest/progress.json",
			},
		},

		// Task documents (markdown files with fest_status in frontmatter).
		{
			ID:          "festival-tasks",
			Path:        "{festival}/**/*.md",
			Type:        contract.TypeFestivalTask,
			Format:      contract.FormatMarkdownFrontmatter,
			Watch:       contract.WatchDirectory,
			Owner:       contract.OwnerFest,
			Template:    true,
			StatusField: "fest_status",
		},
	}
}
