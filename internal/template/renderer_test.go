package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create context with custom variables for tests
func newTestContext(vars map[string]interface{}) *Context {
	ctx := NewContext()
	for k, v := range vars {
		ctx.SetCustom(k, v)
	}
	return ctx
}

func TestRenderer_RenderString(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		context  *Context
		expected string
		wantErr  bool
	}{
		{
			name:    "simple variable substitution",
			content: "Hello {{.name}}!",
			context: newTestContext(map[string]interface{}{
				"name": "World",
			}),
			expected: "Hello World!",
			wantErr:  false,
		},
		{
			name:    "multiple variables",
			content: "Festival: {{.festival_name}}\nGoal: {{.goal}}",
			context: newTestContext(map[string]interface{}{
				"festival_name": "my-festival",
				"goal":          "Build awesome things",
			}),
			expected: "Festival: my-festival\nGoal: Build awesome things",
			wantErr:  false,
		},
		{
			name:     "sprig functions - default",
			content:  "Name: {{.name | default \"Unknown\"}}",
			context:  newTestContext(map[string]interface{}{}),
			expected: "Name: Unknown",
			wantErr:  false,
		},
		{
			name:    "sprig functions - upper",
			content: "Name: {{.name | upper}}",
			context: newTestContext(map[string]interface{}{
				"name": "festival",
			}),
			expected: "Name: FESTIVAL",
			wantErr:  false,
		},
		{
			name:     "preserve guidance markers",
			content:  "# Title\n\n[GUIDANCE: Fill this in]\n\n[FILL: Add description]",
			context:  newTestContext(map[string]interface{}{}),
			expected: "# Title\n\n[GUIDANCE: Fill this in]\n\n[FILL: Add description]",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()
			output, err := renderer.RenderString(tt.content, tt.context)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestRenderer_Render(t *testing.T) {
	tmpl := &Template{
		Path: "test.md",
		Metadata: &Metadata{
			TemplateID:        "TEST",
			RequiredVariables: []string{"name"},
		},
		Content: "Hello {{.name}}!",
	}

	ctx := newTestContext(map[string]interface{}{
		"name": "World",
	})

	renderer := NewRenderer()
	output, err := renderer.Render(tmpl, ctx)

	require.NoError(t, err)
	assert.Equal(t, "Hello World!", output)
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		metadata *Metadata
		context  *Context
		wantErr  bool
	}{
		{
			name: "all required variables present",
			metadata: &Metadata{
				RequiredVariables: []string{"name", "goal"},
			},
			context: newTestContext(map[string]interface{}{
				"name": "test",
				"goal": "test goal",
			}),
			wantErr: false,
		},
		{
			name: "missing required variable",
			metadata: &Metadata{
				RequiredVariables: []string{"name", "goal"},
			},
			context: newTestContext(map[string]interface{}{
				"name": "test",
			}),
			wantErr: true,
		},
		{
			name:     "no metadata",
			metadata: nil,
			context:  newTestContext(map[string]interface{}{}),
			wantErr:  false,
		},
		{
			name: "optional variables missing",
			metadata: &Metadata{
				RequiredVariables: []string{"name"},
				OptionalVariables: []string{"description"},
			},
			context: newTestContext(map[string]interface{}{
				"name": "test",
			}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{
				Metadata: tt.metadata,
			}

			err := ValidateTemplate(tmpl, tt.context)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckUnrenderedVariables(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantUnrendered []string
	}{
		{
			name:           "no unrendered variables",
			content:        "Hello World! This is content.",
			wantUnrendered: []string{},
		},
		{
			name:           "one unrendered variable",
			content:        "Hello {{name}}!",
			wantUnrendered: []string{"name"},
		},
		{
			name:           "multiple unrendered variables",
			content:        "{{festival_name}} - {{goal}} - {{description}}",
			wantUnrendered: []string{"festival_name", "goal", "description"},
		},
		{
			name:           "guidance markers not counted",
			content:        "[GUIDANCE: Fill this in]\n[FILL: Add description]",
			wantUnrendered: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unrendered := CheckUnrenderedVariables(tt.content)
			assert.ElementsMatch(t, tt.wantUnrendered, unrendered)
		})
	}
}

func TestRenderWithDefaults(t *testing.T) {
	tmpl := &Template{
		Metadata: &Metadata{
			OptionalVariables: []string{"description", "owner"},
		},
		Content: "Name: {{.name}}\nDescription: {{.description}}\nOwner: {{.owner}}",
	}

	ctx := newTestContext(map[string]interface{}{
		"name": "Test Festival",
	})

	output, err := RenderWithDefaults(tmpl, ctx)
	require.NoError(t, err)

	// Optional variables should have empty defaults
	assert.Contains(t, output, "Name: Test Festival")
	assert.Contains(t, output, "Description: ")
	assert.Contains(t, output, "Owner: ")
}

func TestPreserveGuidance(t *testing.T) {
	content := `# Title

[GUIDANCE: This should be preserved]

Some content

[FILL: This should also be preserved]`

	result := PreserveGuidance(content)
	assert.Equal(t, content, result)
}

func TestRenderWithMarkerReplacement(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		ctx      *Context
		config   map[string]string
		expected string
		wantErr  bool
	}{
		{
			name:    "replaces festival markers",
			content: "# [REPLACE: Festival Name]\n\nGoal: [REPLACE: Festival goal]",
			ctx: func() *Context {
				ctx := NewContext()
				ctx.SetFestival("My Festival", "Build awesome things", nil)
				ctx.SetFestivalID("MF0001")
				return ctx
			}(),
			config:   nil,
			expected: "# My Festival\n\nGoal: Build awesome things",
			wantErr:  false,
		},
		{
			name:    "replaces phase markers",
			content: "# Phase: [REPLACE: Phase name]\nOrder: [REPLACE: Phase order]",
			ctx: func() *Context {
				ctx := NewContext()
				ctx.SetPhase(1, "Planning", "planning")
				return ctx
			}(),
			config:   nil,
			expected: "# Phase: Planning\nOrder: 1",
			wantErr:  false,
		},
		{
			name:    "replaces task markers",
			content: "# Task: [REPLACE: Task name]\nFile: [REPLACE: NN_task_name]",
			ctx: func() *Context {
				ctx := NewContext()
				ctx.SetTask(5, "User Research")
				return ctx
			}(),
			config:   nil,
			expected: "# Task: User Research\nFile: 05_user_research",
			wantErr:  false,
		},
		{
			name:    "replaces config markers",
			content: "Run: [REPLACE: Run your project's lint command]",
			ctx: func() *Context {
				ctx := NewContext()
				return ctx
			}(),
			config: map[string]string{
				"lint_command": "just lint",
			},
			expected: "Run: just lint",
			wantErr:  false,
		},
		{
			name:     "nil context returns rendered content",
			content:  "# Title\n[REPLACE: Festival Name]",
			ctx:      nil,
			config:   nil,
			expected: "# Title\n[REPLACE: Festival Name]",
			wantErr:  false,
		},
		{
			name:    "preserves markers with no replacement",
			content: "# [REPLACE: Festival Name]\n[REPLACE: Unknown Marker]",
			ctx: func() *Context {
				ctx := NewContext()
				ctx.SetFestival("My Festival", "", nil)
				return ctx
			}(),
			config:   nil,
			expected: "# My Festival\n[REPLACE: Unknown Marker]",
			wantErr:  false,
		},
		{
			name:    "combines go template and marker replacement",
			content: "# {{.festival_name}}\n\nGoal: [REPLACE: Festival goal]",
			ctx: func() *Context {
				ctx := NewContext()
				ctx.SetFestival("My Festival", "Build stuff", nil)
				return ctx
			}(),
			config:   nil,
			expected: "# My Festival\n\nGoal: Build stuff",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()
			output, err := renderer.RenderWithMarkerReplacement(tt.content, tt.ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestReplaceMarkers(t *testing.T) {
	content := "# [REPLACE: Festival Name]\n\n[REPLACE: Phase name] - [REPLACE: Task name]"

	replacements := map[string]string{
		"[REPLACE: Festival Name]": "Test Festival",
		"[REPLACE: Phase name]":    "Planning",
		"[REPLACE: Task name]":     "Research",
	}

	result := ReplaceMarkers(content, replacements)

	assert.Equal(t, "# Test Festival\n\nPlanning - Research", result)
}

func TestReplaceMarkers_EmptyValue(t *testing.T) {
	content := "[REPLACE: Festival Name] [REPLACE: Phase name]"

	replacements := map[string]string{
		"[REPLACE: Festival Name]": "Test",
		"[REPLACE: Phase name]":    "", // Empty - should not replace
	}

	result := ReplaceMarkers(content, replacements)

	assert.Equal(t, "Test [REPLACE: Phase name]", result)
}

func TestCountMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "no markers",
			content: "# Title\nSome content",
			want:    0,
		},
		{
			name:    "one marker",
			content: "# [REPLACE: Festival Name]",
			want:    1,
		},
		{
			name:    "multiple markers",
			content: "[REPLACE: A] [REPLACE: B] [REPLACE: C]",
			want:    3,
		},
		{
			name:    "duplicate markers",
			content: "[REPLACE: A] [REPLACE: A] [REPLACE: B]",
			want:    3, // Counts all occurrences
		},
		{
			name:    "guidance markers not counted",
			content: "[REPLACE: A] [GUIDANCE: B] [FILL: C]",
			want:    1, // Only REPLACE markers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CountMarkers(tt.content))
		})
	}
}

func TestFindUnfilledMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no markers",
			content: "# Title\nSome content",
			want:    []string{},
		},
		{
			name:    "one marker",
			content: "# [REPLACE: Festival Name]",
			want:    []string{"[REPLACE: Festival Name]"},
		},
		{
			name:    "multiple unique markers",
			content: "[REPLACE: A] [REPLACE: B]",
			want:    []string{"[REPLACE: A]", "[REPLACE: B]"},
		},
		{
			name:    "duplicate markers returns unique",
			content: "[REPLACE: A] [REPLACE: A] [REPLACE: B]",
			want:    []string{"[REPLACE: A]", "[REPLACE: B]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindUnfilledMarkers(tt.content)
			assert.ElementsMatch(t, tt.want, result)
		})
	}
}
