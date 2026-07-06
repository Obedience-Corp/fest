package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writePromoteFestival(t *testing.T, festivalsDir, status, name string) {
	t.Helper()
	dir := filepath.Join(festivalsDir, status, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusColor(t *testing.T) {
	cases := map[string]string{
		"active":            "\033[38;5;42m",
		"ready":             "\033[38;5;220m",
		"planning":          "\033[38;5;33m",
		"completed":         "\033[38;5;205m",
		"dungeon/completed": "\033[38;5;248m",
		"mystery":           "\033[2m",
	}
	for status, want := range cases {
		if got := StatusColor(status); got != want {
			t.Errorf("StatusColor(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestPromoteCompletionOrdersByStatusNotAlphabetical(t *testing.T) {
	festivalsDir := t.TempDir()
	writePromoteFestival(t, festivalsDir, "active", "zzz-active-ZA0001")
	writePromoteFestival(t, festivalsDir, "ready", "mmm-ready-MR0001")
	writePromoteFestival(t, festivalsDir, "planning", "aaa-plan-AP0001")

	candidates := CollectFestivalPickCandidates(festivalsDir, FestivalPickerOptions{
		PreferredStatuses:        []string{"active", "ready", "planning"},
		OrderByStatusThenRecency: true,
	})

	got := OrderedSelectorNames(candidates, "")
	want := []string{"zzz-active-ZA0001", "mmm-ready-MR0001", "aaa-plan-AP0001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("promote completion order = %#v, want status order (not alphabetical) %#v", got, want)
	}
}

func TestColorSelectorCompletionsFormat(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "a-active-AA0001", Status: "active", Path: "/f/active/a-active-AA0001"},
		{Name: "b-ready-BR0001", Status: "ready", Path: "/f/ready/b-ready-BR0001"},
	}
	got := ColorSelectorCompletions(candidates, "")
	want := []string{
		"a-active-AA0001\ta-active-AA0001 \033[38;5;42mactive\033[0m",
		"b-ready-BR0001\tb-ready-BR0001 \033[38;5;220mready\033[0m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("color completions = %#v, want %#v", got, want)
	}
}
