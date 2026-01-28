package guidance

import (
	"testing"
)

func TestModeFromPhaseType(t *testing.T) {
	tests := []struct {
		name      string
		phaseType string
		want      Mode
	}{
		{"implementation maps to execute", "implementation", ModeImplementation},
		{"planning maps to plan", "planning", ModePlan},
		{"research maps to research", "research", ModeResearch},
		{"review maps to review", "review", ModeReview},
		{"deployment maps to action", "deployment", ModeAction},
		{"action maps to action", "action", ModeAction},
		{"non_coding_action maps to action", "non_coding_action", ModeAction},
		{"ingest maps to ingest", "ingest", ModeIngest},
		{"unknown defaults to execute", "unknown", ModeImplementation},
		{"empty defaults to execute", "", ModeImplementation},
		{"random string defaults to execute", "foobar", ModeImplementation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ModeFromPhaseType(tt.phaseType)
			if got != tt.want {
				t.Errorf("ModeFromPhaseType(%q) = %v, want %v", tt.phaseType, got, tt.want)
			}
		})
	}
}

func TestPhaseTypeFromMode(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{"execute maps to implementation", ModeImplementation, "implementation"},
		{"plan maps to planning", ModePlan, "planning"},
		{"research maps to research", ModeResearch, "research"},
		{"review maps to review", ModeReview, "review"},
		{"action maps to deployment", ModeAction, "deployment"},
		{"ingest maps to ingest", ModeIngest, "ingest"},
		{"invalid mode defaults to implementation", Mode("invalid"), "implementation"},
		{"empty mode defaults to implementation", Mode(""), "implementation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PhaseTypeFromMode(tt.mode)
			if got != tt.want {
				t.Errorf("PhaseTypeFromMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestModeIsValid(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want bool
	}{
		{"execute is valid", ModeImplementation, true},
		{"plan is valid", ModePlan, true},
		{"ingest is valid", ModeIngest, true},
		{"research is valid", ModeResearch, true},
		{"review is valid", ModeReview, true},
		{"action is valid", ModeAction, true},
		{"invalid mode is not valid", Mode("invalid"), false},
		{"empty mode is not valid", Mode(""), false},
		{"random string is not valid", Mode("foobar"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.want {
				t.Errorf("Mode(%q).IsValid() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeImplementation, "implementation"},
		{ModePlan, "plan"},
		{ModeIngest, "ingest"},
		{ModeResearch, "research"},
		{ModeReview, "review"},
		{ModeAction, "action"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutonomyLevelString(t *testing.T) {
	tests := []struct {
		level AutonomyLevel
		want  string
	}{
		{AutonomyHigh, "high"},
		{AutonomyMedium, "medium"},
		{AutonomyLow, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("AutonomyLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllModes(t *testing.T) {
	modes := AllModes()

	if len(modes) != 6 {
		t.Errorf("AllModes() returned %d modes, want 6", len(modes))
	}

	// Verify all expected modes are present
	expected := map[Mode]bool{
		ModeImplementation: false,
		ModePlan:           false,
		ModeIngest:         false,
		ModeResearch:       false,
		ModeReview:         false,
		ModeAction:         false,
	}

	for _, m := range modes {
		if _, ok := expected[m]; !ok {
			t.Errorf("AllModes() contains unexpected mode: %v", m)
		}
		expected[m] = true
	}

	for mode, found := range expected {
		if !found {
			t.Errorf("AllModes() missing expected mode: %v", mode)
		}
	}
}

func TestModeRoundTrip(t *testing.T) {
	// Test that going from mode -> phase type -> mode gives consistent results
	for _, mode := range AllModes() {
		phaseType := PhaseTypeFromMode(mode)
		roundTripped := ModeFromPhaseType(phaseType)
		if roundTripped != mode {
			t.Errorf("Round trip failed: %v -> %q -> %v", mode, phaseType, roundTripped)
		}
	}
}
