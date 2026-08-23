package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveAutoCommit(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *AgentConfig
		noCommitFlag bool
		wantCommit   bool
		wantRejected bool
	}{
		{
			name:         "nil config, no flag: commits",
			cfg:          nil,
			noCommitFlag: false,
			wantCommit:   true,
			wantRejected: false,
		},
		{
			name:         "nil config, --no-commit: skips",
			cfg:          nil,
			noCommitFlag: true,
			wantCommit:   false,
			wantRejected: false,
		},
		{
			name:         "default config, --no-commit: skips (backward compat)",
			cfg:          &AgentConfig{},
			noCommitFlag: true,
			wantCommit:   false,
			wantRejected: false,
		},
		{
			name:         "default config, no flag: commits",
			cfg:          &AgentConfig{},
			noCommitFlag: false,
			wantCommit:   true,
			wantRejected: false,
		},
		{
			name:         "require_auto_commit, no flag: commits",
			cfg:          &AgentConfig{RequireAutoCommit: true},
			noCommitFlag: false,
			wantCommit:   true,
			wantRejected: false,
		},
		{
			name:         "require_auto_commit, --no-commit: rejected, still commits",
			cfg:          &AgentConfig{RequireAutoCommit: true},
			noCommitFlag: true,
			wantCommit:   true,
			wantRejected: true,
		},
		{
			name:         "require_auto_commit false, --no-commit: skips",
			cfg:          &AgentConfig{RequireAutoCommit: false},
			noCommitFlag: true,
			wantCommit:   false,
			wantRejected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCommit, gotRejected := EffectiveAutoCommit(tt.cfg, tt.noCommitFlag)
			if gotCommit != tt.wantCommit {
				t.Errorf("shouldCommit = %v, want %v", gotCommit, tt.wantCommit)
			}
			if gotRejected != tt.wantRejected {
				t.Errorf("rejected = %v, want %v", gotRejected, tt.wantRejected)
			}
		})
	}
}

func TestMergeAgentConfig_RequireAutoCommit(t *testing.T) {
	t.Run("workspace true, festival false: merged true", func(t *testing.T) {
		merged := MergeAgentConfig(
			&AgentConfig{RequireAutoCommit: true},
			&AgentConfig{RequireAutoCommit: false},
		)
		if !merged.RequireAutoCommit {
			t.Error("expected RequireAutoCommit=true when workspace sets it true")
		}
	})

	t.Run("workspace false, festival true: merged true", func(t *testing.T) {
		merged := MergeAgentConfig(
			&AgentConfig{RequireAutoCommit: false},
			&AgentConfig{RequireAutoCommit: true},
		)
		if !merged.RequireAutoCommit {
			t.Error("expected RequireAutoCommit=true when festival sets it true")
		}
	})

	t.Run("both false: merged false", func(t *testing.T) {
		merged := MergeAgentConfig(
			&AgentConfig{RequireAutoCommit: false},
			&AgentConfig{RequireAutoCommit: false},
		)
		if merged.RequireAutoCommit {
			t.Error("expected RequireAutoCommit=false when both are false")
		}
	})

	t.Run("both nil: merged false", func(t *testing.T) {
		merged := MergeAgentConfig(nil, nil)
		if merged.RequireAutoCommit {
			t.Error("expected RequireAutoCommit=false when both configs are nil")
		}
	})
}

func TestLoadEffectiveAgentConfig_RequireAutoCommitFromWorkspace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := `version: "1.0"
agent:
  require_auto_commit: true
`
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := LoadEffectiveAgentConfig(root, "")
	if !cfg.RequireAutoCommit {
		t.Fatal("expected RequireAutoCommit=true from workspace config")
	}
}

func TestLoadEffectiveAgentConfig_RequireAutoCommitFromFestival(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Workspace config without require_auto_commit
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte("version: \"1.0\"\n"), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	// Festival config with require_auto_commit
	festDir := filepath.Join(root, "ready", "test-festival")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir festival: %v", err)
	}
	festYAML := `version: "1.0"
agent:
  require_auto_commit: true
`
	if err := os.WriteFile(filepath.Join(festDir, FestivalConfigFileName), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write festival config: %v", err)
	}

	cfg := LoadEffectiveAgentConfig(root, festDir)
	if !cfg.RequireAutoCommit {
		t.Fatal("expected RequireAutoCommit=true from festival config overriding workspace")
	}
}
