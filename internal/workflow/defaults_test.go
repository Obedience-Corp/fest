package workflow

import (
	"errors"
	"testing"
)

func TestDefaultSchema(t *testing.T) {
	schema := DefaultSchema()

	t.Run("has correct version", func(t *testing.T) {
		if schema.Version != CurrentSchemaVersion {
			t.Errorf("Version = %d, want %d", schema.Version, CurrentSchemaVersion)
		}
	})

	t.Run("has correct type", func(t *testing.T) {
		if schema.Type != SchemaType {
			t.Errorf("Type = %q, want %q", schema.Type, SchemaType)
		}
	})

	t.Run("has required directories", func(t *testing.T) {
		required := []string{"active", "ready", "dungeon"}
		for _, name := range required {
			if _, ok := schema.Directories[name]; !ok {
				t.Errorf("missing required directory: %s", name)
			}
		}
	})

	t.Run("dungeon is nested", func(t *testing.T) {
		dungeon := schema.Directories["dungeon"]
		if !dungeon.Nested {
			t.Error("dungeon should be nested")
		}
		if len(dungeon.Children) == 0 {
			t.Error("dungeon should have children")
		}
	})

	t.Run("has default status", func(t *testing.T) {
		if schema.DefaultStatus == "" {
			t.Error("DefaultStatus should not be empty")
		}
		if _, ok := schema.Directories[schema.DefaultStatus]; !ok {
			t.Errorf("DefaultStatus %q is not a valid directory", schema.DefaultStatus)
		}
	})

	t.Run("has history tracking", func(t *testing.T) {
		if !schema.TrackHistory {
			t.Error("TrackHistory should be true")
		}
		if schema.HistoryFile == "" {
			t.Error("HistoryFile should not be empty")
		}
	})

	t.Run("validates successfully", func(t *testing.T) {
		if err := schema.Validate(); err != nil {
			t.Errorf("DefaultSchema should validate: %v", err)
		}
	})

	t.Run("has proper transition opts", func(t *testing.T) {
		active := schema.Directories["active"]
		if len(active.TransitionOpts) == 0 {
			t.Error("active should have transition_opts defined")
		}

		// Should be able to transition to ready and dungeon
		found := make(map[string]bool)
		for _, opt := range active.TransitionOpts {
			found[opt] = true
		}
		if !found["ready"] {
			t.Error("active should allow transition to ready")
		}
		if !found["dungeon"] {
			t.Error("active should allow transition to dungeon")
		}
	})

	t.Run("directories have descriptions", func(t *testing.T) {
		for name, dir := range schema.Directories {
			if dir.Description == "" {
				t.Errorf("directory %q should have a description", name)
			}
		}
	})
}

func TestDefaultSchemaV2(t *testing.T) {
	schema := DefaultSchemaV2()

	t.Run("has version 2", func(t *testing.T) {
		if schema.Version != 2 {
			t.Errorf("Version = %d, want 2", schema.Version)
		}
	})

	t.Run("has correct type", func(t *testing.T) {
		if schema.Type != SchemaType {
			t.Errorf("Type = %q, want %q", schema.Type, SchemaType)
		}
	})

	t.Run("default status is root", func(t *testing.T) {
		if schema.DefaultStatus != "." {
			t.Errorf("DefaultStatus = %q, want %q", schema.DefaultStatus, ".")
		}
	})

	t.Run("has root directory", func(t *testing.T) {
		root, ok := schema.Directories["."]
		if !ok {
			t.Fatal("missing root directory '.'")
		}
		if root.Description == "" {
			t.Error("root directory should have a description")
		}
	})

	t.Run("has dungeon with children", func(t *testing.T) {
		dungeon, ok := schema.Directories["dungeon"]
		if !ok {
			t.Fatal("missing dungeon directory")
		}
		if !dungeon.Nested {
			t.Error("dungeon should be nested")
		}

		expectedChildren := []string{"ready", "completed", "archived", "someday"}
		for _, name := range expectedChildren {
			if _, ok := dungeon.Children[name]; !ok {
				t.Errorf("missing dungeon child: %s", name)
			}
		}
	})

	t.Run("no extra top-level directories", func(t *testing.T) {
		for name := range schema.Directories {
			if name != "." && name != "dungeon" {
				t.Errorf("unexpected top-level directory: %q", name)
			}
		}
	})

	t.Run("has history tracking", func(t *testing.T) {
		if !schema.TrackHistory {
			t.Error("TrackHistory should be true")
		}
	})

	t.Run("validates successfully", func(t *testing.T) {
		if err := schema.Validate(); err != nil {
			t.Errorf("DefaultSchemaV2 should validate: %v", err)
		}
	})

	t.Run("root transitions to dungeon", func(t *testing.T) {
		root := schema.Directories["."]
		if len(root.TransitionOpts) == 0 {
			t.Error("root should have transition opts")
		}
		found := false
		for _, opt := range root.TransitionOpts {
			if opt == "dungeon" {
				found = true
			}
		}
		if !found {
			t.Error("root should allow transition to dungeon")
		}
	})
}

func TestFestivalSchema(t *testing.T) {
	schema := FestivalSchema()

	t.Run("has correct version", func(t *testing.T) {
		if schema.Version != CurrentSchemaVersion {
			t.Errorf("Version = %d, want %d", schema.Version, CurrentSchemaVersion)
		}
	})

	t.Run("has correct type", func(t *testing.T) {
		if schema.Type != SchemaType {
			t.Errorf("Type = %q, want %q", schema.Type, SchemaType)
		}
	})

	t.Run("has festival lifecycle directories", func(t *testing.T) {
		required := []string{"planned", "ready", "active", "dungeon"}
		for _, name := range required {
			if _, ok := schema.Directories[name]; !ok {
				t.Errorf("missing required directory: %s", name)
			}
		}
	})

	t.Run("default status is planned", func(t *testing.T) {
		if schema.DefaultStatus != "planned" {
			t.Errorf("DefaultStatus = %q, want %q", schema.DefaultStatus, "planned")
		}
	})

	t.Run("planned transitions to ready active and dungeon", func(t *testing.T) {
		planned := schema.Directories["planned"]
		found := make(map[string]bool)
		for _, opt := range planned.TransitionOpts {
			found[opt] = true
		}
		for _, expected := range []string{"ready", "active", "dungeon"} {
			if !found[expected] {
				t.Errorf("planned should allow transition to %s", expected)
			}
		}
	})

	t.Run("ready transitions to active and dungeon", func(t *testing.T) {
		ready := schema.Directories["ready"]
		found := make(map[string]bool)
		for _, opt := range ready.TransitionOpts {
			found[opt] = true
		}
		if !found["active"] {
			t.Error("ready should allow transition to active")
		}
		if !found["dungeon"] {
			t.Error("ready should allow transition to dungeon")
		}
	})

	t.Run("active transitions to dungeon only", func(t *testing.T) {
		active := schema.Directories["active"]
		if len(active.TransitionOpts) != 1 || active.TransitionOpts[0] != "dungeon" {
			t.Errorf("active transition_opts = %v, want [dungeon]", active.TransitionOpts)
		}
	})

	t.Run("dungeon is nested with children", func(t *testing.T) {
		dungeon := schema.Directories["dungeon"]
		if !dungeon.Nested {
			t.Error("dungeon should be nested")
		}
		expectedChildren := []string{"completed", "archived", "someday"}
		for _, child := range expectedChildren {
			if _, ok := dungeon.Children[child]; !ok {
				t.Errorf("missing dungeon child: %s", child)
			}
		}
	})

	t.Run("has correct ordering", func(t *testing.T) {
		expected := map[string]int{
			"planned": 1,
			"ready":   2,
			"active":  3,
			"dungeon": 4,
		}
		for name, wantOrder := range expected {
			dir := schema.Directories[name]
			if dir.Order != wantOrder {
				t.Errorf("directory %q order = %d, want %d", name, dir.Order, wantOrder)
			}
		}
	})

	t.Run("has history tracking", func(t *testing.T) {
		if !schema.TrackHistory {
			t.Error("TrackHistory should be true")
		}
		if schema.HistoryFile == "" {
			t.Error("HistoryFile should not be empty")
		}
	})

	t.Run("validates successfully", func(t *testing.T) {
		if err := schema.Validate(); err != nil {
			t.Errorf("FestivalSchema should validate: %v", err)
		}
	})

	t.Run("directories have descriptions", func(t *testing.T) {
		for name, dir := range schema.Directories {
			if dir.Description == "" {
				t.Errorf("directory %q should have a description", name)
			}
		}
	})

	t.Run("no unexpected directories", func(t *testing.T) {
		allowed := map[string]bool{"planned": true, "ready": true, "active": true, "dungeon": true}
		for name := range schema.Directories {
			if !allowed[name] {
				t.Errorf("unexpected directory: %q", name)
			}
		}
	})
}

func TestSchemaV2_ValidationRules(t *testing.T) {
	tests := []struct {
		name    string
		schema  Schema
		wantErr bool
	}{
		{
			name: "valid v2 schema",
			schema: Schema{
				Version:       2,
				DefaultStatus: ".",
				Directories: map[string]Directory{
					".": {Description: "Active"},
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
			name: "v2 missing root directory",
			schema: Schema{
				Version:       2,
				DefaultStatus: ".",
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
			wantErr: true,
		},
		{
			name: "v2 wrong default status",
			schema: Schema{
				Version:       2,
				DefaultStatus: "active",
				Directories: map[string]Directory{
					"active": {Description: "Active"},
					".":      {Description: "Root"},
				},
			},
			wantErr: true,
		},
		{
			name: "v2 non-root non-nested directory",
			schema: Schema{
				Version:       2,
				DefaultStatus: ".",
				Directories: map[string]Directory{
					".":      {Description: "Root"},
					"active": {Description: "Active"},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported version",
			schema: Schema{
				Version: 99,
				Directories: map[string]Directory{
					"active": {Description: "Active"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidSchema) {
				t.Errorf("expected ErrInvalidSchema, got: %v", err)
			}
		})
	}
}
