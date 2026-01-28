package agent

import (
	"testing"
)

// TestAllTemplatesHaveContracts verifies contract coverage.
func TestAllTemplatesHaveContracts(t *testing.T) {
	registered := List()
	missing := []string{}

	for _, name := range registered {
		if GetContract(name) == nil {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Logf("Templates without contracts: %v", missing)
	}

	// Require at least 80% coverage
	coverage := ContractCoverage()
	if coverage < 80 {
		t.Errorf("Contract coverage %.1f%% is below 80%% threshold", coverage)
	}
}

// TestContractValidation verifies templates satisfy their contracts.
func TestContractValidation(t *testing.T) {
	for name, contract := range Contracts {
		t.Run(name, func(t *testing.T) {
			data := minimalTestData(name)
			output, err := Render(name, data)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			violations := contract.Validate(output)
			for _, v := range violations {
				t.Errorf("Contract violation: %s", v)
			}
		})
	}
}

// TestContractRequiredFields verifies contracts define required fields.
func TestContractRequiredFields(t *testing.T) {
	for name, contract := range Contracts {
		t.Run(name, func(t *testing.T) {
			// All instruction templates should have at least Header and ProgressCmd
			if isInstructionTemplate(name) {
				hasHeader := false
				hasProgressCmd := false
				for _, field := range contract.RequiredFields {
					if field == "Header" {
						hasHeader = true
					}
					if field == "ProgressCmd" {
						hasProgressCmd = true
					}
				}
				if !hasHeader {
					t.Error("Instruction template should require Header field")
				}
				if !hasProgressCmd {
					t.Error("Instruction template should require ProgressCmd field")
				}
			}
		})
	}
}

// TestForbiddenTokensDetection verifies the forbidden token check works.
func TestForbiddenTokensDetection(t *testing.T) {
	contract := TemplateContract{
		ForbiddenTokens: []string{"{{.", "<no value>"},
	}

	tests := []struct {
		name    string
		output  string
		hasViol bool
	}{
		{"clean output", "This is clean output", false},
		{"has template var", "Value: {{.Missing}}", true},
		{"has no value", "Value: <no value>", true},
		{"has both", "{{.X}} and <no value>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := contract.Validate(tt.output)
			hasViolation := len(violations) > 0
			if hasViolation != tt.hasViol {
				t.Errorf("Validate(%q) = %v violations, want hasViolation=%v",
					tt.output, violations, tt.hasViol)
			}
		})
	}
}

// TestExpectedSectionsDetection verifies section checking works.
func TestExpectedSectionsDetection(t *testing.T) {
	contract := TemplateContract{
		ExpectedSections: []string{"## Header", "Guidelines"},
	}

	tests := []struct {
		name    string
		output  string
		hasViol bool
	}{
		{"has both", "## Header\nGuidelines here", false},
		{"missing header", "Guidelines here", true},
		{"missing guidelines", "## Header\nOther content", true},
		{"missing both", "Random content", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := contract.Validate(tt.output)
			hasViolation := len(violations) > 0
			if hasViolation != tt.hasViol {
				t.Errorf("Validate(%q) = %v violations, want hasViolation=%v",
					tt.output, violations, tt.hasViol)
			}
		})
	}
}

// TestMinimumLengthCheck verifies length checking works.
func TestMinimumLengthCheck(t *testing.T) {
	contract := TemplateContract{
		MinimumLength: 50,
	}

	tests := []struct {
		name    string
		output  string
		hasViol bool
	}{
		{"too short", "Short", true},
		{"exactly minimum", string(make([]byte, 50)), false},
		{"above minimum", string(make([]byte, 100)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := contract.Validate(tt.output)
			hasViolation := len(violations) > 0
			if hasViolation != tt.hasViol {
				t.Errorf("Validate() hasViolation=%v, want %v", hasViolation, tt.hasViol)
			}
		})
	}
}

// TestGetContractReturnsNilForUnknown verifies nil return for unknown templates.
func TestGetContractReturnsNilForUnknown(t *testing.T) {
	contract := GetContract("nonexistent/template")
	if contract != nil {
		t.Error("GetContract should return nil for unknown template")
	}
}

// TestAllContractNames returns all contract names.
func TestAllContractNames(t *testing.T) {
	names := AllContractNames()
	if len(names) == 0 {
		t.Error("AllContractNames should return non-empty list")
	}

	// Verify all returned names exist in Contracts
	for _, name := range names {
		if _, ok := Contracts[name]; !ok {
			t.Errorf("AllContractNames returned %q which is not in Contracts", name)
		}
	}
}

// isInstructionTemplate returns true if the template is an instructions template.
func isInstructionTemplate(name string) bool {
	return len(name) > 13 && name[len(name)-12:] == "instructions"
}
