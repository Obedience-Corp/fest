package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// testWorkflow creates a temporary workflow directory for testing.
func testWorkflow(t *testing.T) (string, *Service) {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks for macOS /private/var
	dir, _ = filepath.EvalSymlinks(dir)
	return dir, NewService(dir)
}

// createTestSchema creates a .workflow.yaml in the given directory.
func createTestSchema(t *testing.T, dir string) {
	t.Helper()
	schema := DefaultSchema()
	svc := NewService(dir, WithSchema(schema))
	if _, err := svc.Init(context.Background(), InitOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestNewService(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)

	if svc.Root() != dir {
		t.Errorf("Root() = %q, want %q", svc.Root(), dir)
	}
	if svc.Schema() != nil {
		t.Error("Schema() should be nil before loading")
	}
}

func TestNewService_WithSchema(t *testing.T) {
	dir := t.TempDir()
	schema := DefaultSchema()
	svc := NewService(dir, WithSchema(schema))

	if svc.Schema() != schema {
		t.Error("Schema() should return the provided schema")
	}
}

func TestService_Init(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string)
		opts      InitOptions
		wantErr   bool
		errIs     error
		checkFunc func(t *testing.T, dir string, result *InitResult)
	}{
		{
			name:    "creates new workflow",
			wantErr: false,
			checkFunc: func(t *testing.T, dir string, result *InitResult) {
				// Schema file should exist
				schemaPath := filepath.Join(dir, SchemaFileName)
				if _, err := os.Stat(schemaPath); err != nil {
					t.Errorf("schema file not created: %v", err)
				}

				// Directories should exist
				expectedDirs := []string{"active", "ready", "dungeon/completed", "dungeon/archived", "dungeon/someday"}
				for _, d := range expectedDirs {
					if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
						t.Errorf("directory %q not created: %v", d, err)
					}
				}

				// OBEY.md files should exist in active, ready, and dungeon
				obeyFiles := []string{"active/OBEY.md", "ready/OBEY.md", "dungeon/OBEY.md"}
				for _, f := range obeyFiles {
					if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
						t.Errorf("OBEY.md file %q not created: %v", f, err)
					}
				}
			},
		},
		{
			name: "fails if schema exists",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, SchemaFileName), []byte("version: 1"), 0644)
			},
			wantErr: true,
			errIs:   ErrSchemaExists,
		},
		{
			name: "force overwrites existing schema",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, SchemaFileName), []byte("version: 1"), 0644)
			},
			opts:    InitOptions{Force: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, svc := testWorkflow(t)

			if tt.setup != nil {
				tt.setup(dir)
			}

			result, err := svc.Init(context.Background(), tt.opts)

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
				tt.checkFunc(t, dir, result)
			}
		})
	}
}

func TestService_Init_ContextCancellation(t *testing.T) {
	dir, svc := testWorkflow(t)
	_ = dir

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Init(ctx, InitOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestService_Sync(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string)
		opts      SyncOptions
		wantErr   bool
		checkFunc func(t *testing.T, dir string, result *SyncResult)
	}{
		{
			name: "creates missing directories",
			setup: func(dir string) {
				// Create schema but only one directory
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
				// Delete some directories to simulate missing
				_ = os.RemoveAll(filepath.Join(dir, "ready"))
				_ = os.RemoveAll(filepath.Join(dir, "dungeon/archived"))
			},
			checkFunc: func(t *testing.T, dir string, result *SyncResult) {
				if len(result.Created) == 0 {
					t.Error("expected some directories to be created")
				}
				// Verify directories were actually created
				for _, d := range result.Created {
					if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
						t.Errorf("directory %q not created: %v", d, err)
					}
				}
			},
		},
		{
			name: "dry run doesn't create directories",
			setup: func(dir string) {
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
				_ = os.RemoveAll(filepath.Join(dir, "ready"))
			},
			opts: SyncOptions{DryRun: true},
			checkFunc: func(t *testing.T, dir string, result *SyncResult) {
				// Should report directory to be created but not actually create it
				if len(result.Created) == 0 {
					t.Error("expected directory in Created list")
				}
				// Verify directory was NOT created
				if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
					t.Error("directory should not be created in dry run mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, svc := testWorkflow(t)

			if tt.setup != nil {
				tt.setup(dir)
			}

			result, err := svc.Sync(context.Background(), tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, dir, result)
			}
		})
	}
}

func TestService_Sync_NoSchema(t *testing.T) {
	dir, svc := testWorkflow(t)
	_ = dir

	_, err := svc.Sync(context.Background(), SyncOptions{})
	if err == nil {
		t.Fatal("expected error when no schema exists")
	}
	if !errors.Is(err, ErrNoSchema) {
		t.Errorf("error = %v, want ErrNoSchema", err)
	}
}

func TestService_List(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		setup     func(string)
		wantErr   bool
		errIs     error
		checkFunc func(t *testing.T, result *ListResult)
	}{
		{
			name:   "lists items in directory",
			status: "active",
			setup: func(dir string) {
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
				// Create some items
				_ = os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "active", "project-2"), 0755)
			},
			checkFunc: func(t *testing.T, result *ListResult) {
				if len(result.Items) != 2 {
					t.Errorf("got %d items, want 2", len(result.Items))
				}
				if result.Status != "active" {
					t.Errorf("Status = %q, want 'active'", result.Status)
				}
			},
		},
		{
			name:   "lists empty directory",
			status: "ready",
			setup: func(dir string) {
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
			},
			checkFunc: func(t *testing.T, result *ListResult) {
				if len(result.Items) != 0 {
					t.Errorf("got %d items, want 0", len(result.Items))
				}
			},
		},
		{
			name:   "lists nested directory",
			status: "dungeon/completed",
			setup: func(dir string) {
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
				_ = os.MkdirAll(filepath.Join(dir, "dungeon/completed", "old-project"), 0755)
			},
			checkFunc: func(t *testing.T, result *ListResult) {
				if len(result.Items) != 1 {
					t.Errorf("got %d items, want 1", len(result.Items))
				}
			},
		},
		{
			name:    "fails for invalid status",
			status:  "nonexistent",
			setup:   func(dir string) { createTestSchema(t, dir) },
			wantErr: true,
			errIs:   ErrInvalidStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, svc := testWorkflow(t)

			if tt.setup != nil {
				tt.setup(dir)
			}

			result, err := svc.List(context.Background(), tt.status, ListOptions{})

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
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestService_Move(t *testing.T) {
	tests := []struct {
		name      string
		item      string
		to        string
		opts      MoveOptions
		setup     func(string)
		wantErr   bool
		errIs     error
		checkFunc func(t *testing.T, dir string, result *MoveResult)
	}{
		{
			name: "moves item to valid destination",
			item: "project-1",
			to:   "ready",
			setup: func(dir string) {
				svc := NewService(dir)
				_, _ = svc.Init(context.Background(), InitOptions{})
				_ = os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
			},
			checkFunc: func(t *testing.T, dir string, result *MoveResult) {
				// Old location should not exist
				if _, err := os.Stat(filepath.Join(dir, "active", "project-1")); err == nil {
					t.Error("item should not exist at old location")
				}
				// New location should exist
				if _, err := os.Stat(filepath.Join(dir, "ready", "project-1")); err != nil {
					t.Errorf("item should exist at new location: %v", err)
				}
				if result.From != "active" {
					t.Errorf("From = %q, want 'active'", result.From)
				}
				if result.To != "ready" {
					t.Errorf("To = %q, want 'ready'", result.To)
				}
			},
		},
		{
			name: "moves to nested destination",
			item: "old-project",
			to:   "dungeon/completed",
			setup: func(dir string) {
				svc := NewService(dir)
				svc.Init(context.Background(), InitOptions{})
				os.MkdirAll(filepath.Join(dir, "active", "old-project"), 0755)
			},
			checkFunc: func(t *testing.T, dir string, result *MoveResult) {
				if _, err := os.Stat(filepath.Join(dir, "dungeon/completed", "old-project")); err != nil {
					t.Errorf("item should exist at dungeon/completed: %v", err)
				}
			},
		},
		{
			name: "fails for item not found",
			item: "nonexistent",
			to:   "ready",
			setup: func(dir string) {
				svc := NewService(dir)
				svc.Init(context.Background(), InitOptions{})
			},
			wantErr: true,
			errIs:   ErrItemNotFound,
		},
		{
			name: "fails for invalid destination",
			item: "project-1",
			to:   "nonexistent",
			setup: func(dir string) {
				svc := NewService(dir)
				svc.Init(context.Background(), InitOptions{})
				os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
			},
			wantErr: true,
			errIs:   ErrInvalidStatus,
		},
		{
			name: "force bypasses transition validation",
			item: "project-1",
			to:   "active", // Same status - normally not in transition_opts
			opts: MoveOptions{Force: true},
			setup: func(dir string) {
				svc := NewService(dir)
				svc.Init(context.Background(), InitOptions{})
				os.MkdirAll(filepath.Join(dir, "ready", "project-1"), 0755)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, svc := testWorkflow(t)

			if tt.setup != nil {
				tt.setup(dir)
			}

			result, err := svc.Move(context.Background(), tt.item, tt.to, tt.opts)

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
				tt.checkFunc(t, dir, result)
			}
		})
	}
}

func TestService_Move_ContextCancellation(t *testing.T) {
	dir, svc := testWorkflow(t)

	svc.Init(context.Background(), InitOptions{})
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Move(ctx, "project-1", "ready", MoveOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_HasSchema(t *testing.T) {
	t.Run("returns false when no schema", func(t *testing.T) {
		_, svc := testWorkflow(t)
		if svc.HasSchema() {
			t.Error("HasSchema() should return false when no schema exists")
		}
	})

	t.Run("returns true when schema exists", func(t *testing.T) {
		dir, svc := testWorkflow(t)
		svc.Init(context.Background(), InitOptions{})
		// Create new service to test file-based detection
		svc2 := NewService(dir)
		if !svc2.HasSchema() {
			t.Error("HasSchema() should return true when schema file exists")
		}
	})

	t.Run("returns true when schema loaded", func(t *testing.T) {
		_, svc := testWorkflow(t)
		svc = NewService(svc.Root(), WithSchema(DefaultSchema()))
		if !svc.HasSchema() {
			t.Error("HasSchema() should return true when schema is set")
		}
	})
}

func TestService_LoadSchema(t *testing.T) {
	t.Run("loads valid schema", func(t *testing.T) {
		dir, svc := testWorkflow(t)
		svc.Init(context.Background(), InitOptions{})
		svc2 := NewService(dir)

		err := svc2.LoadSchema(context.Background())
		if err != nil {
			t.Fatalf("LoadSchema() error = %v", err)
		}
		if svc2.Schema() == nil {
			t.Error("Schema() should not be nil after loading")
		}
	})

	t.Run("fails when no schema file", func(t *testing.T) {
		_, svc := testWorkflow(t)

		err := svc.LoadSchema(context.Background())
		if err == nil {
			t.Fatal("LoadSchema() should fail when no schema file exists")
		}
	})
}

func TestService_List_ContextCancellation(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})
	_ = dir

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.List(ctx, "active", ListOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_Sync_ContextCancellation(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})
	_ = dir

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Sync(ctx, SyncOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_Move_AlreadyExists(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})

	// Create an item in active and another with same name in ready
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
	os.MkdirAll(filepath.Join(dir, "ready", "project-1"), 0755)

	_, err := svc.Move(context.Background(), "project-1", "ready", MoveOptions{})
	if err == nil {
		t.Fatal("expected error when destination already exists")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestService_Move_WithReason(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)

	result, err := svc.Move(context.Background(), "project-1", "ready", MoveOptions{
		Reason: "Ready for review",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reason != "Ready for review" {
		t.Errorf("Reason = %q, want 'Ready for review'", result.Reason)
	}
}

func TestService_findItem(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})

	// Create item in nested directory
	os.MkdirAll(filepath.Join(dir, "dungeon/completed", "old-project"), 0755)

	status, path, err := svc.findItem(context.Background(), "old-project")
	if err != nil {
		t.Fatalf("findItem() error = %v", err)
	}
	if status != "dungeon/completed" {
		t.Errorf("status = %q, want 'dungeon/completed'", status)
	}
	if path == "" {
		t.Error("path should not be empty")
	}
}

func TestService_findItem_NotFound(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})
	_ = dir

	_, _, err := svc.findItem(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("error = %v, want ErrItemNotFound", err)
	}
}

func TestService_Migrate_NoDungeon(t *testing.T) {
	dir, svc := testWorkflow(t)

	result, err := svc.Migrate(context.Background(), MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Should have created directories and files (via Init)
	if len(result.Created) == 0 {
		t.Error("expected created items")
	}

	// Schema should be set
	if result.Schema == nil {
		t.Error("Schema should be set")
	}

	// Verify schema file exists
	if _, err := os.Stat(filepath.Join(dir, SchemaFileName)); err != nil {
		t.Errorf("schema file not created: %v", err)
	}
}

func TestService_Migrate_WithDungeon(t *testing.T) {
	dir, svc := testWorkflow(t)

	// Create a pre-existing dungeon structure
	os.MkdirAll(filepath.Join(dir, "dungeon", "completed"), 0755)
	os.MkdirAll(filepath.Join(dir, "dungeon", "custom-status"), 0755)

	result, err := svc.Migrate(context.Background(), MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Should preserve dungeon
	foundDungeon := false
	for _, p := range result.Preserved {
		if p == "dungeon/" {
			foundDungeon = true
		}
	}
	if !foundDungeon {
		t.Error("should preserve dungeon/")
	}

	// Should create schema and non-dungeon dirs
	if len(result.Created) == 0 {
		t.Error("expected created items")
	}

	// Verify active and ready were created
	for _, d := range []string{"active", "ready"} {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("directory %q should be created: %v", d, err)
		}
	}
}

func TestService_Migrate_DryRun(t *testing.T) {
	dir, svc := testWorkflow(t)

	// Create pre-existing dungeon
	os.MkdirAll(filepath.Join(dir, "dungeon", "completed"), 0755)

	result, err := svc.Migrate(context.Background(), MigrateOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Should have preserved items
	if len(result.Preserved) == 0 {
		t.Error("expected preserved items")
	}

	// Schema file should NOT be created in dry run
	if _, err := os.Stat(filepath.Join(dir, SchemaFileName)); err == nil {
		t.Error("schema should not be created during dry run")
	}
}

func TestService_Migrate_ContextCancellation(t *testing.T) {
	_, svc := testWorkflow(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Migrate(ctx, MigrateOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_MigrateV1ToV2(t *testing.T) {
	dir, svc := testWorkflow(t)

	// Initialize a v1 workflow
	svc.Init(context.Background(), InitOptions{})

	// Create items in active and ready
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
	os.MkdirAll(filepath.Join(dir, "ready", "project-2"), 0755)

	result, err := svc.MigrateV1ToV2(context.Background(), false)
	if err != nil {
		t.Fatalf("MigrateV1ToV2() error = %v", err)
	}

	// Should have moved items
	if len(result.MovedItems) != 2 {
		t.Errorf("MovedItems = %d, want 2", len(result.MovedItems))
	}

	// project-1 should be at root
	if _, err := os.Stat(filepath.Join(dir, "project-1")); err != nil {
		t.Errorf("project-1 should be at root: %v", err)
	}

	// project-2 should be in dungeon/ready
	if _, err := os.Stat(filepath.Join(dir, "dungeon", "ready", "project-2")); err != nil {
		t.Errorf("project-2 should be in dungeon/ready: %v", err)
	}

	// active and ready should be removed
	if len(result.RemovedDirs) < 2 {
		t.Errorf("expected at least 2 removed dirs, got %d", len(result.RemovedDirs))
	}

	// Schema should be updated
	if !result.SchemaUpdate {
		t.Error("SchemaUpdate should be true")
	}

	// Schema should be v2
	if svc.Schema().Version != 2 {
		t.Errorf("Schema version = %d, want 2", svc.Schema().Version)
	}
}

func TestService_MigrateV1ToV2_DryRun(t *testing.T) {
	dir, svc := testWorkflow(t)

	// Initialize a v1 workflow
	svc.Init(context.Background(), InitOptions{})

	// Create an item in active
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)

	result, err := svc.MigrateV1ToV2(context.Background(), true)
	if err != nil {
		t.Fatalf("MigrateV1ToV2() dry run error = %v", err)
	}

	// Should report moved items but not actually move
	if len(result.MovedItems) == 0 {
		t.Error("expected moved items in dry run report")
	}

	// project-1 should still be in active
	if _, err := os.Stat(filepath.Join(dir, "active", "project-1")); err != nil {
		t.Error("project-1 should still be in active during dry run")
	}

	// Schema should still be v1
	if svc.Schema().Version != CurrentSchemaVersion {
		t.Errorf("Schema should still be v1 during dry run")
	}
}

func TestService_MigrateV1ToV2_AlreadyV2(t *testing.T) {
	dir, svc := testWorkflow(t)
	_ = dir

	// Initialize a v2 workflow
	svc.Init(context.Background(), InitOptions{SchemaVersion: 2})

	_, err := svc.MigrateV1ToV2(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when already v2")
	}
}

func TestService_MigrateV1ToV2_ContextCancellation(t *testing.T) {
	_, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.MigrateV1ToV2(ctx, false)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_Crawl(t *testing.T) {
	dir, svc := testWorkflow(t)

	// Initialize workflow with items
	svc.Init(context.Background(), InitOptions{})
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
	os.MkdirAll(filepath.Join(dir, "active", "project-2"), 0755)
	os.MkdirAll(filepath.Join(dir, "ready", "project-3"), 0755)

	result, err := svc.Crawl(context.Background(), CrawlOptions{})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}

	// Should review all items
	if result.Reviewed != 3 {
		t.Errorf("Reviewed = %d, want 3", result.Reviewed)
	}
	if result.Kept != 3 {
		t.Errorf("Kept = %d, want 3", result.Kept)
	}
}

func TestService_Crawl_SpecificStatus(t *testing.T) {
	dir, svc := testWorkflow(t)

	svc.Init(context.Background(), InitOptions{})
	os.MkdirAll(filepath.Join(dir, "active", "project-1"), 0755)
	os.MkdirAll(filepath.Join(dir, "ready", "project-2"), 0755)

	result, err := svc.Crawl(context.Background(), CrawlOptions{Status: "active"})
	if err != nil {
		t.Fatalf("Crawl() error = %v", err)
	}

	// Should only review active items
	if result.Reviewed != 1 {
		t.Errorf("Reviewed = %d, want 1", result.Reviewed)
	}
}

func TestService_Crawl_ContextCancellation(t *testing.T) {
	_, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Crawl(ctx, CrawlOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestService_Init_V2(t *testing.T) {
	dir, svc := testWorkflow(t)

	result, err := svc.Init(context.Background(), InitOptions{SchemaVersion: 2})
	if err != nil {
		t.Fatalf("Init v2 error = %v", err)
	}

	if len(result.CreatedDirs) == 0 {
		t.Error("expected created dirs")
	}

	// Verify dungeon OBEY.md exists (v2 only creates dungeon OBEY.md)
	if _, err := os.Stat(filepath.Join(dir, "dungeon", "OBEY.md")); err != nil {
		t.Error("dungeon OBEY.md should exist in v2")
	}

	// Schema should be v2
	if svc.Schema().Version != 2 {
		t.Errorf("Schema version = %d, want 2", svc.Schema().Version)
	}
}

func TestService_List_StatusNotFound(t *testing.T) {
	dir, svc := testWorkflow(t)
	svc.Init(context.Background(), InitOptions{})

	// Remove a directory after init
	os.RemoveAll(filepath.Join(dir, "ready"))

	_, err := svc.List(context.Background(), "ready", ListOptions{})
	if err == nil {
		t.Fatal("expected error for missing status directory")
	}
	if !errors.Is(err, ErrStatusNotFound) {
		t.Errorf("error = %v, want ErrStatusNotFound", err)
	}
}
