package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSchema(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	tests := []struct {
		name      string
		file      string
		wantErr   bool
		errIs     error
		checkFunc func(t *testing.T, s *Schema)
	}{
		{
			name:    "valid complete schema",
			file:    "valid_complete.yaml",
			wantErr: false,
			checkFunc: func(t *testing.T, s *Schema) {
				if s.Name != "Test Workflow" {
					t.Errorf("Name = %q, want %q", s.Name, "Test Workflow")
				}
				if s.DefaultStatus != "active" {
					t.Errorf("DefaultStatus = %q, want %q", s.DefaultStatus, "active")
				}
				if !s.TrackHistory {
					t.Error("TrackHistory = false, want true")
				}
				if len(s.Directories) != 3 {
					t.Errorf("len(Directories) = %d, want 3", len(s.Directories))
				}
				dungeon := s.Directories["dungeon"]
				if !dungeon.Nested {
					t.Error("dungeon.Nested = false, want true")
				}
				if len(dungeon.Children) != 3 {
					t.Errorf("len(dungeon.Children) = %d, want 3", len(dungeon.Children))
				}
			},
		},
		{
			name:    "valid minimal schema",
			file:    "valid_minimal.yaml",
			wantErr: false,
			checkFunc: func(t *testing.T, s *Schema) {
				if len(s.Directories) != 1 {
					t.Errorf("len(Directories) = %d, want 1", len(s.Directories))
				}
				if _, ok := s.Directories["inbox"]; !ok {
					t.Error("expected inbox directory")
				}
			},
		},
		{
			name:    "invalid - no directories",
			file:    "invalid_no_directories.yaml",
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name:    "invalid - nested without children",
			file:    "invalid_nested_no_children.yaml",
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name:    "invalid - bad transition target",
			file:    "invalid_bad_transition.yaml",
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name:    "invalid - bad default status",
			file:    "invalid_bad_default.yaml",
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name:    "invalid - malformed YAML",
			file:    "invalid_yaml.yaml",
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name:    "file not found",
			file:    "nonexistent.yaml",
			wantErr: true,
			errIs:   ErrNoSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(testdataDir, tt.file)
			schema, err := LoadSchema(ctx, path)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("error = %v, want %v", err, tt.errIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, schema)
			}
		})
	}
}

func TestLoadSchema_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := LoadSchema(ctx, "testdata/valid_complete.yaml")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestLoadSchema_EmptyFile(t *testing.T) {
	// Create a temp empty file
	dir := t.TempDir()
	path := filepath.Join(dir, ".workflow.yaml")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := LoadSchema(ctx, path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !errors.Is(err, ErrInvalidSchema) {
		t.Errorf("error = %v, want ErrInvalidSchema", err)
	}
}

func TestFindSchema(t *testing.T) {
	// Create temp directory structure
	dir := t.TempDir()

	// Resolve symlinks for the temp dir (macOS uses /private/var symlinks)
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(dir, "level1", "level2")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create schema in root
	schemaContent := `version: 1
type: status-workflow
directories:
  active:
    description: "Work in progress"
`
	schemaPath := filepath.Join(dir, SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		startPath string
		wantRoot  string
		wantErr   bool
		errIs     error
	}{
		{
			name:      "find from root",
			startPath: dir,
			wantRoot:  dir,
			wantErr:   false,
		},
		{
			name:      "find from subdirectory",
			startPath: subdir,
			wantRoot:  dir,
			wantErr:   false,
		},
		{
			name:      "find from file in subdirectory",
			startPath: filepath.Join(subdir, "somefile.txt"),
			wantRoot:  dir,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create the file if testing file path
			if tt.name == "find from file in subdirectory" {
				if err := os.WriteFile(tt.startPath, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			root, schema, err := FindSchema(ctx, tt.startPath)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("error = %v, want %v", err, tt.errIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if root != tt.wantRoot {
				t.Errorf("root = %q, want %q", root, tt.wantRoot)
			}

			if schema == nil {
				t.Error("schema is nil")
			}
		})
	}
}

func TestFindSchema_NoSchemaFound(t *testing.T) {
	dir := t.TempDir()

	ctx := context.Background()
	_, _, err := FindSchema(ctx, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoSchema) {
		t.Errorf("error = %v, want ErrNoSchema", err)
	}
}

func TestFindSchema_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := FindSchema(ctx, t.TempDir())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestFindSchema_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Nanosecond) // Ensure timeout

	_, _, err := FindSchema(ctx, t.TempDir())
	if err == nil {
		t.Fatal("expected error for timed out context")
	}
}

func TestSchema_Validate(t *testing.T) {
	tests := []struct {
		name    string
		schema  Schema
		wantErr bool
		errIs   error
	}{
		{
			name: "valid simple schema",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {Description: "Active items"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with nested directories",
			schema: Schema{
				Directories: map[string]Directory{
					"dungeon": {
						Description: "Archive",
						Nested:      true,
						Children: map[string]Directory{
							"completed": {Description: "Done"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with transition opts to nested parent",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {
						Description:    "Active items",
						TransitionOpts: []string{"dungeon"},
					},
					"dungeon": {
						Description: "Archive",
						Nested:      true,
						Children: map[string]Directory{
							"completed": {Description: "Done"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - no directories",
			schema: Schema{
				Directories: map[string]Directory{},
			},
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name: "invalid - nested without children",
			schema: Schema{
				Directories: map[string]Directory{
					"dungeon": {
						Description: "Archive",
						Nested:      true,
					},
				},
			},
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name: "invalid - bad transition target",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {
						Description:    "Active",
						TransitionOpts: []string{"nonexistent"},
					},
				},
			},
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
		{
			name: "invalid - bad default status",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {Description: "Active"},
				},
				DefaultStatus: "nonexistent",
			},
			wantErr: true,
			errIs:   ErrInvalidSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("error = %v, want %v", err, tt.errIs)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSchema_HasDirectory(t *testing.T) {
	schema := Schema{
		Directories: map[string]Directory{
			"active": {Description: "Active"},
			"dungeon": {
				Description: "Archive",
				Nested:      true,
				Children: map[string]Directory{
					"completed": {Description: "Done"},
					"archived":  {Description: "Old"},
				},
			},
		},
	}

	tests := []struct {
		path string
		want bool
	}{
		{"active", true},
		{"dungeon/completed", true},
		{"dungeon/archived", true},
		{"nonexistent", false},
		{"dungeon/nonexistent", false},
		{"active/child", false}, // active is not nested
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := schema.HasDirectory(tt.path)
			if got != tt.want {
				t.Errorf("HasDirectory(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSchema_GetDirectory(t *testing.T) {
	schema := Schema{
		Directories: map[string]Directory{
			"active": {Description: "Active", Order: 1},
			"dungeon": {
				Description: "Archive",
				Nested:      true,
				Children: map[string]Directory{
					"completed": {Description: "Done", Order: 1},
				},
			},
		},
	}

	tests := []struct {
		path     string
		wantOk   bool
		wantDesc string
	}{
		{"active", true, "Active"},
		{"dungeon/completed", true, "Done"},
		{"nonexistent", false, ""},
		{"dungeon/nonexistent", false, ""},
		{"too/many/levels", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			dir, ok := schema.GetDirectory(tt.path)
			if ok != tt.wantOk {
				t.Errorf("GetDirectory(%q) ok = %v, want %v", tt.path, ok, tt.wantOk)
			}
			if ok && dir.Description != tt.wantDesc {
				t.Errorf("GetDirectory(%q).Description = %q, want %q", tt.path, dir.Description, tt.wantDesc)
			}
		})
	}
}

func TestSchema_AllDirectories(t *testing.T) {
	tests := []struct {
		name   string
		schema Schema
		want   []string
	}{
		{
			name: "flat directories",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {Description: "Active"},
					"ready":  {Description: "Ready"},
				},
			},
			want: []string{"active", "ready"},
		},
		{
			name: "nested directories",
			schema: Schema{
				Directories: map[string]Directory{
					"active": {Description: "Active"},
					"dungeon": {
						Description: "Archive",
						Nested:      true,
						Children: map[string]Directory{
							"completed": {Description: "Done"},
							"archived":  {Description: "Old"},
						},
					},
				},
			},
			want: []string{"active", "dungeon/completed", "dungeon/archived"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.schema.AllDirectories()

			// Convert to set for comparison (order may vary)
			gotSet := make(map[string]bool)
			for _, p := range got {
				gotSet[p] = true
			}
			wantSet := make(map[string]bool)
			for _, p := range tt.want {
				wantSet[p] = true
			}

			if len(gotSet) != len(wantSet) {
				t.Errorf("AllDirectories() = %v, want %v", got, tt.want)
				return
			}

			for p := range wantSet {
				if !gotSet[p] {
					t.Errorf("AllDirectories() missing %q", p)
				}
			}
		})
	}
}

func TestSchema_IsValidTransition(t *testing.T) {
	schema := Schema{
		Directories: map[string]Directory{
			"active": {
				Description:    "Active",
				TransitionOpts: []string{"ready", "dungeon"},
			},
			"ready": {
				Description:    "Ready",
				TransitionOpts: []string{"active", "dungeon"},
			},
			"dungeon": {
				Description: "Archive",
				Nested:      true,
				Children: map[string]Directory{
					"completed": {Description: "Done"},
					"archived":  {Description: "Old"},
				},
			},
		},
	}

	tests := []struct {
		from string
		to   string
		want bool
	}{
		// Direct transitions
		{"active", "ready", true},
		{"active", "dungeon/completed", true},
		{"active", "dungeon/archived", true},
		{"ready", "active", true},

		// Invalid transitions
		{"active", "nonexistent", false},
		{"nonexistent", "active", false},

		// From nested (no restrictions)
		{"dungeon/completed", "active", true},
		{"dungeon/completed", "ready", true},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			got := schema.IsValidTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("IsValidTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestSchema_IsValidTransition_NoRestrictions(t *testing.T) {
	// Schema without transition_opts = all transitions allowed
	schema := Schema{
		Directories: map[string]Directory{
			"active": {Description: "Active"},
			"ready":  {Description: "Ready"},
		},
	}

	if !schema.IsValidTransition("active", "ready") {
		t.Error("expected transition to be allowed when no restrictions")
	}
	if !schema.IsValidTransition("ready", "active") {
		t.Error("expected transition to be allowed when no restrictions")
	}
}
