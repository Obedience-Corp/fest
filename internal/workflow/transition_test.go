package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTransition(t *testing.T) {
	tmpDir := t.TempDir()

	// Create status directories with an item
	os.MkdirAll(filepath.Join(tmpDir, "inbox", "my-item"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "active"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "dungeon", "completed", "old-item"), 0755)

	ctx := context.Background()

	tests := []struct {
		name    string
		item    string
		dest    string
		from    string
		wantErr bool
	}{
		{
			name: "normal transition",
			item: "my-item",
			dest: "active",
			from: "inbox",
		},
		{
			name:    "same status",
			item:    "my-item",
			dest:    "inbox",
			wantErr: true,
		},
		{
			name: "nested status",
			item: "old-item",
			dest: "inbox",
			from: "dungeon/completed",
		},
		{
			name:    "missing item",
			item:    "nonexistent",
			dest:    "active",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := DetectTransition(ctx, tmpDir, tt.item, tt.dest)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.From != tt.from {
				t.Errorf("From = %q, want %q", tr.From, tt.from)
			}
			if tr.To != tt.dest {
				t.Errorf("To = %q, want %q", tr.To, tt.dest)
			}
			if tr.Item != tt.item {
				t.Errorf("Item = %q, want %q", tr.Item, tt.item)
			}
		})
	}
}

func TestDetectTransitionCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DetectTransition(ctx, "/fake", "item", "dest")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestTransitionCommitMessage(t *testing.T) {
	tr := Transition{Item: "my-project", From: "inbox", To: "active"}
	want := "flow: move my-project from inbox to active"
	if got := tr.CommitMessage(); got != want {
		t.Errorf("CommitMessage() = %q, want %q", got, want)
	}
}

func TestAutoCommitConfigShouldAutoCommit(t *testing.T) {
	tests := []struct {
		name   string
		config AutoCommitConfig
		from   string
		to     string
		want   bool
	}{
		{
			name:   "globally enabled",
			config: AutoCommitConfig{Enabled: true},
			from:   "inbox",
			to:     "active",
			want:   true,
		},
		{
			name:   "globally disabled",
			config: AutoCommitConfig{Enabled: false},
			from:   "inbox",
			to:     "active",
			want:   false,
		},
		{
			name: "override enables",
			config: AutoCommitConfig{
				Enabled: false,
				Transitions: []TransitionRule{
					{From: "active", To: "completed", Commit: true},
				},
			},
			from: "active",
			to:   "completed",
			want: true,
		},
		{
			name: "override disables",
			config: AutoCommitConfig{
				Enabled: true,
				Transitions: []TransitionRule{
					{From: "inbox", To: "active", Commit: false},
				},
			},
			from: "inbox",
			to:   "active",
			want: false,
		},
		{
			name: "no matching override falls to global",
			config: AutoCommitConfig{
				Enabled: true,
				Transitions: []TransitionRule{
					{From: "active", To: "completed", Commit: false},
				},
			},
			from: "inbox",
			to:   "active",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldAutoCommit(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("ShouldAutoCommit(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
