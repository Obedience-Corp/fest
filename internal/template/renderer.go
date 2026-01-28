package template

import (
	"bytes"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// Renderer renders templates with variable substitution
type Renderer interface {
	Render(tmpl *Template, ctx *Context) (string, error)
	RenderString(content string, ctx *Context) (string, error)
	RenderWithMarkerReplacement(content string, ctx *Context, config map[string]string) (string, error)
}

type rendererImpl struct {
	// Template cache for performance
	cache map[string]*template.Template
}

// NewRenderer creates a new template renderer
func NewRenderer() Renderer {
	return &rendererImpl{
		cache: make(map[string]*template.Template),
	}
}

// Render renders a template with the given context
func (r *rendererImpl) Render(tmpl *Template, ctx *Context) (string, error) {
	return r.RenderString(tmpl.Content, ctx)
}

// RenderString renders a template string with the given context
func (r *rendererImpl) RenderString(content string, ctx *Context) (string, error) {
	// Create Go template with Sprig functions
	tmpl, err := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Parse(content)
	if err != nil {
		return "", errors.Parse("parsing template", err)
	}

	// Convert context to template data for execution
	vars := ctx.ToTemplateData()

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", errors.Wrap(err, "executing template").
			WithCode(errors.ErrCodeTemplate)
	}

	return buf.String(), nil
}

// RenderWithMarkerReplacement renders a template and then replaces [REPLACE: ...] markers
// with values from the context and config. This combines Go template rendering with
// marker-based auto-fill for a complete rendering pipeline.
//
// The replacement order is:
// 1. Go template variables ({{.variable}})
// 2. Context-based markers from ctx.ToReplacementMap()
// 3. Config-based markers from ctx.ToReplacementMapWithConfig(config)
// 4. Frontmatter-based markers (if content has frontmatter)
//
// If ctx is nil, content is returned as-is (backward compatibility).
func (r *rendererImpl) RenderWithMarkerReplacement(content string, ctx *Context, config map[string]string) (string, error) {
	// If no context, return as-is (backward compatibility)
	if ctx == nil {
		return content, nil
	}

	// First, render Go templates
	rendered, err := r.RenderString(content, ctx)
	if err != nil {
		return "", err
	}

	// Build replacement map from context (includes config markers)
	replacements := ctx.ToReplacementMapWithConfig(config)

	// Try to extract frontmatter for additional context
	fm, _, parseErr := frontmatter.Parse([]byte(rendered))
	if parseErr == nil && fm != nil {
		// Add frontmatter-based replacements (context takes precedence)
		fmReplacements := FrontmatterToReplacements(fm)
		for k, v := range fmReplacements {
			if _, exists := replacements[k]; !exists {
				replacements[k] = v
			}
		}
	}

	// Perform marker replacements
	result := rendered
	for marker, value := range replacements {
		if value != "" {
			result = strings.ReplaceAll(result, marker, value)
		}
	}

	return result, nil
}

// ReplaceMarkers replaces [REPLACE: ...] markers in content using the provided replacement map.
// This is a lower-level function for when you have a pre-built replacement map.
func ReplaceMarkers(content string, replacements map[string]string) string {
	result := content
	for marker, value := range replacements {
		if value != "" {
			result = strings.ReplaceAll(result, marker, value)
		}
	}
	return result
}

// CountMarkers counts the number of [REPLACE: ...] markers in content.
// Useful for debugging and reporting unfilled markers.
func CountMarkers(content string) int {
	re := regexp.MustCompile(`\[REPLACE:[^\]]+\]`)
	return len(re.FindAllString(content, -1))
}

// FindUnfilledMarkers returns all [REPLACE: ...] markers that remain in content.
func FindUnfilledMarkers(content string) []string {
	re := regexp.MustCompile(`\[REPLACE:[^\]]+\]`)
	matches := re.FindAllString(content, -1)

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// ValidateTemplate checks if a template can be rendered with required variables
func ValidateTemplate(tmpl *Template, ctx *Context) error {
	if tmpl.Metadata == nil {
		// No metadata, can't validate
		return nil
	}

	// Check required variables
	missing := []string{}
	for _, required := range tmpl.Metadata.RequiredVariables {
		if _, ok := ctx.Get(required); !ok {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return errors.Validation("missing required variables: " + strings.Join(missing, ", ")).
			WithOp("ValidateTemplate")
	}

	return nil
}

// CheckUnrenderedVariables scans rendered content for unrendered variables
func CheckUnrenderedVariables(content string) []string {
	unrendered := []string{}

	// Find all {{variable}} patterns
	start := 0
	for {
		idx := strings.Index(content[start:], "{{")
		if idx == -1 {
			break
		}

		// Find closing }}
		idx += start
		closeIdx := strings.Index(content[idx:], "}}")
		if closeIdx == -1 {
			break
		}

		// Extract variable name
		variable := content[idx+2 : idx+closeIdx]
		variable = strings.TrimSpace(variable)

		// Skip if it's a comment or conditional
		if !strings.HasPrefix(variable, "#") &&
			!strings.HasPrefix(variable, "/") &&
			!strings.HasPrefix(variable, "!") {
			unrendered = append(unrendered, variable)
		}

		start = idx + closeIdx + 2
	}

	return unrendered
}

// PreserveGuidance ensures [GUIDANCE: ...] and [FILL: ...] markers are preserved
func PreserveGuidance(content string) string {
	// This is a no-op for now since Go templates don't touch [GUIDANCE]
	// markers - they're just plain text
	// This function exists for future enhancements if needed
	return content
}

// RenderWithDefaults renders a template with default values for missing variables
func RenderWithDefaults(tmpl *Template, ctx *Context) (string, error) {
	// Create a copy of the context
	enrichedCtx := NewContext()

	// Copy all fields from original context
	enrichedCtx.FestivalName = ctx.FestivalName
	enrichedCtx.FestivalGoal = ctx.FestivalGoal
	enrichedCtx.FestivalTags = ctx.FestivalTags
	enrichedCtx.FestivalDescription = ctx.FestivalDescription
	enrichedCtx.PhaseNumber = ctx.PhaseNumber
	enrichedCtx.PhaseName = ctx.PhaseName
	enrichedCtx.PhaseID = ctx.PhaseID
	enrichedCtx.PhaseType = ctx.PhaseType
	enrichedCtx.PhaseStructure = ctx.PhaseStructure
	enrichedCtx.PhaseObjective = ctx.PhaseObjective
	enrichedCtx.SequenceNumber = ctx.SequenceNumber
	enrichedCtx.SequenceName = ctx.SequenceName
	enrichedCtx.SequenceID = ctx.SequenceID
	enrichedCtx.SequenceObjective = ctx.SequenceObjective
	enrichedCtx.TaskNumber = ctx.TaskNumber
	enrichedCtx.TaskName = ctx.TaskName
	enrichedCtx.TaskID = ctx.TaskID
	enrichedCtx.TaskObjective = ctx.TaskObjective
	enrichedCtx.CurrentLevel = ctx.CurrentLevel
	enrichedCtx.ParentPhaseID = ctx.ParentPhaseID
	enrichedCtx.ParentSequenceID = ctx.ParentSequenceID
	enrichedCtx.FullPath = ctx.FullPath

	// Copy custom variables
	for k, v := range ctx.Custom {
		enrichedCtx.Custom[k] = v
	}

	// Add defaults for missing optional variables
	if tmpl.Metadata != nil {
		for _, optional := range tmpl.Metadata.OptionalVariables {
			if _, ok := enrichedCtx.Get(optional); !ok {
				// Set default value (empty string)
				enrichedCtx.SetCustom(optional, "")
			}
		}
	}

	// Render with enriched context
	renderer := NewRenderer()
	return renderer.Render(tmpl, enrichedCtx)
}
