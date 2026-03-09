package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewDownloaderWithRef verifies that the ref/refType fields are set correctly.
func TestNewDownloaderWithRef(t *testing.T) {
	t.Run("tag ref type", func(t *testing.T) {
		d := NewDownloaderWithRef("https://github.com/org/repo", "tag", "v1.2.3", "path/to/dir")
		if d.ref != "v1.2.3" {
			t.Errorf("ref = %q, want %q", d.ref, "v1.2.3")
		}
		if d.refType != "tag" {
			t.Errorf("refType = %q, want %q", d.refType, "tag")
		}
		if d.repoURL != "https://github.com/org/repo" {
			t.Errorf("repoURL = %q", d.repoURL)
		}
		if d.repoPath != "path/to/dir" {
			t.Errorf("repoPath = %q", d.repoPath)
		}
	})

	t.Run("branch ref type", func(t *testing.T) {
		d := NewDownloaderWithRef("https://github.com/org/repo", "branch", "main", "festivals")
		if d.ref != "main" {
			t.Errorf("ref = %q, want %q", d.ref, "main")
		}
		if d.refType != "branch" {
			t.Errorf("refType = %q, want %q", d.refType, "branch")
		}
	})
}

// TestNewDownloaderBackwardCompat verifies that NewDownloader still creates a branch-type downloader.
func TestNewDownloaderBackwardCompat(t *testing.T) {
	d := NewDownloader("https://github.com/org/repo", "main", "festivals")
	if d.ref != "main" {
		t.Errorf("ref = %q, want %q", d.ref, "main")
	}
	if d.refType != "branch" {
		t.Errorf("refType = %q, want %q", d.refType, "branch")
	}
}

// TestSyncStateRoundTrip verifies that SyncState JSON marshaling/unmarshaling preserves
// the new RefType and RefName fields while remaining backward-compatible with old
// state files that lack those fields.
func TestSyncStateRoundTrip(t *testing.T) {
	t.Run("full state with ref fields", func(t *testing.T) {
		state := &SyncState{
			CommitSHA:   "abc123",
			ContentHash: "def456",
			RefType:     "tag",
			RefName:     "v1.2.3",
		}

		data, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var got SyncState
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if got.CommitSHA != state.CommitSHA {
			t.Errorf("CommitSHA = %q, want %q", got.CommitSHA, state.CommitSHA)
		}
		if got.ContentHash != state.ContentHash {
			t.Errorf("ContentHash = %q, want %q", got.ContentHash, state.ContentHash)
		}
		if got.RefType != state.RefType {
			t.Errorf("RefType = %q, want %q", got.RefType, state.RefType)
		}
		if got.RefName != state.RefName {
			t.Errorf("RefName = %q, want %q", got.RefName, state.RefName)
		}
	})

	t.Run("old state without ref fields is backward compatible", func(t *testing.T) {
		// Simulate old state file that has no ref_type or ref_name fields.
		oldJSON := `{"commit_sha":"abc123","content_hash":"def456"}`

		var got SyncState
		if err := json.Unmarshal([]byte(oldJSON), &got); err != nil {
			t.Fatalf("unmarshal old state: %v", err)
		}

		if got.CommitSHA != "abc123" {
			t.Errorf("CommitSHA = %q, want abc123", got.CommitSHA)
		}
		if got.ContentHash != "def456" {
			t.Errorf("ContentHash = %q, want def456", got.ContentHash)
		}
		// New fields should be zero-valued (empty string) — not an error.
		if got.RefType != "" {
			t.Errorf("RefType = %q, want empty", got.RefType)
		}
		if got.RefName != "" {
			t.Errorf("RefName = %q, want empty", got.RefName)
		}
	})

	t.Run("state with omitempty omits empty ref fields", func(t *testing.T) {
		state := &SyncState{
			CommitSHA:   "abc123",
			ContentHash: "def456",
		}
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		// ref_type and ref_name should NOT appear in the JSON.
		if strings.Contains(string(data), "ref_type") {
			t.Errorf("expected ref_type to be omitted, got: %s", data)
		}
		if strings.Contains(string(data), "ref_name") {
			t.Errorf("expected ref_name to be omitted, got: %s", data)
		}
	})
}

// TestReadWriteSyncStateWithRefFields verifies that ReadSyncState/WriteSyncState
// round-trip the new RefType and RefName fields correctly.
func TestReadWriteSyncStateWithRefFields(t *testing.T) {
	dir := t.TempDir()

	state := &SyncState{
		CommitSHA:   "deadbeef",
		ContentHash: "cafebabe",
		RefType:     "tag",
		RefName:     "v2.0.0",
	}

	if err := WriteSyncState(dir, state); err != nil {
		t.Fatalf("WriteSyncState: %v", err)
	}

	got, err := ReadSyncState(dir)
	if err != nil {
		t.Fatalf("ReadSyncState: %v", err)
	}
	if got == nil {
		t.Fatal("ReadSyncState returned nil")
	}

	if got.CommitSHA != state.CommitSHA {
		t.Errorf("CommitSHA = %q, want %q", got.CommitSHA, state.CommitSHA)
	}
	if got.RefType != state.RefType {
		t.Errorf("RefType = %q, want %q", got.RefType, state.RefType)
	}
	if got.RefName != state.RefName {
		t.Errorf("RefName = %q, want %q", got.RefName, state.RefName)
	}
}

// TestReadSyncStateOldFormat verifies backward compat with plain SHA files.
func TestReadSyncStateOldFormat(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, SHAMarkerFile)

	// Write plain SHA (old format)
	if err := os.WriteFile(stateFile, []byte("  oldsha123\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadSyncState(dir)
	if err != nil {
		t.Fatalf("ReadSyncState: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil state")
	}
	if got.CommitSHA != "oldsha123" {
		t.Errorf("CommitSHA = %q, want %q", got.CommitSHA, "oldsha123")
	}
	if got.RefType != "" {
		t.Errorf("RefType = %q, want empty for old format", got.RefType)
	}
}
