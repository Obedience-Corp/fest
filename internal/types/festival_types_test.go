package types

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
)

func TestLoadFestivalTypesConfig_ValidFile(t *testing.T) {
	t.Run("loads valid config from file", func(t *testing.T) {
		ctx := context.Background()

		// Create temp directory with valid config
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "festival_types.yaml")
		validYAML := `version: 1
types:
  - name: test
    description: Test type
    default: true
    phases:
      - name: DO
        type: simple
        auto: true
`
		if err := os.WriteFile(configPath, []byte(validYAML), 0644); err != nil {
			t.Fatal(err)
		}

		config, err := loadConfigFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if config == nil {
			t.Fatal("expected config, got nil")
		}
		if len(config.Types) != 1 {
			t.Errorf("expected 1 type, got %d", len(config.Types))
		}
		if config.Types[0].Name != "test" {
			t.Errorf("expected name 'test', got %q", config.Types[0].Name)
		}
	})
}

func TestLoadFestivalTypesConfig_MissingFile(t *testing.T) {
	t.Run("returns default config when file missing", func(t *testing.T) {
		ctx := context.Background()
		nonExistentPath := "/nonexistent/festival_types.yaml"

		_, err := loadConfigFromFile(ctx, nonExistentPath)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !errors.Is(err, errors.ErrCodeNotFound) {
			t.Errorf("expected NOT_FOUND error, got %v", err)
		}
	})
}

func TestValidateConfig_DuplicateNames(t *testing.T) {
	t.Run("rejects duplicate type names", func(t *testing.T) {
		config := &FestivalTypesConfig{
			Version: 1,
			Types: []FestivalType{
				{Name: "test", Description: "First", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
				{Name: "test", Description: "Duplicate", Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			},
		}

		err := validateConfig(config)
		if err == nil {
			t.Fatal("expected validation error for duplicate names")
		}
		if !errors.Is(err, errors.ErrCodeValidation) {
			t.Errorf("expected VALIDATION error, got %v", err)
		}
	})
}

func TestValidateConfig_RequiresPhases(t *testing.T) {
	t.Run("requires at least one phase except for ritual", func(t *testing.T) {
		tests := []struct {
			name      string
			typeName  string
			phases    []PhaseSpec
			wantError bool
		}{
			{
				name:      "standard with no phases fails",
				typeName:  "standard",
				phases:    []PhaseSpec{},
				wantError: true,
			},
			{
				name:      "ritual with no phases succeeds",
				typeName:  "ritual",
				phases:    []PhaseSpec{},
				wantError: false,
			},
			{
				name:     "standard with phases succeeds",
				typeName: "standard",
				phases: []PhaseSpec{
					{Name: "INGEST", Type: "standard", Auto: true},
				},
				wantError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &FestivalTypesConfig{
					Version: 1,
					Types: []FestivalType{
						{Name: tt.typeName, Description: "Test", Default: true, Phases: tt.phases},
					},
				}

				err := validateConfig(config)
				if tt.wantError && err == nil {
					t.Error("expected validation error, got nil")
				}
				if !tt.wantError && err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			})
		}
	})
}

func TestValidateConfig_ExactlyOneDefault(t *testing.T) {
	tests := []struct {
		name         string
		types        []FestivalType
		wantError    bool
		errorMessage string
	}{
		{
			name: "one default succeeds",
			types: []FestivalType{
				{Name: "standard", Description: "Default", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
				{Name: "other", Description: "Not default", Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			},
			wantError: false,
		},
		{
			name: "no defaults fails",
			types: []FestivalType{
				{Name: "standard", Description: "No default", Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			},
			wantError:    true,
			errorMessage: "exactly one default",
		},
		{
			name: "multiple defaults fails",
			types: []FestivalType{
				{Name: "standard", Description: "Default 1", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
				{Name: "other", Description: "Default 2", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			},
			wantError:    true,
			errorMessage: "exactly one default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FestivalTypesConfig{Version: 1, Types: tt.types}
			err := validateConfig(config)

			if tt.wantError && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateConfig_RequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		festType  FestivalType
		wantError bool
	}{
		{
			name: "missing name fails",
			festType: FestivalType{
				Name:        "",
				Description: "Has description",
				Default:     true,
				Phases:      []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}},
			},
			wantError: true,
		},
		{
			name: "missing description fails",
			festType: FestivalType{
				Name:    "test",
				Default: true,
				Phases:  []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}},
			},
			wantError: true,
		},
		{
			name: "all required fields succeeds",
			festType: FestivalType{
				Name:        "test",
				Description: "Test type",
				Default:     true,
				Phases:      []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FestivalTypesConfig{Version: 1, Types: []FestivalType{tt.festType}}
			err := validateConfig(config)

			if tt.wantError && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestGetAutoPhases(t *testing.T) {
	festType := FestivalType{
		Name: "test",
		Phases: []PhaseSpec{
			{Name: "AUTO1", Auto: true},
			{Name: "MANUAL1", Auto: false},
			{Name: "AUTO2", Auto: true},
			{Name: "MANUAL2", Auto: false},
		},
	}

	autoPhases := festType.GetAutoPhases()
	if len(autoPhases) != 2 {
		t.Errorf("expected 2 auto phases, got %d", len(autoPhases))
	}
	if autoPhases[0].Name != "AUTO1" || autoPhases[1].Name != "AUTO2" {
		t.Error("unexpected auto phase names")
	}
}

func TestGetPendingPhases(t *testing.T) {
	festType := FestivalType{
		Name: "test",
		Phases: []PhaseSpec{
			{Name: "AUTO1", Auto: true},
			{Name: "MANUAL1", Auto: false},
			{Name: "AUTO2", Auto: true},
			{Name: "MANUAL2", Auto: false},
		},
	}

	pendingPhases := festType.GetPendingPhases()
	if len(pendingPhases) != 2 {
		t.Errorf("expected 2 pending phases, got %d", len(pendingPhases))
	}
	if pendingPhases[0].Name != "MANUAL1" || pendingPhases[1].Name != "MANUAL2" {
		t.Error("unexpected pending phase names")
	}
}

func TestGetFestivalType(t *testing.T) {
	config := &FestivalTypesConfig{
		Version: 1,
		Types: []FestivalType{
			{Name: "standard", Description: "Standard type", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			{Name: "custom", Description: "Custom type", Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
		},
	}

	t.Run("finds existing type", func(t *testing.T) {
		festType, err := config.GetFestivalType("custom")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if festType.Name != "custom" {
			t.Errorf("expected name 'custom', got %q", festType.Name)
		}
	})

	t.Run("returns error for unknown type", func(t *testing.T) {
		_, err := config.GetFestivalType("nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown type")
		}
		if !errors.Is(err, errors.ErrCodeNotFound) {
			t.Errorf("expected NOT_FOUND error, got %v", err)
		}
	})
}

func TestGetDefaultType(t *testing.T) {
	config := &FestivalTypesConfig{
		Version: 1,
		Types: []FestivalType{
			{Name: "other", Description: "Not default", Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
			{Name: "standard", Description: "Default type", Default: true, Phases: []PhaseSpec{{Name: "DO", Type: "simple", Auto: true}}},
		},
	}

	defaultType := config.GetDefaultType()
	if defaultType == nil {
		t.Fatal("expected default type, got nil")
	}
	if defaultType.Name != "standard" {
		t.Errorf("expected default type 'standard', got %q", defaultType.Name)
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()
	if config == nil {
		t.Fatal("expected default config, got nil")
	}
	if len(config.Types) == 0 {
		t.Fatal("expected at least one type in default config")
	}

	// Validate default config structure
	if err := validateConfig(config); err != nil {
		t.Errorf("default config failed validation: %v", err)
	}

	// Ensure standard type exists and is default
	standardType, err := config.GetFestivalType("standard")
	if err != nil {
		t.Fatalf("default config missing 'standard' type: %v", err)
	}
	if !standardType.Default {
		t.Error("standard type should be default")
	}
}

func TestLoadFestivalTypesConfig_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := LoadFestivalTypesConfig(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestLoadConfigFromFile_InvalidYAML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
version: 1
types:
  - name: test
    invalid yaml here [[[
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfigFromFile(ctx, configPath)
	if err == nil {
		t.Fatal("expected parse error for invalid YAML")
	}
	if !errors.Is(err, errors.ErrCodeParse) {
		t.Errorf("expected PARSE error, got %v", err)
	}
}
