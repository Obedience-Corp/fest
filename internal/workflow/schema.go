// Package workflow provides status workflow management for fest.
//
// A workflow defines a set of status directories that items can move between,
// with optional transition rules and history tracking. The workflow is configured
// via a .workflow.yaml file in the workflow root directory.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SchemaFileName is the name of the workflow schema file.
const SchemaFileName = ".workflow.yaml"

// DefaultHistoryFile is the default history file name.
const DefaultHistoryFile = ".workflow-history.jsonl"

// CurrentSchemaVersion is the current schema version.
const CurrentSchemaVersion = 1

// SchemaType is the type field value for status workflows.
const SchemaType = "status-workflow"

// Schema represents a .workflow.yaml configuration file.
// It defines the structure and behavior of a status workflow.
type Schema struct {
	// Version is the schema version (currently 1).
	Version int `yaml:"version" json:"version"`

	// Type identifies this as a status workflow (always "status-workflow").
	Type string `yaml:"type" json:"type"`

	// Name is the human-readable workflow name.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Description explains the workflow's purpose.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Directories defines the status directories in this workflow.
	Directories map[string]Directory `yaml:"directories" json:"directories"`

	// DefaultStatus is the initial status for new items.
	DefaultStatus string `yaml:"default_status,omitempty" json:"default_status,omitempty"`

	// TrackHistory enables transition logging.
	TrackHistory bool `yaml:"track_history,omitempty" json:"track_history,omitempty"`

	// HistoryFile is the path to the history file (relative to workflow root).
	HistoryFile string `yaml:"history_file,omitempty" json:"history_file,omitempty"`

	// Metadata contains user-defined key-value pairs.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`

	// AutoCommit configures automatic git commits on flow moves.
	AutoCommit AutoCommitConfig `yaml:"auto_commit,omitempty" json:"auto_commit,omitempty"`
}

// AutoCommitConfig controls automatic git commits after flow moves.
type AutoCommitConfig struct {
	// Enabled turns auto-commit on/off globally.
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Transitions defines per-transition overrides.
	Transitions []TransitionRule `yaml:"transitions,omitempty" json:"transitions,omitempty"`
}

// TransitionRule defines an auto-commit override for a specific transition.
type TransitionRule struct {
	From   string `yaml:"from" json:"from"`
	To     string `yaml:"to" json:"to"`
	Commit bool   `yaml:"commit" json:"commit"`
}

// ShouldAutoCommit returns whether a given transition should trigger a commit.
func (c *AutoCommitConfig) ShouldAutoCommit(from, to string) bool {
	// Check per-transition overrides first
	for _, t := range c.Transitions {
		if t.From == from && t.To == to {
			return t.Commit
		}
	}
	// Fall back to global setting
	return c.Enabled
}

// Directory represents a status directory configuration.
// Directories can be flat or nested (containing child directories).
type Directory struct {
	// Description explains the directory's purpose.
	Description string `yaml:"description" json:"description"`

	// Order determines display ordering (lower = first).
	Order int `yaml:"order,omitempty" json:"order,omitempty"`

	// Nested indicates this directory has child subdirectories.
	Nested bool `yaml:"nested,omitempty" json:"nested,omitempty"`

	// Children defines nested subdirectories (only if Nested is true).
	Children map[string]Directory `yaml:"children,omitempty" json:"children,omitempty"`

	// TransitionOpts lists valid move targets from this directory.
	// If nil or empty, all transitions are allowed.
	TransitionOpts []string `yaml:"transition_opts,omitempty" json:"transition_opts,omitempty"`
}

// HistoryEntry represents a single transition in the history log.
// Entries are stored as JSONL (one JSON object per line).
type HistoryEntry struct {
	// Timestamp is when the transition occurred.
	Timestamp time.Time `json:"timestamp"`

	// Item is the name of the item that was moved.
	Item string `json:"item"`

	// From is the source status (e.g., "active", "dungeon/someday").
	From string `json:"from"`

	// To is the destination status.
	To string `json:"to"`

	// Reason is an optional note about why the move was made.
	Reason string `json:"reason,omitempty"`
}

// Item represents an item in a workflow status directory.
type Item struct {
	// Name is the item's name (filename or directory name).
	Name string `json:"name"`

	// Path is the full filesystem path to the item.
	Path string `json:"path"`

	// IsDir indicates whether this is a directory.
	IsDir bool `json:"is_dir"`

	// ModTime is when the item was last modified.
	ModTime time.Time `json:"mod_time"`

	// Size is the file size in bytes, or file count for directories.
	Size int64 `json:"size"`
}

// LoadSchema loads and validates a workflow schema from a file.
func LoadSchema(ctx context.Context, path string) (*Schema, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSchema
		}
		return nil, fmt.Errorf("failed to read schema file %s: %w", path, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: file is empty", ErrInvalidSchema)
	}

	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSchema, err)
	}

	if err := schema.Validate(); err != nil {
		return nil, err
	}

	return &schema, nil
}

// FindSchema walks up from startPath looking for a .workflow.yaml file.
// Returns the workflow root directory (where the schema was found) and the loaded schema.
func FindSchema(ctx context.Context, startPath string) (string, *Schema, error) {
	if ctx.Err() != nil {
		return "", nil, ctx.Err()
	}

	dir := startPath

	// Resolve symlinks
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// If startPath is a file, start from its directory
	info, err := os.Stat(dir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}

		schemaPath := filepath.Join(dir, SchemaFileName)
		schema, err := LoadSchema(ctx, schemaPath)
		if err == nil {
			return dir, schema, nil
		}

		// If the error is not "file not found", return it
		if err != ErrNoSchema && !os.IsNotExist(err) {
			return "", nil, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding schema
			return "", nil, ErrNoSchema
		}
		dir = parent
	}
}

// Validate checks the schema for internal consistency.
// Supports both v1 and v2 schemas with version-specific rules.
func (s *Schema) Validate() error {
	// Version 0 is treated as v1 (unset/legacy schemas)
	if s.Version != 0 && s.Version != 1 && s.Version != 2 {
		return fmt.Errorf("%w: unsupported schema version %d (must be 1 or 2)", ErrInvalidSchema, s.Version)
	}

	// Common validation
	if err := s.validateCommon(); err != nil {
		return err
	}

	// Version-specific validation
	if s.Version == 2 {
		return s.validateV2()
	}
	return nil
}

// validateCommon contains validation rules shared between v1 and v2.
func (s *Schema) validateCommon() error {
	// Rule 1: Must have at least one directory
	if len(s.Directories) == 0 {
		return fmt.Errorf("%w: must have at least one directory", ErrInvalidSchema)
	}

	// Rule 2: Nested directories must have children
	for name, dir := range s.Directories {
		if dir.Nested && len(dir.Children) == 0 {
			return fmt.Errorf("%w: nested directory %q must have children", ErrInvalidSchema, name)
		}
	}

	// Rule 3: Transition targets must reference valid directories
	allDirs := s.AllDirectories()
	dirSet := make(map[string]bool)
	for _, d := range allDirs {
		dirSet[d] = true
	}
	// Also add top-level nested dirs as valid targets (inferring children)
	for name, dir := range s.Directories {
		if dir.Nested {
			dirSet[name] = true
		}
	}

	for name, dir := range s.Directories {
		for _, opt := range dir.TransitionOpts {
			if !dirSet[opt] {
				return fmt.Errorf("%w: directory %q has invalid transition target %q", ErrInvalidSchema, name, opt)
			}
		}
		// Also validate children's transition opts
		if dir.Nested {
			for childName, child := range dir.Children {
				for _, opt := range child.TransitionOpts {
					if !dirSet[opt] {
						return fmt.Errorf("%w: directory %q has invalid transition target %q", ErrInvalidSchema, name+"/"+childName, opt)
					}
				}
			}
		}
	}

	// Rule 4: Default status must be a valid top-level directory
	if s.DefaultStatus != "" {
		if _, ok := s.Directories[s.DefaultStatus]; !ok {
			return fmt.Errorf("%w: default_status %q is not a valid directory", ErrInvalidSchema, s.DefaultStatus)
		}
	}

	return nil
}

// validateV2 enforces v2-specific rules: root is default, all named
// statuses live under dungeon/.
func (s *Schema) validateV2() error {
	// V2 must have "." as default status
	if s.DefaultStatus != "." {
		return fmt.Errorf("%w: v2 schema default_status must be \".\" (root), got %q", ErrInvalidSchema, s.DefaultStatus)
	}

	// V2 must have root "." directory
	if _, ok := s.Directories["."]; !ok {
		return fmt.Errorf("%w: v2 schema must have root directory \".\"", ErrInvalidSchema)
	}

	// V2: all non-root top-level directories must be nested (dungeon-like)
	for name, dir := range s.Directories {
		if name == "." {
			continue
		}
		if !dir.Nested {
			return fmt.Errorf("%w: v2 schema non-root directory %q must be nested (dungeon-centric model)", ErrInvalidSchema, name)
		}
	}

	return nil
}

// HasDirectory returns true if the schema has a directory at the given path.
// The path can be a top-level directory (e.g., "active") or a nested path (e.g., "dungeon/completed").
func (s *Schema) HasDirectory(path string) bool {
	_, ok := s.GetDirectory(path)
	return ok
}

// GetDirectory returns a directory by path (e.g., "active", "dungeon/completed").
// Returns nil and false if the directory doesn't exist.
func (s *Schema) GetDirectory(path string) (*Directory, bool) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return nil, false
	}

	// Top-level directory
	dir, ok := s.Directories[parts[0]]
	if !ok {
		return nil, false
	}

	if len(parts) == 1 {
		return &dir, true
	}

	// Nested directory (only support one level of nesting)
	if len(parts) == 2 {
		if !dir.Nested || dir.Children == nil {
			return nil, false
		}
		child, ok := dir.Children[parts[1]]
		if !ok {
			return nil, false
		}
		return &child, true
	}

	// More than 2 levels not supported
	return nil, false
}

// AllDirectories returns all directory paths including nested ones.
// Paths are in the format "name" for top-level or "parent/child" for nested.
func (s *Schema) AllDirectories() []string {
	var paths []string
	for name, dir := range s.Directories {
		if dir.Nested && len(dir.Children) > 0 {
			// For nested directories, only list children, not the parent itself
			for childName := range dir.Children {
				paths = append(paths, name+"/"+childName)
			}
		} else {
			paths = append(paths, name)
		}
	}
	return paths
}

// IsValidTransition checks if moving from one status to another is allowed.
// Returns true if the transition is permitted based on transition_opts.
func (s *Schema) IsValidTransition(from, to string) bool {
	fromDir, ok := s.GetDirectory(from)
	if !ok {
		return false
	}

	// No restrictions if transition_opts not defined or empty
	if len(fromDir.TransitionOpts) == 0 {
		return true
	}

	// Check if target is in allowed list
	for _, opt := range fromDir.TransitionOpts {
		if opt == to {
			return true
		}
		// Check if target is a child of a nested directory in the opts
		if strings.HasPrefix(to, opt+"/") {
			if parent, ok := s.Directories[opt]; ok && parent.Nested {
				return true
			}
		}
	}

	return false
}
