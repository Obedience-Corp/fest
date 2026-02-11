package workflow

// DefaultSchemaV2 returns the default workflow schema version 2.
// V2 uses a dungeon-centric model where:
//   - Root directory (`.`) = active work (default status)
//   - All other statuses live under `dungeon/`
//
// This eliminates the need for separate `active/` and `ready/` directories.
func DefaultSchemaV2() *Schema {
	return &Schema{
		Version:       2,
		Type:          SchemaType,
		Name:          "Workflow",
		Description:   "Dungeon-centric workflow for organizing work",
		DefaultStatus: ".",
		TrackHistory:  true,
		HistoryFile:   DefaultHistoryFile,
		Directories: map[string]Directory{
			".": {
				Description:    "Active work in progress",
				Order:          1,
				TransitionOpts: []string{"dungeon"},
			},
			"dungeon": {
				Description: "All non-active statuses",
				Order:       2,
				Nested:      true,
				Children: map[string]Directory{
					"ready": {
						Description: "Ready to work on",
						Order:       1,
					},
					"completed": {
						Description: "Successfully finished work",
						Order:       2,
					},
					"archived": {
						Description: "Preserved but no longer active",
						Order:       3,
					},
					"someday": {
						Description: "Backlog — nice-to-have work",
						Order:       4,
					},
				},
			},
		},
	}
}

// DefaultSchema returns the default workflow schema.
// This schema provides a standard structure for organizing work:
// - active: Work in progress
// - ready: Prepared for action
// - dungeon: Archive area with nested subdirectories
func DefaultSchema() *Schema {
	return &Schema{
		Version:       CurrentSchemaVersion,
		Type:          SchemaType,
		Name:          "Workflow",
		Description:   "Status workflow for organizing work",
		DefaultStatus: "active",
		TrackHistory:  true,
		HistoryFile:   DefaultHistoryFile,
		Directories: map[string]Directory{
			"active": {
				Description:    "Work actively being done",
				Order:          1,
				TransitionOpts: []string{"ready", "dungeon"},
			},
			"ready": {
				Description:    "Prepared and ready for action",
				Order:          2,
				TransitionOpts: []string{"active", "dungeon"},
			},
			"dungeon": {
				Description: "Archive area for completed, archived, or deferred work",
				Order:       3,
				Nested:      true,
				Children: map[string]Directory{
					"completed": {
						Description: "Successfully finished work",
						Order:       1,
					},
					"archived": {
						Description: "Preserved but no longer active",
						Order:       2,
					},
					"someday": {
						Description: "Backlog — nice-to-have work",
						Order:       3,
					},
				},
			},
		},
	}
}

// FestivalSchema returns the workflow schema for festival lifecycle management.
// Festivals have a unique lifecycle with a planning stage:
//   - planning: Being designed and documented
//   - ready: Planned and ready for execution
//   - active: Currently being worked on
//   - dungeon: Terminal states (completed, archived, someday)
func FestivalSchema() *Schema {
	return &Schema{
		Version:       CurrentSchemaVersion,
		Type:          SchemaType,
		Name:          "Festival Workflow",
		Description:   "Status workflow for festival lifecycle management",
		DefaultStatus: "planning",
		TrackHistory:  true,
		HistoryFile:   DefaultHistoryFile,
		Directories: map[string]Directory{
			"planning": {
				Description:    "Festivals being designed and documented",
				Order:          1,
				TransitionOpts: []string{"ready", "active", "dungeon"},
			},
			"ready": {
				Description:    "Planned and ready for execution",
				Order:          2,
				TransitionOpts: []string{"active", "dungeon"},
			},
			"active": {
				Description:    "Festivals currently being executed",
				Order:          3,
				TransitionOpts: []string{"dungeon"},
			},
			"dungeon": {
				Description: "Terminal festival states",
				Order:       4,
				Nested:      true,
				Children: map[string]Directory{
					"completed": {
						Description: "Successfully finished festivals",
						Order:       1,
					},
					"archived": {
						Description: "Preserved but no longer active",
						Order:       2,
					},
					"someday": {
						Description: "Backlog — nice-to-have work",
						Order:       3,
					},
				},
			},
		},
	}
}
