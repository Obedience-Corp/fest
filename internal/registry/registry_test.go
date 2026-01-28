package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	reg := New("/path/to/registry.yaml")

	if reg.Version != CurrentVersion {
		t.Errorf("Version = %q, want %q", reg.Version, CurrentVersion)
	}
	if reg.Entries == nil {
		t.Error("Entries should be initialized, got nil")
	}
	if len(reg.Entries) != 0 {
		t.Errorf("Entries length = %d, want 0", len(reg.Entries))
	}
}

func TestRegistry_Add(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	entry := RegistryEntry{
		ID:        "GU0001",
		Name:      "guild-usable",
		Status:    "active",
		Path:      "/festivals/active/guild-usable_GU0001",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := reg.Add(ctx, entry); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify entry was added
	if !reg.Exists(ctx, "GU0001") {
		t.Error("Entry should exist after Add")
	}

	// Test duplicate add
	if err := reg.Add(ctx, entry); err == nil {
		t.Error("Add() should return error for duplicate ID")
	}
}

func TestRegistry_Get(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	entry := RegistryEntry{
		ID:        "GU0001",
		Name:      "guild-usable",
		Status:    "active",
		Path:      "/festivals/active/guild-usable_GU0001",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = reg.Add(ctx, entry)

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"existing entry", "GU0001", false},
		{"non-existent entry", "XX9999", true},
		{"empty ID", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reg.Get(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.ID != tt.id {
				t.Errorf("Get() ID = %v, want %v", got.ID, tt.id)
			}
		})
	}
}

func TestRegistry_Update(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	entry := RegistryEntry{
		ID:        "GU0001",
		Name:      "guild-usable",
		Status:    "active",
		Path:      "/festivals/active/guild-usable_GU0001",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = reg.Add(ctx, entry)

	// Update status
	entry.Status = "completed"
	entry.Path = "/festivals/completed/2025-01/guild-usable_GU0001"
	if err := reg.Update(ctx, entry); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	got, _ := reg.Get(ctx, "GU0001")
	if got.Status != "completed" {
		t.Errorf("Status = %q, want %q", got.Status, "completed")
	}

	// Test update non-existent
	nonExistent := RegistryEntry{ID: "XX9999"}
	if err := reg.Update(ctx, nonExistent); err == nil {
		t.Error("Update() should return error for non-existent ID")
	}
}

func TestRegistry_Delete(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	entry := RegistryEntry{
		ID:     "GU0001",
		Name:   "guild-usable",
		Status: "active",
		Path:   "/festivals/active/guild-usable_GU0001",
	}
	_ = reg.Add(ctx, entry)

	// Delete entry
	if err := reg.Delete(ctx, "GU0001"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	if reg.Exists(ctx, "GU0001") {
		t.Error("Entry should not exist after Delete")
	}

	// Test delete non-existent
	if err := reg.Delete(ctx, "XX9999"); err == nil {
		t.Error("Delete() should return error for non-existent ID")
	}
}

func TestRegistry_Exists(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	entry := RegistryEntry{ID: "GU0001", Name: "test", Status: "active"}
	_ = reg.Add(ctx, entry)

	tests := []struct {
		id   string
		want bool
	}{
		{"GU0001", true},
		{"XX9999", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := reg.Exists(ctx, tt.id); got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	ctx := context.Background()
	reg := New("/tmp/test-registry.yaml")

	// Empty list
	list := reg.List(ctx)
	if len(list) != 0 {
		t.Errorf("List() length = %d, want 0 for empty registry", len(list))
	}

	// Add entries
	entries := []RegistryEntry{
		{ID: "GU0001", Name: "first", Status: "active"},
		{ID: "GU0002", Name: "second", Status: "active"},
		{ID: "FN0001", Name: "fest-node", Status: "active"},
	}
	for _, e := range entries {
		_ = reg.Add(ctx, e)
	}

	list = reg.List(ctx)
	if len(list) != 3 {
		t.Errorf("List() length = %d, want 3", len(list))
	}
}

func TestRegistry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reg := New("/tmp/test-registry.yaml")
	entry := RegistryEntry{ID: "GU0001", Name: "test", Status: "active"}

	// All operations should fail with cancelled context
	if err := reg.Add(ctx, entry); err == nil {
		t.Error("Add() should return error with cancelled context")
	}
	if _, err := reg.Get(ctx, "GU0001"); err == nil {
		t.Error("Get() should return error with cancelled context")
	}
	if err := reg.Update(ctx, entry); err == nil {
		t.Error("Update() should return error with cancelled context")
	}
	if err := reg.Delete(ctx, "GU0001"); err == nil {
		t.Error("Delete() should return error with cancelled context")
	}
	if reg.Exists(ctx, "GU0001") {
		t.Error("Exists() should return false with cancelled context")
	}
	if reg.Count(ctx) != 0 {
		t.Error("Count() should return 0 with cancelled context")
	}
	if reg.List(ctx) != nil {
		t.Error("List() should return nil with cancelled context")
	}
	if reg.ByStatus(ctx, "active") != nil {
		t.Error("ByStatus() should return nil with cancelled context")
	}
}

func TestLoad_NewRegistry(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "id_registry.yaml")

	// Load non-existent file should return new registry
	reg, err := Load(ctx, path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if reg == nil {
		t.Fatal("Load() returned nil registry")
	}
	if reg.Version != CurrentVersion {
		t.Errorf("Version = %q, want %q", reg.Version, CurrentVersion)
	}
}

func TestRegistry_SaveLoad(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	// Use JSONL events path instead of YAML
	eventsPath := filepath.Join(tmpDir, EventsFileName)

	// Create registry with events path
	reg := New(eventsPath)
	entry := RegistryEntry{
		ID:        "GU0001",
		Name:      "guild-usable",
		Status:    "active",
		Path:      "/festivals/active/guild-usable_GU0001",
		CreatedAt: time.Now().Truncate(time.Second),
		UpdatedAt: time.Now().Truncate(time.Second),
	}

	// Use AddWithEvent to add entry and write event
	if err := reg.AddWithEvent(ctx, entry); err != nil {
		t.Fatalf("AddWithEvent() error = %v", err)
	}

	// Verify events file exists
	if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
		t.Fatal("Events file not created")
	}

	// Load and verify
	loaded, err := Load(ctx, eventsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, err := loaded.Get(ctx, "GU0001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != entry.Name {
		t.Errorf("Name = %q, want %q", got.Name, entry.Name)
	}
	if got.Status != entry.Status {
		t.Errorf("Status = %q, want %q", got.Status, entry.Status)
	}
}

func TestRegistry_AtomicSave(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "id_registry.yaml")

	reg := New(path)
	_ = reg.Add(ctx, RegistryEntry{ID: "GU0001", Name: "test", Status: "active"})

	// Save should use atomic write
	if err := reg.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify no temp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after successful save")
	}
}

func TestLoad_CorruptedFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "id_registry.yaml")

	// Write corrupted YAML
	if err := os.WriteFile(path, []byte("not: valid: yaml: {{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(ctx, path)
	if err == nil {
		t.Error("Load() should return error for corrupted file")
	}
}

func TestLoad_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Load(ctx, "/any/path")
	if err == nil {
		t.Error("Load() should return error with cancelled context")
	}
}

// JSONL Migration and Event Tests

func TestLoad_MigratesLegacyYAML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	festivalDir := filepath.Join(tmpDir, ".festival")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create legacy YAML registry
	yamlPath := filepath.Join(festivalDir, LegacyRegistryFileName)
	yamlContent := `version: "1.0"
entries:
  GU0001:
    id: GU0001
    name: test-festival
    status: active
    path: /festivals/active/test-festival-GU0001
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load should trigger migration
	reg, err := Load(ctx, yamlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify entry was loaded
	entry, err := reg.Get(ctx, "GU0001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if entry.Name != "test-festival" {
		t.Errorf("Name = %q, want %q", entry.Name, "test-festival")
	}

	// Verify YAML file was deleted
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Error("Legacy YAML file should be deleted after migration")
	}

	// Verify JSONL file was created
	eventsPath := filepath.Join(festivalDir, EventsFileName)
	if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
		t.Error("Events file should exist after migration")
	}

	// Verify events file content
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("Events file should not be empty")
	}
}

func TestLoad_PrefersJSONLOverYAML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	festivalDir := filepath.Join(tmpDir, ".festival")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create both YAML and JSONL files
	yamlPath := filepath.Join(festivalDir, LegacyRegistryFileName)
	yamlContent := `version: "1.0"
entries:
  YAML0001:
    id: YAML0001
    name: yaml-festival
    status: active
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	eventsPath := filepath.Join(festivalDir, EventsFileName)
	eventsContent := `{"ts":"2026-01-27T00:00:00Z","event":"create","id":"JSONL0001","name":"jsonl-festival","status":"planned"}
`
	if err := os.WriteFile(eventsPath, []byte(eventsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load should prefer JSONL
	reg, err := Load(ctx, yamlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have JSONL entry, not YAML entry
	if reg.Exists(ctx, "YAML0001") {
		t.Error("Should not have YAML entry when JSONL exists")
	}
	if !reg.Exists(ctx, "JSONL0001") {
		t.Error("Should have JSONL entry")
	}
}

func TestAddWithEvent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, EventsFileName)

	reg := New(eventsPath)
	entry := RegistryEntry{
		ID:     "EV0001",
		Name:   "event-test",
		Status: "planned",
	}

	if err := reg.AddWithEvent(ctx, entry); err != nil {
		t.Fatalf("AddWithEvent() error = %v", err)
	}

	// Verify events file was created with event
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"event":"create"`) {
		t.Error("Events file should contain create event")
	}
	if !strings.Contains(string(data), `"id":"EV0001"`) {
		t.Error("Events file should contain festival ID")
	}
}

func TestUpdateStatusWithEvent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, EventsFileName)

	reg := New(eventsPath)
	entry := RegistryEntry{
		ID:     "MV0001",
		Name:   "move-test",
		Status: "planned",
	}

	// Add initial entry
	if err := reg.AddWithEvent(ctx, entry); err != nil {
		t.Fatal(err)
	}

	// Update status
	if err := reg.UpdateStatusWithEvent(ctx, "MV0001", "planned", "active"); err != nil {
		t.Fatalf("UpdateStatusWithEvent() error = %v", err)
	}

	// Verify move event was written
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"event":"move"`) {
		t.Error("Events file should contain move event")
	}
	if !strings.Contains(string(data), `"from":"planned"`) {
		t.Error("Events file should contain from status")
	}
	if !strings.Contains(string(data), `"to":"active"`) {
		t.Error("Events file should contain to status")
	}

	// Verify in-memory state was updated
	got, _ := reg.Get(ctx, "MV0001")
	if got.Status != "active" {
		t.Errorf("Status = %q, want %q", got.Status, "active")
	}
}

func TestDeleteWithEvent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, EventsFileName)

	reg := New(eventsPath)
	entry := RegistryEntry{
		ID:     "DEL0001",
		Name:   "delete-test",
		Status: "dungeon",
	}

	// Add initial entry
	if err := reg.AddWithEvent(ctx, entry); err != nil {
		t.Fatal(err)
	}

	// Delete entry
	if err := reg.DeleteWithEvent(ctx, "DEL0001"); err != nil {
		t.Fatalf("DeleteWithEvent() error = %v", err)
	}

	// Verify delete event was written
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"event":"delete"`) {
		t.Error("Events file should contain delete event")
	}

	// Verify entry was removed from registry
	if reg.Exists(ctx, "DEL0001") {
		t.Error("Entry should be removed from registry")
	}
}

func TestLoadFromEventsPreservesState(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, EventsFileName)

	// Write events that simulate festival lifecycle
	eventsContent := `{"ts":"2026-01-01T00:00:00Z","event":"create","id":"LC0001","name":"lifecycle-test","status":"planned"}
{"ts":"2026-01-02T00:00:00Z","event":"move","id":"LC0001","from":"planned","to":"active"}
{"ts":"2026-01-03T00:00:00Z","event":"create","id":"LC0002","name":"second-festival","status":"planned"}
{"ts":"2026-01-04T00:00:00Z","event":"delete","id":"LC0002"}
`
	if err := os.WriteFile(eventsPath, []byte(eventsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load and verify materialized state
	reg, err := Load(ctx, eventsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// LC0001 should exist with active status (was moved from planned)
	entry, err := reg.Get(ctx, "LC0001")
	if err != nil {
		t.Fatalf("Get(LC0001) error = %v", err)
	}
	if entry.Status != "active" {
		t.Errorf("LC0001 Status = %q, want %q", entry.Status, "active")
	}

	// LC0002 should not exist (was deleted)
	if reg.Exists(ctx, "LC0002") {
		t.Error("LC0002 should not exist (was deleted)")
	}

	// Registry should have exactly 1 entry
	if reg.Count(ctx) != 1 {
		t.Errorf("Count = %d, want 1", reg.Count(ctx))
	}
}

// Benchmarks for performance-critical operations

// setupBenchmarkRegistry creates a registry with n entries for benchmarking
func setupBenchmarkRegistry(n int) *Registry {
	ctx := context.Background()
	reg := New("/tmp/benchmark-registry.yaml")

	for i := 0; i < n; i++ {
		entry := RegistryEntry{
			ID:     generateTestID(i),
			Name:   "test-festival",
			Status: StatusActive,
			Path:   "/festivals/active/test",
		}
		_ = reg.Add(ctx, entry)
	}

	return reg
}

// generateTestID creates a test ID like TE0001, TE0002, etc.
func generateTestID(n int) string {
	return fmt.Sprintf("TE%04d", n+1)
}

func BenchmarkRegistry_Get(b *testing.B) {
	reg := setupBenchmarkRegistry(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = reg.Get(ctx, "TE0500")
	}
}

func BenchmarkRegistry_Add(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := New("/tmp/benchmark-registry.yaml")
		_ = reg.Add(ctx, RegistryEntry{
			ID:     generateTestID(i),
			Name:   "test",
			Status: StatusActive,
		})
	}
}

func BenchmarkRegistry_Exists(b *testing.B) {
	reg := setupBenchmarkRegistry(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.Exists(ctx, "TE0500")
	}
}

func BenchmarkRegistry_List(b *testing.B) {
	reg := setupBenchmarkRegistry(100)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.List(ctx)
	}
}

func BenchmarkRegistry_ByStatus(b *testing.B) {
	reg := setupBenchmarkRegistry(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.ByStatus(ctx, StatusActive)
	}
}
