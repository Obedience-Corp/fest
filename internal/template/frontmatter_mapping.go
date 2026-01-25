// Package template provides frontmatter-to-marker mapping functionality.
package template

import (
	"fmt"
	"strconv"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// FrontmatterToReplacements generates a marker replacement map from parsed frontmatter.
// The frontmatter values are mapped to corresponding body markers based on the document type.
// Only non-empty values are included in the returned map.
func FrontmatterToReplacements(fm *frontmatter.Frontmatter) map[string]string {
	if fm == nil {
		return make(map[string]string)
	}

	replacements := make(map[string]string)

	// Map fest_name based on document type
	if fm.Name != "" {
		switch fm.Type {
		case frontmatter.TypeFestival:
			replacements["[REPLACE: Festival Name]"] = fm.Name
		case frontmatter.TypePhase:
			replacements["[REPLACE: Phase name]"] = fm.Name
			replacements["[REPLACE: Phase Name]"] = fm.Name
			replacements["[REPLACE: PHASE_NAME]"] = fm.Name
		case frontmatter.TypeSequence:
			replacements["[REPLACE: Sequence name]"] = fm.Name
			replacements["[REPLACE: Sequence Name]"] = fm.Name
			replacements["[REPLACE: SEQUENCE_NAME]"] = fm.Name
		case frontmatter.TypeTask, frontmatter.TypeGate:
			replacements["[REPLACE: Task name]"] = fm.Name
			replacements["[REPLACE: Task Name]"] = fm.Name
			replacements["[REPLACE: TASK_NAME]"] = fm.Name
		}
	}

	// Map fest_id based on document type
	if fm.ID != "" {
		switch fm.Type {
		case frontmatter.TypeFestival:
			replacements["[REPLACE: FESTIVAL_ID]"] = fm.ID
		case frontmatter.TypePhase:
			replacements["[REPLACE: Phase ID]"] = fm.ID
		case frontmatter.TypeSequence:
			replacements["[REPLACE: Sequence ID]"] = fm.ID
		case frontmatter.TypeTask, frontmatter.TypeGate:
			replacements["[REPLACE: Task ID]"] = fm.ID
			// NN_task_name is ID without .md extension
			if len(fm.ID) > 3 && fm.ID[len(fm.ID)-3:] == ".md" {
				replacements["[REPLACE: NN_task_name]"] = fm.ID[:len(fm.ID)-3]
			} else {
				replacements["[REPLACE: NN_task_name]"] = fm.ID
			}
		}
	}

	// Map fest_parent based on document type
	if fm.Parent != "" {
		switch fm.Type {
		case frontmatter.TypePhase:
			// Phase parent is festival (not typically a marker)
		case frontmatter.TypeSequence:
			replacements["[REPLACE: Parent phase ID]"] = fm.Parent
		case frontmatter.TypeTask, frontmatter.TypeGate:
			replacements["[REPLACE: Parent sequence ID]"] = fm.Parent
		}
	}

	// Map fest_order based on document type
	if fm.Order > 0 {
		orderStr := strconv.Itoa(fm.Order)
		switch fm.Type {
		case frontmatter.TypePhase:
			replacements["[REPLACE: Phase order]"] = orderStr
		case frontmatter.TypeSequence:
			replacements["[REPLACE: Sequence order]"] = orderStr
		case frontmatter.TypeTask, frontmatter.TypeGate:
			replacements["[REPLACE: Task order]"] = orderStr
			// Also map to NN pattern (two-digit format)
			replacements["[REPLACE: NN]"] = fmt.Sprintf("%02d", fm.Order)
		}
	}

	// Map fest_type
	if fm.Type != "" {
		replacements["[REPLACE: Document type]"] = string(fm.Type)
	}

	// Map fest_phase_type
	if fm.PhaseType != "" {
		replacements["[REPLACE: Phase type]"] = string(fm.PhaseType)
	}

	// Map fest_status
	if fm.Status != "" {
		replacements["[REPLACE: Status]"] = string(fm.Status)
	}

	// Map fest_autonomy
	if fm.Autonomy != "" {
		replacements["[REPLACE: Autonomy]"] = string(fm.Autonomy)
	}

	// Map fest_created
	if !fm.Created.IsZero() {
		replacements["[REPLACE: Created date]"] = fm.Created.Format("2006-01-02T15:04:05-07:00")
	}

	// Map fest_gate_type for gate documents
	if fm.GateType != "" {
		replacements["[REPLACE: Gate type]"] = string(fm.GateType)
	}

	// Map fest_priority
	if fm.Priority != "" {
		replacements["[REPLACE: Priority]"] = string(fm.Priority)
	}

	return replacements
}

// MergeFrontmatterIntoContext merges frontmatter values into a Context struct.
// This populates the Context with values from frontmatter based on document type.
func MergeFrontmatterIntoContext(ctx *Context, fm *frontmatter.Frontmatter) {
	if fm == nil || ctx == nil {
		return
	}

	// Track parent IDs from frontmatter (applied after ComputeStructureVariables)
	var parentPhaseID, parentSequenceID string

	// Set values based on document type
	switch fm.Type {
	case frontmatter.TypeFestival:
		ctx.SetFestival(fm.Name, "", nil) // Goal and tags not in frontmatter
		ctx.SetFestivalID(fm.ID)

	case frontmatter.TypePhase:
		ctx.SetPhase(fm.Order, fm.Name, string(fm.PhaseType))
		// Parent of phase is the festival name, but we don't track that in context
		// The phase ID is computed from order + name

	case frontmatter.TypeSequence:
		ctx.SetSequence(fm.Order, fm.Name)
		// Parent phase ID from frontmatter
		if fm.Parent != "" {
			parentPhaseID = fm.Parent
		}

	case frontmatter.TypeTask, frontmatter.TypeGate:
		ctx.SetTask(fm.Order, fm.Name)
		// Parent sequence ID from frontmatter
		if fm.Parent != "" {
			parentSequenceID = fm.Parent
		}
	}

	// Set created date
	if !fm.Created.IsZero() {
		ctx.SetCreatedDate(fm.Created)
	}

	// Compute structure variables after setting values
	ctx.ComputeStructureVariables()

	// Override parent IDs with frontmatter values (ComputeStructureVariables may have set different values)
	if parentPhaseID != "" {
		ctx.ParentPhaseID = parentPhaseID
	}
	if parentSequenceID != "" {
		ctx.ParentSequenceID = parentSequenceID
	}
}

// ContextFromFrontmatter creates a new Context populated from frontmatter values.
// This is a convenience function that combines NewContext() and MergeFrontmatterIntoContext().
func ContextFromFrontmatter(fm *frontmatter.Frontmatter) *Context {
	ctx := NewContext()
	MergeFrontmatterIntoContext(ctx, fm)
	return ctx
}
