package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/workflow"
)

func TestRunRepair(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, root string)
		expectRenames map[string]string
		expectMoves   map[string]string
		expectCreate  []string
		expectSchema  bool
		expectError   bool
	}{
		{
			name: "clean_directory_no_issues",
			setupFunc: func(t *testing.T, root string) {
				// Create proper structure with schema
				createProperStructure(t, root)
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{},
			expectSchema:  false,
			expectError:   false,
		},
		{
			name: "old_planned_directory",
			setupFunc: func(t *testing.T, root string) {
				// Create old "planned/" directory
				mustMkdir(t, filepath.Join(root, "planned"))
			},
			expectRenames: map[string]string{"planned": "planning"},
			expectMoves:   map[string]string{},
			expectCreate:  []string{"ready", "active", "dungeon/completed", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "old_completed_directory",
			setupFunc: func(t *testing.T, root string) {
				// Create old top-level "completed/" directory
				mustMkdir(t, filepath.Join(root, "completed"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{"completed": "dungeon/completed"},
			expectCreate:  []string{"planning", "ready", "active", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "both_old_issues",
			setupFunc: func(t *testing.T, root string) {
				// Create both old patterns
				mustMkdir(t, filepath.Join(root, "planned"))
				mustMkdir(t, filepath.Join(root, "completed"))
			},
			expectRenames: map[string]string{"planned": "planning"},
			expectMoves:   map[string]string{"completed": "dungeon/completed"},
			expectCreate:  []string{"ready", "active", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "missing_directories",
			setupFunc: func(t *testing.T, root string) {
				// Create only planning/ but missing others
				mustMkdir(t, filepath.Join(root, "planning"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{"ready", "active", "dungeon/completed", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "missing_schema_only",
			setupFunc: func(t *testing.T, root string) {
				// Create all directories but no schema
				mustMkdir(t, filepath.Join(root, "planning"))
				mustMkdir(t, filepath.Join(root, "ready"))
				mustMkdir(t, filepath.Join(root, "active"))
				mustMkdir(t, filepath.Join(root, "dungeon", "completed"))
				mustMkdir(t, filepath.Join(root, "dungeon", "archived"))
				mustMkdir(t, filepath.Join(root, "dungeon", "someday"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{},
			expectSchema:  true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()
			festivalsRoot := filepath.Join(tmpDir, "festivals")
			mustMkdir(t, festivalsRoot)

			// Setup test structure
			if tt.setupFunc != nil {
				tt.setupFunc(t, festivalsRoot)
			}

			// Change to festivals root for the test
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(oldWd)
			if err := os.Chdir(festivalsRoot); err != nil {
				t.Fatal(err)
			}

			// Analyze repair
			ctx := context.Background()
			plan, err := analyzeRepair(ctx, festivalsRoot)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify renames
			if len(plan.renameDirs) != len(tt.expectRenames) {
				t.Errorf("expected %d renames, got %d", len(tt.expectRenames), len(plan.renameDirs))
			}
			for old, new := range tt.expectRenames {
				if plan.renameDirs[old] != new {
					t.Errorf("expected rename %s → %s, got %s", old, new, plan.renameDirs[old])
				}
			}

			// Verify moves
			if len(plan.moveDirs) != len(tt.expectMoves) {
				t.Errorf("expected %d moves, got %d", len(tt.expectMoves), len(plan.moveDirs))
			}
			for src, dst := range tt.expectMoves {
				if plan.moveDirs[src] != dst {
					t.Errorf("expected move %s → %s, got %s", src, dst, plan.moveDirs[src])
				}
			}

			// Verify creates (order-independent check)
			if len(plan.createDirs) != len(tt.expectCreate) {
				t.Errorf("expected %d creates, got %d: %v", len(tt.expectCreate), len(plan.createDirs), plan.createDirs)
			} else {
				createMap := make(map[string]bool)
				for _, dir := range plan.createDirs {
					createMap[dir] = true
				}
				for _, expected := range tt.expectCreate {
					if !createMap[expected] {
						t.Errorf("expected to create %s but it wasn't in plan", expected)
					}
				}
			}

			// Verify schema creation
			if plan.createSchema != tt.expectSchema {
				t.Errorf("expected createSchema=%v, got %v", tt.expectSchema, plan.createSchema)
			}

			// Execute repair if there are changes
			if plan.hasIssues() {
				if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
					t.Fatalf("executeRepair failed: %v", err)
				}

				// Verify all renames were applied
				for _, new := range tt.expectRenames {
					newPath := filepath.Join(festivalsRoot, new)
					if _, err := os.Stat(newPath); os.IsNotExist(err) {
						t.Errorf("rename target %s does not exist after repair", new)
					}
				}

				// Verify all moves were applied
				for _, dst := range tt.expectMoves {
					dstPath := filepath.Join(festivalsRoot, dst)
					if _, err := os.Stat(dstPath); os.IsNotExist(err) {
						t.Errorf("move destination %s does not exist after repair", dst)
					}
				}

				// Verify all creates were applied
				for _, dir := range tt.expectCreate {
					dirPath := filepath.Join(festivalsRoot, dir)
					if _, err := os.Stat(dirPath); os.IsNotExist(err) {
						t.Errorf("directory %s was not created", dir)
					}
				}

				// Verify schema was created if expected
				if tt.expectSchema {
					schemaPath := filepath.Join(festivalsRoot, workflow.SchemaFileName)
					if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
						t.Error("schema file was not created")
					}
				}
			}
		})
	}
}

func TestRunRepair_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	mustMkdir(t, festivalsRoot)

	// Create old structure
	mustMkdir(t, filepath.Join(festivalsRoot, "planned"))

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Analyze should fail with context error
	_, err := analyzeRepair(ctx, festivalsRoot)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRunRepair_ExecuteContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	mustMkdir(t, festivalsRoot)

	// Create repair plan
	plan := &repairPlan{
		festivalsRoot: festivalsRoot,
		createDirs:    []string{"planning"},
	}

	// Create context with timeout that will expire
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	<-ctx.Done()

	// Execute should fail with context error
	err := executeRepair(ctx, festivalsRoot, plan)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

// Helper functions

func createProperStructure(t *testing.T, root string) {
	t.Helper()

	// Create FestivalSchema structure
	schema := workflow.FestivalSchema()
	for _, dir := range schema.AllDirectories() {
		mustMkdir(t, filepath.Join(root, dir))
	}

	// Create schema file
	data := []byte(`version: 1
type: status-workflow
name: Festival Workflow
description: Status workflow for festival lifecycle management
default_status: planning
track_history: true
history_file: .workflow-history.jsonl
directories:
  planning:
    description: Festivals being designed and documented
    order: 1
  ready:
    description: Planned and ready for execution
    order: 2
  active:
    description: Festivals currently being executed
    order: 3
  dungeon:
    description: Terminal festival states
    order: 4
    nested: true
    children:
      completed:
        description: Successfully finished festivals
        order: 1
      archived:
        description: Preserved but no longer active
        order: 2
      someday:
        description: Backlog — nice-to-have work
        order: 3
`)
	schemaPath := filepath.Join(root, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}
