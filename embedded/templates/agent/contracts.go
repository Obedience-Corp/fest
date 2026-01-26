package agent

import (
	"strings"
)

// TemplateContract defines the structural requirements for a template.
// Contracts are used for testing to verify templates meet their requirements
// without being fragile to content changes.
type TemplateContract struct {
	// Name is the template name (e.g., "implementation/instructions")
	Name string

	// RequiredFields are data fields that must be present in the data passed to the template.
	// These represent the template's interface contract.
	RequiredFields []string

	// ForbiddenTokens are strings that should never appear in rendered output.
	// These indicate template rendering problems.
	ForbiddenTokens []string

	// ExpectedSections are markdown headers or structural elements expected in output.
	// Uses prefix matching - "##" matches any level-2 header.
	ExpectedSections []string

	// MinimumLength is the minimum expected output length in bytes.
	// Zero means no minimum.
	MinimumLength int
}

// DefaultForbiddenTokens are tokens that indicate template rendering problems.
var DefaultForbiddenTokens = []string{
	"{{.",        // Unrendered template variable
	"{{",         // Any unrendered template syntax
	"<no value>", // Go template's default for missing values
}

// Contracts defines structural contracts for each template.
// These contracts verify templates work correctly without testing specific content.
var Contracts = map[string]TemplateContract{
	// Implementation mode templates
	"implementation/instructions": {
		Name: "implementation/instructions",
		RequiredFields: []string{
			"Header",
			"ActionInstruction",
			"TasksSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections:  []string{"## ", "ACTION REQUIRED"},
		ForbiddenTokens:   DefaultForbiddenTokens,
		MinimumLength:     100,
	},
	"implementation/complete": {
		Name:             "implementation/complete",
		RequiredFields:   []string{"Header", "Message"},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    20,
	},
	"implementation/dry_run": {
		Name: "implementation/dry_run",
		RequiredFields: []string{
			"Header",
			"FestivalLine",
			"PathLine",
			"SummarySection",
			"PhasesSection",
		},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    50,
	},

	// Plan mode templates
	"plan/instructions": {
		Name: "plan/instructions",
		RequiredFields: []string{
			"Header",
			"ObjectivesSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections: []string{"## ", "Planning Mode", "Guidelines"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    100,
	},

	// Research mode templates
	"research/instructions": {
		Name: "research/instructions",
		RequiredFields: []string{
			"Header",
			"TopicSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections: []string{"## ", "Research Mode", "Guidelines"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    100,
	},

	// Review mode templates
	"review/instructions": {
		Name: "review/instructions",
		RequiredFields: []string{
			"Header",
			"ChecklistSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections: []string{"## ", "Review Mode", "Guidelines"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    100,
	},

	// Action mode templates
	"action/instructions": {
		Name: "action/instructions",
		RequiredFields: []string{
			"Header",
			"ActionSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections: []string{"## ", "Action Mode", "Guidelines"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    100,
	},

	// Ingest mode templates
	"ingest/instructions": {
		Name: "ingest/instructions",
		RequiredFields: []string{
			"Header",
			"InputSection",
			"ProgressLine",
			"PositionSection",
			"ContextSection",
			"ProgressCmd",
		},
		ExpectedSections: []string{"## ", "Ingest Mode", "Guidelines"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    100,
	},

	// Next task templates
	"next/task": {
		Name: "next/task",
		RequiredFields: []string{
			"Header",
			"SequenceLine",
			"PhaseLine",
			"RecommendationLine",
			"ProgressCmd",
			"ContextSection",
		},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    50,
	},
	"next/blocked": {
		Name: "next/blocked",
		RequiredFields: []string{
			"Header",
			"PhaseLine",
			"TypeLine",
			"DescriptionLine",
		},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    30,
	},
	"next/complete": {
		Name:             "next/complete",
		RequiredFields:   []string{"Header", "Message"},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    20,
	},
	"next/no_task": {
		Name:             "next/no_task",
		RequiredFields:   []string{"Header"},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    10,
	},

	// Validation templates
	"validate/markers": {
		Name:             "validate/markers",
		RequiredFields:   []string{"Header"},
		ExpectedSections: []string{},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    10,
	},
	"validate/reflection": {
		Name:             "validate/reflection",
		RequiredFields:   []string{"Header", "Criteria"},
		ExpectedSections: []string{"verify"},
		ForbiddenTokens:  DefaultForbiddenTokens,
		MinimumLength:    30,
	},
}

// Validate checks if rendered output satisfies the contract.
func (c *TemplateContract) Validate(output string) []string {
	var violations []string

	// Check minimum length
	if c.MinimumLength > 0 && len(output) < c.MinimumLength {
		violations = append(violations, "output too short")
	}

	// Check for forbidden tokens
	for _, token := range c.ForbiddenTokens {
		if strings.Contains(output, token) {
			violations = append(violations, "contains forbidden token: "+token)
		}
	}

	// Check for expected sections
	for _, section := range c.ExpectedSections {
		if !strings.Contains(output, section) {
			violations = append(violations, "missing expected section: "+section)
		}
	}

	return violations
}

// GetContract returns the contract for a template, or nil if not defined.
func GetContract(templateName string) *TemplateContract {
	if c, ok := Contracts[templateName]; ok {
		return &c
	}
	return nil
}

// AllContractNames returns all template names that have contracts defined.
func AllContractNames() []string {
	names := make([]string, 0, len(Contracts))
	for name := range Contracts {
		names = append(names, name)
	}
	return names
}

// ContractCoverage returns the percentage of registered templates that have contracts.
func ContractCoverage() float64 {
	registered := List()
	if len(registered) == 0 {
		return 0
	}

	covered := 0
	for _, name := range registered {
		if _, ok := Contracts[name]; ok {
			covered++
		}
	}

	return float64(covered) / float64(len(registered)) * 100
}
