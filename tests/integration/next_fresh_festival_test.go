//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/require"
)

// freshFestivalRootMarkers is the number of unfilled markers a bare
// `fest create festival` leaves in the festival's root documents, using the
// methodology templates the harness mounts. It is asserted exactly so a
// template edit that changes the first-run experience shows up here.
const freshFestivalRootMarkers = 21

type nextStepOutput struct {
	Kind             string          `json:"kind"`
	Task             json.RawMessage `json:"task"`
	Reason           string          `json:"reason"`
	FestivalComplete bool            `json:"festival_complete"`
	FestivalPlanning *struct {
		Status      string `json:"status"`
		PhaseCount  int    `json:"phase_count"`
		Goal        string `json:"goal"`
		MarkerTotal int    `json:"marker_total"`
		MarkerFiles []struct {
			File    string `json:"file"`
			Count   int    `json:"count"`
			Markers []struct {
				Line int    `json:"line"`
				Hint string `json:"hint"`
			} `json:"markers"`
		} `json:"marker_files"`
		NextCommands []string `json:"next_commands"`
	} `json:"festival_planning"`
}

type validateOutput struct {
	Valid          bool `json:"valid"`
	MarkersPending bool `json:"markers_pending"`
	Issues         []struct {
		Level string `json:"level"`
		Code  string `json:"code"`
		Path  string `json:"path"`
	} `json:"issues"`
}

// runFestExit runs fest in a directory and returns its combined output with the
// process exit status, so a test can assert on the status a shell would see.
func runFestExit(t *testing.T, tc *TestContainer, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := []string{"sh", "-c", "cd " + dir + " && /fest " + strings.Join(args, " ")}
	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	require.NoError(t, err, "exec fest %v", args)

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	require.NoError(t, err, "demultiplex fest output")
	return stdout.String() + stderr.String(), exitCode
}

// initFreshFestival runs the real first-run path: fest init, then a bare
// fest create festival. It returns the new festival's directory.
func initFreshFestival(t *testing.T, tc *TestContainer, basePath, name string, createArgs ...string) string {
	t.Helper()

	_, err := tc.runCommand([]string{"mkdir", "-p", basePath})
	require.NoError(t, err, "create camp directory")

	output, exit := runFestExit(t, tc, basePath, "init")
	require.Zero(t, exit, "fest init should succeed: %s", output)

	args := append([]string{"create", "festival", "--name", name}, createArgs...)
	output, exit = runFestExit(t, tc, basePath, args...)
	require.Zero(t, exit, "fest create festival should succeed: %s", output)

	festivalsPath := filepath.Join(basePath, "festivals", "planning")
	dirs, err := tc.ListDirectories(festivalsPath)
	require.NoError(t, err)
	require.Len(t, dirs, 1, "create should leave exactly one planning festival")
	return filepath.Join(festivalsPath, dirs[0])
}

func TestNextOnFreshFestivalReturnsThePlanningStep(t *testing.T) {
	tc := GetSharedContainer(t)
	festivalPath := initFreshFestival(t, tc, "/fresh-next-camp", "demo-thing")

	output, exit := runFestExit(t, tc, festivalPath, "next", "--no-color")
	require.Zero(t, exit, "fest next on a fresh festival must not stop: %s", output)
	require.Contains(t, output, "FESTIVAL PLANNING")
	require.Contains(t, output, "no phases yet")
	require.NotContains(t, output, "STOP",
		"a scaffold's own markers are expected while it is in planning")
	require.NotContains(t, strings.ToUpper(output), "FESTIVAL COMPLETE",
		"an unwritten festival must not read as complete")

	for _, file := range []string{"FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md", "TODO.md"} {
		require.Contains(t, output, file, "the step must name every file holding markers")
	}
	require.Contains(t, output, "[REPLACE:",
		"the step must inline the marker hints an agent has to fill")
	for _, command := range []string{
		"fest understand planning",
		"fest understand structure",
		"fest create phase --name PHASE_NAME --type TYPE",
		"fest create sequence --name SEQUENCE_NAME",
		"fest create task --name TASK_NAME",
		"fest validate",
		"fest promote",
	} {
		require.Contains(t, output, command, "the step must give the build commands")
	}

	jsonOutput, exit := runFestExit(t, tc, festivalPath, "next", "--json")
	require.Zero(t, exit, "fest next --json must exit 0 so the hub reads the festival as runnable: %s", jsonOutput)

	var step nextStepOutput
	require.NoError(t, json.Unmarshal([]byte(jsonOutput), &step), "fest next --json output: %s", jsonOutput)
	require.Equal(t, "festival_planning", step.Kind)
	require.Nil(t, step.Task, "a planning step names no task")
	require.False(t, step.FestivalComplete)
	require.NotEmpty(t, step.Reason)
	require.NotNil(t, step.FestivalPlanning)
	require.Equal(t, "planning", step.FestivalPlanning.Status)
	require.Zero(t, step.FestivalPlanning.PhaseCount)
	require.Equal(t, freshFestivalRootMarkers, step.FestivalPlanning.MarkerTotal)
	require.NotEmpty(t, step.FestivalPlanning.NextCommands)

	counted := 0
	files := make([]string, 0, len(step.FestivalPlanning.MarkerFiles))
	for _, file := range step.FestivalPlanning.MarkerFiles {
		files = append(files, file.File)
		counted += file.Count
		require.Len(t, file.Markers, file.Count, "%s must list every marker it counts", file.File)
		for _, marker := range file.Markers {
			require.Positive(t, marker.Line)
			require.NotEmpty(t, marker.Hint, "an agent fills markers by hint")
		}
	}
	require.Equal(t, []string{"FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md", "TODO.md"}, files)
	require.Equal(t, step.FestivalPlanning.MarkerTotal, counted,
		"marker_total must be the sum of the per-file counts")
}

func TestValidateMarkerLevelsFollowFestivalStatus(t *testing.T) {
	tc := GetSharedContainer(t)

	tests := []struct {
		name           string
		camp           string
		promote        bool
		wantValid      bool
		wantExit       int
		wantLevel      string
		wantNextExit   int
		wantNextOutput string
	}{
		{
			// Error case first: once the festival is promoted the plan is
			// supposed to be written, so the same markers are failures.
			name:           "ready festival fails on unfilled markers",
			camp:           "/marker-status-ready-camp",
			promote:        true,
			wantValid:      false,
			wantExit:       1,
			wantLevel:      "error",
			wantNextExit:   1,
			wantNextOutput: "STOP: FESTIVAL VALIDATION FAILED",
		},
		{
			name:           "planning festival passes with markers pending",
			camp:           "/marker-status-planning-camp",
			promote:        false,
			wantValid:      true,
			wantExit:       0,
			wantLevel:      "warning",
			wantNextExit:   0,
			wantNextOutput: "FESTIVAL PLANNING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := initFreshFestival(t, tc, tt.camp, "status-thing")
			if tt.promote {
				output, exit := runFestExit(t, tc, path, "promote", "--no-commit")
				require.Zero(t, exit, "promote to ready should succeed: %s", output)
				path = strings.Replace(path, "/planning/", "/ready/", 1)
			}

			jsonOutput, exit := runFestExit(t, tc, path, "validate", "--json")
			require.Equal(t, tt.wantExit, exit, "fest validate --json exit status: %s", jsonOutput)

			var result validateOutput
			require.NoError(t, json.Unmarshal([]byte(jsonOutput), &result), "validate output: %s", jsonOutput)
			require.Equal(t, tt.wantValid, result.Valid)
			require.True(t, result.MarkersPending, "markers_pending records the markers either way")

			markerIssues := 0
			for _, issue := range result.Issues {
				if issue.Code != "unfilled_template" {
					continue
				}
				markerIssues++
				require.Equal(t, tt.wantLevel, issue.Level,
					"%s marker level must match the lifecycle status", issue.Path)
			}
			require.NotZero(t, markerIssues, "the festival still holds markers: %s", jsonOutput)

			nextOutput, nextExit := runFestExit(t, tc, path, "next", "--no-color")
			require.Equal(t, tt.wantNextExit, nextExit, "fest next exit status: %s", nextOutput)
			require.Contains(t, nextOutput, tt.wantNextOutput)
			require.NotContains(t, nextOutput, "\u2014", "user-visible copy must not use em dashes")
		})
	}
}

func TestNextNarrowsToPhasesOnceTheDocumentsAreWritten(t *testing.T) {
	tc := GetSharedContainer(t)
	basePath := "/written-docs-camp"
	goal := "Ship a reliable first-run experience"

	// Discover the marker hints the way the docs tell an agent to, then create
	// the festival with every one of them filled.
	_, err := tc.runCommand([]string{"mkdir", "-p", basePath})
	require.NoError(t, err)
	output, exit := runFestExit(t, tc, basePath, "init")
	require.Zero(t, exit, "fest init should succeed: %s", output)

	discovery, exit := runFestExit(t, tc, basePath,
		"create", "festival", "--name", "written-thing", "--dry-run", "--json")
	require.Zero(t, exit, "marker discovery should succeed: %s", discovery)

	var preview createFestivalPreviewResult
	require.NoError(t, json.Unmarshal([]byte(discovery), &preview), "dry-run output: %s", discovery)
	require.NotEmpty(t, preview.Markers)

	values := make(map[string]string, len(preview.Markers))
	for _, marker := range preview.Markers {
		values[marker.Hint] = "written: " + marker.Hint
	}
	encoded, err := json.Marshal(values)
	require.NoError(t, err)
	markersFile := "/tmp/written-thing-markers.json"
	require.NoError(t, tc.WriteFile(markersFile, string(encoded)))

	output, exit = runFestExit(t, tc, basePath,
		"create", "festival", "--name", "written-thing",
		"--goal", fmt.Sprintf("%q", goal),
		"--markers-file", markersFile)
	require.Zero(t, exit, "create with a full markers file should succeed: %s", output)

	dirs, err := tc.ListDirectories(filepath.Join(basePath, "festivals", "planning"))
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	festivalPath := filepath.Join(basePath, "festivals", "planning", dirs[0])

	stepOutput, exit := runFestExit(t, tc, festivalPath, "next", "--no-color")
	require.Zero(t, exit, "a written festival with no phases still gets a step: %s", stepOutput)
	require.Contains(t, stepOutput, "FESTIVAL PLANNING")
	require.Contains(t, stepOutput, "documents are written")
	require.Contains(t, stepOutput, goal, "a written goal belongs in the step")
	require.NotContains(t, stepOutput, "Unfilled Markers",
		"nothing is left to fill, so the inventory must be gone")
	require.NotContains(t, stepOutput, "wizard fill",
		"nothing is left to fill, so the fill instruction must be gone")
	require.Contains(t, stepOutput, "fest create phase --name PHASE_NAME --type TYPE")

	jsonOutput, exit := runFestExit(t, tc, festivalPath, "next", "--json")
	require.Zero(t, exit)
	var step nextStepOutput
	require.NoError(t, json.Unmarshal([]byte(jsonOutput), &step), "next --json output: %s", jsonOutput)
	require.Equal(t, "festival_planning", step.Kind)
	require.NotNil(t, step.FestivalPlanning)
	require.Equal(t, goal, step.FestivalPlanning.Goal)
	require.Zero(t, step.FestivalPlanning.MarkerTotal)
	require.Empty(t, step.FestivalPlanning.MarkerFiles)

	// Adding a phase hands routing back to the normal fest next path.
	output, exit = runFestExit(t, tc, festivalPath,
		"create", "phase", "--name", "PLAN", "--type", "planning")
	require.Zero(t, exit, "create phase should succeed: %s", output)

	stepOutput, exit = runFestExit(t, tc, festivalPath, "next", "--no-color")
	require.Zero(t, exit, "fest next after a phase exists: %s", stepOutput)
	require.NotContains(t, stepOutput, "FESTIVAL PLANNING",
		"once a phase exists the planning step must stop firing")
}
