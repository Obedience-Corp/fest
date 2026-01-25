package template

import (
	"testing"
)

func TestMarkerCategoryString(t *testing.T) {
	tests := []struct {
		category MarkerCategory
		expected string
	}{
		{CategoryAutoFill, "auto-fill"},
		{CategoryConfig, "config"},
		{CategoryManual, "manual"},
		{MarkerCategory(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("MarkerCategory.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDefaultMarkerDefinitions(t *testing.T) {
	defs := DefaultMarkerDefinitions()

	if len(defs) == 0 {
		t.Error("DefaultMarkerDefinitions() returned empty slice")
	}

	// Count by category
	autoFillCount := 0
	configCount := 0
	for _, def := range defs {
		switch def.Category {
		case CategoryAutoFill:
			autoFillCount++
		case CategoryConfig:
			configCount++
		}
	}

	// Should have multiple auto-fill markers
	if autoFillCount < 10 {
		t.Errorf("Expected at least 10 auto-fill markers, got %d", autoFillCount)
	}

	// Should have multiple config markers
	if configCount < 5 {
		t.Errorf("Expected at least 5 config markers, got %d", configCount)
	}
}

func TestMarkerDefinitionFields(t *testing.T) {
	defs := DefaultMarkerDefinitions()

	for _, def := range defs {
		// All markers must have a pattern
		if def.Pattern == "" {
			t.Error("Found marker definition with empty pattern")
		}

		// All markers must have a description
		if def.Description == "" {
			t.Errorf("Marker %q has empty description", def.Pattern)
		}

		// Auto-fill markers must have ContextField
		if def.Category == CategoryAutoFill && def.ContextField == "" {
			t.Errorf("Auto-fill marker %q has empty ContextField", def.Pattern)
		}

		// Config markers must have ConfigKey
		if def.Category == CategoryConfig && def.ConfigKey == "" {
			t.Errorf("Config marker %q has empty ConfigKey", def.Pattern)
		}
	}
}

func TestMarkerRegistryCreation(t *testing.T) {
	registry := NewMarkerRegistry()

	if registry == nil {
		t.Fatal("NewMarkerRegistry() returned nil")
	}

	if registry.Count() == 0 {
		t.Error("NewMarkerRegistry() created empty registry")
	}

	// Should match DefaultMarkerDefinitions count
	expected := len(DefaultMarkerDefinitions())
	if registry.Count() != expected {
		t.Errorf("Registry count = %d, want %d", registry.Count(), expected)
	}
}

func TestMarkerRegistryLookup(t *testing.T) {
	registry := NewMarkerRegistry()

	// Test existing marker
	def, ok := registry.Lookup("[REPLACE: Festival Name]")
	if !ok {
		t.Error("Lookup failed for known marker")
	}
	if def.Category != CategoryAutoFill {
		t.Errorf("Expected CategoryAutoFill, got %v", def.Category)
	}
	if def.ContextField != "FestivalName" {
		t.Errorf("Expected ContextField 'FestivalName', got %q", def.ContextField)
	}

	// Test non-existent marker
	_, ok = registry.Lookup("[REPLACE: Unknown Marker]")
	if ok {
		t.Error("Lookup should return false for unknown marker")
	}
}

func TestMarkerRegistryIsAutoFill(t *testing.T) {
	registry := NewMarkerRegistry()

	// Auto-fill marker
	if !registry.IsAutoFill("[REPLACE: Festival Name]") {
		t.Error("IsAutoFill should return true for Festival Name")
	}

	// Config marker
	if registry.IsAutoFill("[REPLACE: Run your project's lint command]") {
		t.Error("IsAutoFill should return false for config marker")
	}

	// Unknown marker
	if registry.IsAutoFill("[REPLACE: Unknown]") {
		t.Error("IsAutoFill should return false for unknown marker")
	}
}

func TestMarkerRegistryIsConfig(t *testing.T) {
	registry := NewMarkerRegistry()

	// Config marker
	if !registry.IsConfig("[REPLACE: Run your project's lint command]") {
		t.Error("IsConfig should return true for lint command")
	}

	// Auto-fill marker
	if registry.IsConfig("[REPLACE: Festival Name]") {
		t.Error("IsConfig should return false for auto-fill marker")
	}

	// Unknown marker
	if registry.IsConfig("[REPLACE: Unknown]") {
		t.Error("IsConfig should return false for unknown marker")
	}
}

func TestMarkerRegistryGetByCategory(t *testing.T) {
	registry := NewMarkerRegistry()

	autoFill := registry.GetByCategory(CategoryAutoFill)
	if len(autoFill) == 0 {
		t.Error("GetByCategory(CategoryAutoFill) returned empty slice")
	}

	config := registry.GetByCategory(CategoryConfig)
	if len(config) == 0 {
		t.Error("GetByCategory(CategoryConfig) returned empty slice")
	}

	// Manual category should be empty (not registered)
	manual := registry.GetByCategory(CategoryManual)
	if len(manual) != 0 {
		t.Errorf("GetByCategory(CategoryManual) should be empty, got %d", len(manual))
	}
}

func TestMarkerRegistryGetAutoFillMarkers(t *testing.T) {
	registry := NewMarkerRegistry()

	markers := registry.GetAutoFillMarkers()

	// Verify all returned markers are auto-fill
	for _, def := range markers {
		if def.Category != CategoryAutoFill {
			t.Errorf("GetAutoFillMarkers returned non-auto-fill marker: %q", def.Pattern)
		}
	}
}

func TestMarkerRegistryGetConfigMarkers(t *testing.T) {
	registry := NewMarkerRegistry()

	markers := registry.GetConfigMarkers()

	// Verify all returned markers are config
	for _, def := range markers {
		if def.Category != CategoryConfig {
			t.Errorf("GetConfigMarkers returned non-config marker: %q", def.Pattern)
		}
	}
}

func TestMarkerRegistryAll(t *testing.T) {
	registry := NewMarkerRegistry()

	all := registry.All()

	if len(all) != registry.Count() {
		t.Errorf("All() returned %d markers, Count() returned %d", len(all), registry.Count())
	}
}

func TestNoDuplicatePatterns(t *testing.T) {
	defs := DefaultMarkerDefinitions()
	seen := make(map[string]bool)

	for _, def := range defs {
		if seen[def.Pattern] {
			t.Errorf("Duplicate marker pattern: %q", def.Pattern)
		}
		seen[def.Pattern] = true
	}
}

func TestSpecificMarkersExist(t *testing.T) {
	registry := NewMarkerRegistry()

	// Category A markers from spec
	categoryAMarkers := []string{
		"[REPLACE: Festival Name]",
		"[REPLACE: FESTIVAL_ID]",
		"[REPLACE: Phase name]",
		"[REPLACE: Sequence name]",
		"[REPLACE: Task name]",
	}

	for _, marker := range categoryAMarkers {
		def, ok := registry.Lookup(marker)
		if !ok {
			t.Errorf("Expected Category A marker %q not found", marker)
			continue
		}
		if def.Category != CategoryAutoFill {
			t.Errorf("Marker %q should be CategoryAutoFill, got %v", marker, def.Category)
		}
	}

	// Category B markers from spec
	categoryBMarkers := []string{
		"[REPLACE: Run your project's lint command]",
		"[REPLACE: Run your project's test command]",
		"[REPLACE: Run your project's integration test command]",
		"[REPLACE: coverage threshold, e.g., 80%]",
		"[REPLACE: Run your project's coverage command]",
	}

	for _, marker := range categoryBMarkers {
		def, ok := registry.Lookup(marker)
		if !ok {
			t.Errorf("Expected Category B marker %q not found", marker)
			continue
		}
		if def.Category != CategoryConfig {
			t.Errorf("Marker %q should be CategoryConfig, got %v", marker, def.Category)
		}
	}
}
