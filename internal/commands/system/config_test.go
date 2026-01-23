package system

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string allowed", "", false},
		{"zero is valid", "0", false},
		{"positive number", "42", false},
		{"large positive number", "1000", false},
		{"negative number", "-1", true},
		{"negative large number", "-100", true},
		{"not a number", "abc", true},
		{"float", "3.14", true},
		{"number with spaces", " 5 ", true},
		{"mixed", "12abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePositiveInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNewConfigCommand(t *testing.T) {
	cmd := NewConfigCommand()

	if cmd.Use != "config" {
		t.Errorf("Use = %q, want %q", cmd.Use, "config")
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Check --show flag exists
	showFlag := cmd.Flags().Lookup("show")
	if showFlag == nil {
		t.Error("--show flag not found")
	}

	if showFlag.DefValue != "false" {
		t.Errorf("--show default = %q, want %q", showFlag.DefValue, "false")
	}
}

func TestShowCurrentConfig(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	os.Setenv("FEST_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("FEST_CONFIG_DIR")

	ctx := context.Background()

	// Create a config file
	cfg := config.DefaultConfig()
	cfg.Behavior.Verbose = true
	cfg.Network.Timeout = 60
	if err := config.Save(ctx, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showCurrentConfig(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("showCurrentConfig() error = %v", err)
	}

	// Check output contains expected content
	if !strings.Contains(output, "config.json") {
		t.Error("Output should contain config file path")
	}
	if !strings.Contains(output, `"verbose": true`) {
		t.Error("Output should contain verbose setting")
	}
	if !strings.Contains(output, `"timeout": 60`) {
		t.Error("Output should contain timeout setting")
	}
}

func TestShowCurrentConfigContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := showCurrentConfig(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestRunConfigTUIContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runConfigTUI(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestConfigCommandShowFlag(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	os.Setenv("FEST_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("FEST_CONFIG_DIR")

	// Create a config file
	cfg := config.DefaultConfig()
	if err := config.Save(context.Background(), cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	cmd := NewConfigCommand()
	cmd.SetArgs([]string{"--show"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("Command execute error = %v", err)
	}

	if !strings.Contains(output, "version") {
		t.Error("Output should contain version field")
	}
}

func TestConfigDirUsed(t *testing.T) {
	// Verify config is loaded from the right location
	tmpDir := t.TempDir()
	os.Setenv("FEST_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("FEST_CONFIG_DIR")

	ctx := context.Background()

	// Save a config with custom values
	cfg := config.DefaultConfig()
	cfg.Behavior.Editor = "custom-editor"
	if err := config.Save(ctx, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created in expected location")
	}

	// Load and verify
	loaded, err := config.Load(ctx, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Behavior.Editor != "custom-editor" {
		t.Errorf("Editor = %q, want %q", loaded.Behavior.Editor, "custom-editor")
	}
}

func TestValidatePositiveIntEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"max int", "9223372036854775807", false, ""},
		{"leading zeros", "007", false, ""},
		{"plus sign", "+5", false, ""}, // Go's strconv.Atoi accepts +5
		{"underscore separator", "1_000", true, ""},
		{"hex notation", "0xFF", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePositiveInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
