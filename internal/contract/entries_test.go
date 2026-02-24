package contract

import (
	"testing"

	"github.com/obediencecorp/obey-shared/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFestEntries_ReturnsNonEmpty(t *testing.T) {
	entries := FestEntries()
	require.NotEmpty(t, entries, "FestEntries must return at least one entry")
}

func TestFestEntries_AllOwnedByFest(t *testing.T) {
	for _, e := range FestEntries() {
		assert.Equal(t, contract.OwnerFest, e.Owner,
			"entry %q must be owned by fest", e.ID)
	}
}

func TestFestEntries_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range FestEntries() {
		assert.False(t, seen[e.ID], "duplicate entry ID: %s", e.ID)
		seen[e.ID] = true
	}
}

func TestFestEntries_RequiredFieldsPresent(t *testing.T) {
	for _, e := range FestEntries() {
		t.Run(e.ID, func(t *testing.T) {
			assert.NotEmpty(t, e.ID, "ID must not be empty")
			assert.NotEmpty(t, e.Path, "Path must not be empty")
			assert.NotEmpty(t, e.Type, "Type must not be empty")
			assert.NotEmpty(t, string(e.Format), "Format must not be empty")
			assert.NotEmpty(t, string(e.Watch), "Watch must not be empty")
			assert.NotEmpty(t, e.Owner, "Owner must not be empty")
		})
	}
}

func TestFestEntries_ValidFormats(t *testing.T) {
	validFormats := map[contract.Format]bool{
		contract.FormatYAML:              true,
		contract.FormatJSON:              true,
		contract.FormatJSONL:             true,
		contract.FormatDirectory:         true,
		contract.FormatMarkdownFrontmatter: true,
	}
	for _, e := range FestEntries() {
		assert.True(t, validFormats[e.Format],
			"entry %q has invalid format: %s", e.ID, e.Format)
	}
}

func TestFestEntries_ValidWatchModes(t *testing.T) {
	validModes := map[contract.WatchMode]bool{
		contract.WatchFile:      true,
		contract.WatchDirectory: true,
		contract.WatchAppend:    true,
	}
	for _, e := range FestEntries() {
		assert.True(t, validModes[e.Watch],
			"entry %q has invalid watch mode: %s", e.ID, e.Watch)
	}
}

func TestFestEntries_PassesContractValidation(t *testing.T) {
	c := &contract.Contract{
		Version: contract.ContractVersion,
		Entries: FestEntries(),
	}
	err := contract.Validate(c)
	assert.NoError(t, err, "FestEntries must produce a valid contract")
}

func TestFestEntries_StatusDirEntries(t *testing.T) {
	expectedStatuses := map[string]bool{
		"planning":  false,
		"active":    false,
		"ready":     false,
		"ritual":    false,
		"archived":  false,
		"completed": false,
		"someday":   false,
	}

	for _, e := range FestEntries() {
		if e.Type == contract.TypeFestivalStatusDir {
			assert.NotEmpty(t, e.Status, "status dir entry %q must have Status", e.ID)
			assert.Equal(t, contract.FormatDirectory, e.Format,
				"status dir entry %q must use FormatDirectory", e.ID)
			assert.Equal(t, contract.WatchDirectory, e.Watch,
				"status dir entry %q must use WatchDirectory", e.ID)
			if _, ok := expectedStatuses[e.Status]; ok {
				expectedStatuses[e.Status] = true
			}
		}
	}

	for status, found := range expectedStatuses {
		assert.True(t, found, "expected status directory for %q not found", status)
	}
}

func TestFestEntries_TemplateEntries(t *testing.T) {
	var templates []contract.Entry
	for _, e := range FestEntries() {
		if e.Template {
			templates = append(templates, e)
		}
	}

	require.NotEmpty(t, templates, "should have template entries")

	for _, e := range templates {
		assert.Contains(t, e.Path, "{festival}",
			"template entry %q path must contain {festival} placeholder", e.ID)
	}
}

func TestFestEntries_FallbackPaths(t *testing.T) {
	for _, e := range FestEntries() {
		if len(e.FallbackPaths) > 0 {
			t.Run(e.ID, func(t *testing.T) {
				for _, fp := range e.FallbackPaths {
					assert.NotEmpty(t, fp, "fallback path must not be empty")
					assert.NotEqual(t, e.Path, fp,
						"fallback path must differ from primary path")
				}
			})
		}
	}
}

func TestFestEntries_ExpectedCount(t *testing.T) {
	entries := FestEntries()
	// 5 methodology state + 7 status dirs + 3 templates = 15
	assert.Len(t, entries, 15, "expected 15 fest entries")
}
