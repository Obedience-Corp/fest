package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Service provides all workflow operations.
// It implements WorkflowInitializer, WorkflowReader, WorkflowWriter, and Crawler interfaces.
type Service struct {
	root       string  // Workflow root path
	schemaPath string  // Path to .workflow.yaml
	schema     *Schema // Loaded schema (nil if not loaded)
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// NewService creates a new workflow Service for the given root directory.
// The service will look for a .workflow.yaml file in the root directory.
func NewService(root string, opts ...ServiceOption) *Service {
	s := &Service{
		root:       root,
		schemaPath: filepath.Join(root, SchemaFileName),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithSchema sets a pre-loaded schema on the service.
// Useful for testing or when the schema has already been loaded.
func WithSchema(schema *Schema) ServiceOption {
	return func(s *Service) {
		s.schema = schema
	}
}

// Root returns the workflow root directory path.
func (s *Service) Root() string {
	return s.root
}

// HasParentFlow checks if any parent directory of dir contains a .workflow.yaml file.
// Returns the path to the parent schema file and true if found, empty string and false otherwise.
// This is used to prevent flow nesting.
func HasParentFlow(dir string) (string, bool) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	current := filepath.Dir(absDir)
	for current != "/" && current != "." && current != absDir {
		schemaPath := filepath.Join(current, SchemaFileName)
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
}

// Schema returns the loaded workflow schema, or nil if not loaded.
func (s *Service) Schema() *Schema {
	return s.schema
}

// HasSchema returns true if a workflow schema exists at the root path.
func (s *Service) HasSchema() bool {
	if s.schema != nil {
		return true
	}
	// Check if schema file exists
	_, err := LoadSchema(context.Background(), s.schemaPath)
	return err == nil
}

// LoadSchema loads the workflow schema from disk.
// Returns an error if the schema file doesn't exist or is invalid.
func (s *Service) LoadSchema(ctx context.Context) error {
	schema, err := LoadSchema(ctx, s.schemaPath)
	if err != nil {
		return err
	}
	s.schema = schema
	return nil
}

// InitOptions configures workflow initialization.
type InitOptions struct {
	Force         bool // Overwrite existing files
	SchemaVersion int  // Schema version to use (0 or 1 = v1, 2 = v2)
}

// InitResult contains what was created during init.
type InitResult struct {
	CreatedDirs  []string
	CreatedFiles []string
	Skipped      []string
}

// SyncOptions configures sync behavior.
type SyncOptions struct {
	DryRun bool // Preview without making changes
}

// SyncResult contains what was synced.
type SyncResult struct {
	Created  []string // Directories created
	Existing []string // Directories that already exist
}

// MigrateOptions configures migration behavior.
type MigrateOptions struct {
	DryRun bool // Preview without making changes
	Force  bool // Skip confirmation
}

// MigrateResult contains migration outcomes.
type MigrateResult struct {
	Created   []string // New directories/files
	Preserved []string // Existing items kept
	Schema    *Schema  // Generated schema
}

// StatusResult contains workflow statistics.
type StatusResult struct {
	Name           string         // Workflow name
	Location       string         // Root path
	Counts         map[string]int // Items per status
	TotalItems     int
	LastTransition *HistoryEntry // Most recent transition
}

// ShowOptions configures show output.
type ShowOptions struct {
	Tree   bool // Display as tree
	Schema bool // Show raw schema
}

// ShowResult contains workflow structure.
type ShowResult struct {
	Name        string
	Description string
	Directories []DirectoryInfo
	SchemaRaw   string // Raw YAML if requested
}

// DirectoryInfo describes a directory for display.
type DirectoryInfo struct {
	Path        string
	Description string
	Order       int
	ItemCount   int
	Children    []DirectoryInfo
}

// ListOptions configures list output.
type ListOptions struct {
	All  bool // List all statuses
	JSON bool // Output as JSON
}

// ListResult contains items in a status.
type ListResult struct {
	Status string
	Items  []Item
}

// HistoryOptions configures history queries.
type HistoryOptions struct {
	Limit int    // Max entries (0 = all)
	Item  string // Filter by item name
	JSON  bool   // Output as JSON
}

// MoveOptions configures move behavior.
type MoveOptions struct {
	Reason string // Reason for move (logged to history)
	Force  bool   // Skip transition validation
}

// MoveResult contains move outcomes.
type MoveResult struct {
	Item   string
	From   string
	To     string
	Reason string
}

// CrawlOptions configures crawl behavior.
type CrawlOptions struct {
	Status string // Specific status to crawl (empty = all)
	All    bool   // Include recently modified items
}

// CrawlResult contains crawl session summary.
type CrawlResult struct {
	Reviewed    int
	Kept        int
	Moved       int
	Skipped     int
	Transitions map[string]int // "from → to": count
}

// resolvePath converts a status path to an absolute filesystem path.
func (s *Service) resolvePath(status string) string {
	return filepath.Join(s.root, status)
}

// Init creates a new workflow with the default structure.
// It creates the schema file, all directories defined in the schema,
// and OBEY.md documentation files in active/, ready/, and dungeon/.
// Returns ErrSchemaExists if a schema already exists and Force is false.
// Returns FlowNestedError if attempting to create a flow inside another flow.
func (s *Service) Init(ctx context.Context, opts InitOptions) (*InitResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	result := &InitResult{
		CreatedDirs:  []string{},
		CreatedFiles: []string{},
		Skipped:      []string{},
	}

	// Check for flow nesting - cannot create a flow inside another flow
	if parentPath, found := HasParentFlow(s.root); found {
		return nil, &FlowNestedError{ParentSchemaPath: parentPath}
	}

	// Check if schema already exists
	if _, err := os.Stat(s.schemaPath); err == nil {
		if !opts.Force {
			return nil, ErrSchemaExists
		}
		result.Skipped = append(result.Skipped, s.schemaPath)
	}

	// Get default schema
	var schema *Schema
	if opts.SchemaVersion == 2 {
		schema = DefaultSchemaV2()
	} else {
		schema = DefaultSchema()
	}

	// Write schema file
	data, err := yaml.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := os.WriteFile(s.schemaPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write schema file: %w", err)
	}
	result.CreatedFiles = append(result.CreatedFiles, s.schemaPath)

	// Store schema
	s.schema = schema

	// Create directories
	for _, dirPath := range schema.AllDirectories() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		fullPath := s.resolvePath(dirPath)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
		result.CreatedDirs = append(result.CreatedDirs, dirPath)
	}

	// Create OBEY.md files based on schema version
	var obeyFiles []struct {
		path        string
		getTemplate func() ([]byte, error)
	}
	if schema.Version == 2 {
		obeyFiles = []struct {
			path        string
			getTemplate func() ([]byte, error)
		}{
			{filepath.Join(s.root, "dungeon", "OBEY.md"), GetDungeonOBEYTemplate},
		}
	} else {
		obeyFiles = []struct {
			path        string
			getTemplate func() ([]byte, error)
		}{
			{filepath.Join(s.root, "active", "OBEY.md"), GetActiveOBEYTemplate},
			{filepath.Join(s.root, "ready", "OBEY.md"), GetReadyOBEYTemplate},
			{filepath.Join(s.root, "dungeon", "OBEY.md"), GetDungeonOBEYTemplate},
		}
	}

	for _, obey := range obeyFiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Skip if exists and not forcing
		if _, err := os.Stat(obey.path); err == nil {
			if !opts.Force {
				result.Skipped = append(result.Skipped, obey.path)
				continue
			}
		}

		content, err := obey.getTemplate()
		if err != nil {
			return nil, fmt.Errorf("failed to read template for %s: %w", obey.path, err)
		}

		if err := os.WriteFile(obey.path, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", obey.path, err)
		}
		result.CreatedFiles = append(result.CreatedFiles, obey.path)
	}

	// Create .gitkeep in empty directories that won't get other files
	emptyDirs := []string{"dungeon/completed", "dungeon/archived", "dungeon/someday"}
	for _, dirPath := range emptyDirs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		gitkeepPath := filepath.Join(s.resolvePath(dirPath), ".gitkeep")
		if _, err := os.Stat(gitkeepPath); os.IsNotExist(err) {
			if err := os.WriteFile(gitkeepPath, []byte{}, 0644); err != nil {
				return nil, fmt.Errorf("failed to create .gitkeep in %s: %w", dirPath, err)
			}
			result.CreatedFiles = append(result.CreatedFiles, filepath.Join(dirPath, ".gitkeep"))
		}
	}

	return result, nil
}

// Sync ensures all directories defined in the schema exist.
// It creates any missing directories but does not remove extra directories.
// If DryRun is true, it reports what would be created without making changes.
func (s *Service) Sync(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load schema if not already loaded
	if s.schema == nil {
		if err := s.LoadSchema(ctx); err != nil {
			return nil, err
		}
	}

	result := &SyncResult{}

	// Check each directory
	for _, dirPath := range s.schema.AllDirectories() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		fullPath := s.resolvePath(dirPath)
		info, err := os.Stat(fullPath)
		if err == nil && info.IsDir() {
			result.Existing = append(result.Existing, dirPath)
			continue
		}

		// Directory doesn't exist
		if !opts.DryRun {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
			}
		}
		result.Created = append(result.Created, dirPath)
	}

	return result, nil
}

// List returns items in a status directory.
// The status can be a top-level directory (e.g., "active") or
// a nested path (e.g., "dungeon/completed").
func (s *Service) List(ctx context.Context, status string, opts ListOptions) (*ListResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load schema if not already loaded
	if s.schema == nil {
		if err := s.LoadSchema(ctx); err != nil {
			return nil, err
		}
	}

	// Validate status exists in schema
	if !s.schema.HasDirectory(status) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, status)
	}

	statusPath := s.resolvePath(status)
	entries, err := os.ReadDir(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrStatusNotFound, status)
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", status, err)
	}

	result := &ListResult{
		Status: status,
		Items:  make([]Item, 0, len(entries)),
	}

	// System files to exclude from listings
	excludedFiles := map[string]bool{
		"OBEY.md":  true,
		".gitkeep": true,
	}

	// For v2 root listing, also exclude dungeon dir and hidden entries
	isRootListing := status == "." && s.schema != nil && s.schema.Version == 2

	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Skip excluded files
		if excludedFiles[entry.Name()] {
			continue
		}

		// Skip hidden files and dungeon when listing root in v2
		if isRootListing {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if entry.Name() == "dungeon" && entry.IsDir() {
				continue
			}
		}

		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		item := Item{
			Name:    entry.Name(),
			Path:    filepath.Join(statusPath, entry.Name()),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

// Move moves an item from its current status to a new status.
// It validates the transition against schema rules unless Force is set.
// If the workflow has history tracking enabled, the move is logged.
func (s *Service) Move(ctx context.Context, item, to string, opts MoveOptions) (*MoveResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load schema if not already loaded
	if s.schema == nil {
		if err := s.LoadSchema(ctx); err != nil {
			return nil, err
		}
	}

	// Validate destination status exists
	if !s.schema.HasDirectory(to) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, to)
	}

	// Find the item in the workflow
	from, itemPath, err := s.findItem(ctx, item)
	if err != nil {
		return nil, err
	}

	// Validate transition unless Force is set
	if !opts.Force && !s.schema.IsValidTransition(from, to) {
		return nil, fmt.Errorf("%w: cannot move from %s to %s", ErrInvalidTransition, from, to)
	}

	// Destination path
	destPath := filepath.Join(s.resolvePath(to), filepath.Base(itemPath))

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyExists, destPath)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(s.resolvePath(to), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Move the item
	if err := os.Rename(itemPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to move item: %w", err)
	}

	result := &MoveResult{
		Item:   item,
		From:   from,
		To:     to,
		Reason: opts.Reason,
	}

	// Log to history if enabled
	if s.schema.TrackHistory {
		entry := HistoryEntry{
			Item:   item,
			From:   from,
			To:     to,
			Reason: opts.Reason,
		}
		if err := s.appendHistory(ctx, entry); err != nil {
			// Log but don't fail the move
		}
	}

	return result, nil
}

// findItem searches for an item in all status directories.
// Returns the status directory and full path where the item was found.
func (s *Service) findItem(ctx context.Context, itemName string) (string, string, error) {
	for _, status := range s.schema.AllDirectories() {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}

		itemPath := filepath.Join(s.resolvePath(status), itemName)
		if _, err := os.Stat(itemPath); err == nil {
			return status, itemPath, nil
		}
	}

	return "", "", fmt.Errorf("%w: %s", ErrItemNotFound, itemName)
}

// appendHistory adds an entry to the history file.
func (s *Service) appendHistory(ctx context.Context, entry HistoryEntry) error {
	// TODO: Implement history file writing
	return nil
}

// Migrate upgrades a legacy dungeon structure to a full workflow.
// It creates a .workflow.yaml file and any missing directories.
func (s *Service) Migrate(ctx context.Context, opts MigrateOptions) (*MigrateResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	result := &MigrateResult{}

	// Check for existing dungeon directory
	dungeonPath := s.resolvePath("dungeon")
	if _, err := os.Stat(dungeonPath); os.IsNotExist(err) {
		// No dungeon - just do a regular init
		schema := DefaultSchema()
		initResult, err := s.Init(ctx, InitOptions{Force: opts.Force})
		if err != nil {
			return nil, err
		}
		result.Created = append(result.Created, initResult.CreatedFiles...)
		result.Created = append(result.Created, initResult.CreatedDirs...)
		result.Schema = schema
		return result, nil
	}

	// Dungeon exists - preserve it and add workflow
	result.Preserved = append(result.Preserved, "dungeon/")

	// Check for subdirectories
	entries, err := os.ReadDir(dungeonPath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				result.Preserved = append(result.Preserved, "dungeon/"+entry.Name()+"/")
			}
		}
	}

	if !opts.DryRun {
		// Create schema and remaining directories
		schema := DefaultSchema()
		data, err := yaml.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema: %w", err)
		}

		if err := os.WriteFile(s.schemaPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write schema file: %w", err)
		}
		result.Created = append(result.Created, s.schemaPath)
		s.schema = schema

		// Create non-dungeon directories
		for _, dir := range []string{"active", "ready"} {
			dirPath := s.resolvePath(dir)
			if _, err := os.Stat(dirPath); os.IsNotExist(err) {
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
				result.Created = append(result.Created, dir+"/")
			}
		}

		// Create any missing dungeon subdirectories
		for childName := range schema.Directories["dungeon"].Children {
			childPath := s.resolvePath("dungeon/" + childName)
			if _, err := os.Stat(childPath); os.IsNotExist(err) {
				if err := os.MkdirAll(childPath, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory dungeon/%s: %w", childName, err)
				}
				result.Created = append(result.Created, "dungeon/"+childName+"/")
			}
		}

		// Create OBEY.md files if they don't exist
		obeyFiles := []struct {
			path        string
			getTemplate func() ([]byte, error)
		}{
			{filepath.Join(s.root, "active", "OBEY.md"), GetActiveOBEYTemplate},
			{filepath.Join(s.root, "ready", "OBEY.md"), GetReadyOBEYTemplate},
			{filepath.Join(s.root, "dungeon", "OBEY.md"), GetDungeonOBEYTemplate},
		}

		for _, obey := range obeyFiles {
			if _, err := os.Stat(obey.path); os.IsNotExist(err) {
				content, err := obey.getTemplate()
				if err != nil {
					return nil, fmt.Errorf("failed to read template for %s: %w", obey.path, err)
				}
				if err := os.WriteFile(obey.path, content, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", obey.path, err)
				}
				result.Created = append(result.Created, obey.path)
			}
		}

		result.Schema = schema
	}

	return result, nil
}

// MigrateV1ToV2Result contains the result of a v1 to v2 migration.
type MigrateV1ToV2Result struct {
	MovedItems   []string // Items moved
	RemovedDirs  []string // Directories removed
	SchemaUpdate bool     // Whether schema was updated
}

// MigrateV1ToV2 upgrades a v1 workflow to v2 dungeon-centric model.
// Moves active/ items to root, ready/ items to dungeon/ready/,
// removes empty active/ and ready/ dirs, and updates schema to v2.
func (s *Service) MigrateV1ToV2(ctx context.Context, dryRun bool) (*MigrateV1ToV2Result, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if s.schema == nil {
		if err := s.LoadSchema(ctx); err != nil {
			return nil, err
		}
	}

	if s.schema.Version == 2 {
		return nil, fmt.Errorf("workflow is already v2")
	}

	result := &MigrateV1ToV2Result{}

	// Ensure dungeon/ready exists
	dungeonReadyPath := s.resolvePath("dungeon/ready")
	if !dryRun {
		if err := os.MkdirAll(dungeonReadyPath, 0755); err != nil {
			return nil, fmt.Errorf("creating dungeon/ready: %w", err)
		}
	}

	// Move active/ items to root
	activePath := s.resolvePath("active")
	if entries, err := os.ReadDir(activePath); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if name == ".gitkeep" || name == "OBEY.md" {
				continue
			}
			src := filepath.Join(activePath, name)
			dst := filepath.Join(s.root, name)

			if !dryRun {
				if err := os.Rename(src, dst); err != nil {
					return nil, fmt.Errorf("moving %s to root: %w", name, err)
				}
			}
			result.MovedItems = append(result.MovedItems, fmt.Sprintf("active/%s → ./%s", name, name))
		}
	}

	// Move ready/ items to dungeon/ready/
	readyPath := s.resolvePath("ready")
	if entries, err := os.ReadDir(readyPath); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if name == ".gitkeep" || name == "OBEY.md" {
				continue
			}
			src := filepath.Join(readyPath, name)
			dst := filepath.Join(dungeonReadyPath, name)

			if !dryRun {
				if err := os.Rename(src, dst); err != nil {
					return nil, fmt.Errorf("moving %s to dungeon/ready: %w", name, err)
				}
			}
			result.MovedItems = append(result.MovedItems, fmt.Sprintf("ready/%s → dungeon/ready/%s", name, name))
		}
	}

	// Remove empty active/ and ready/ directories
	for _, dir := range []string{activePath, readyPath} {
		if entries, err := os.ReadDir(dir); err == nil {
			isEmpty := true
			for _, e := range entries {
				if e.Name() != ".gitkeep" && e.Name() != "OBEY.md" {
					isEmpty = false
					break
				}
			}
			if isEmpty && !dryRun {
				if err := os.RemoveAll(dir); err == nil {
					result.RemovedDirs = append(result.RemovedDirs, dir)
				}
			} else if isEmpty {
				result.RemovedDirs = append(result.RemovedDirs, dir)
			}
		}
	}

	// Update schema to v2
	if !dryRun {
		newSchema := DefaultSchemaV2()
		data, err := yaml.Marshal(newSchema)
		if err != nil {
			return nil, fmt.Errorf("marshaling v2 schema: %w", err)
		}
		if err := os.WriteFile(s.schemaPath, data, 0644); err != nil {
			return nil, fmt.Errorf("writing v2 schema: %w", err)
		}
		s.schema = newSchema
	}
	result.SchemaUpdate = true

	return result, nil
}

// Crawl interactively reviews items across statuses.
// This is a placeholder implementation - full TUI will be implemented later.
func (s *Service) Crawl(ctx context.Context, opts CrawlOptions) (*CrawlResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load schema if not already loaded
	if s.schema == nil {
		if err := s.LoadSchema(ctx); err != nil {
			return nil, err
		}
	}

	result := &CrawlResult{
		Transitions: make(map[string]int),
	}

	// Determine which statuses to crawl
	statuses := []string{}
	if opts.Status != "" {
		statuses = []string{opts.Status}
	} else {
		statuses = s.schema.AllDirectories()
	}

	// Count items in each status
	for _, status := range statuses {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		listResult, err := s.List(ctx, status, ListOptions{})
		if err != nil {
			continue
		}
		result.Reviewed += len(listResult.Items)
		result.Kept += len(listResult.Items) // For now, just mark as kept
	}

	return result, nil
}
