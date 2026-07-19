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

func TestPromoteCompletionKeepsStatusOrderWithTypedPrefix(t *testing.T) {
	festivalsDir := t.TempDir()
	writePromoteFestival(t, festivalsDir, "active", "zzz-feature-ZF0003")
	writePromoteFestival(t, festivalsDir, "ready", "mmm-feature-MF0002")
	writePromoteFestival(t, festivalsDir, "planning", "aaa-feature-AF0001")

	candidates := CollectFestivalPickCandidates(festivalsDir, FestivalPickerOptions{
		PreferredStatuses:        []string{"active", "ready", "planning"},
		OrderByStatusThenRecency: true,
	})

	got := OrderedSelectorNames(candidates, "feature")
	want := []string{"zzz-feature-ZF0003", "mmm-feature-MF0002", "aaa-feature-AF0001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("typed-prefix completion order = %#v, want status order (not alphabetical) %#v", got, want)
	}
}

func TestColorSelectorCompletionsFormat(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "a-active-AA0001", Status: "active", Path: "/f/active/a-active-AA0001"},
		{Name: "b-ready-BR0001", Status: "ready", Path: "/f/ready/b-ready-BR0001"},
	}
	got := ColorSelectorCompletions(candidates, "")
	want := []string{
		"a-active-AA0001\ta-active-AA0001 \033[38;5;84mactive\033[0m",
		"b-ready-BR0001\tb-ready-BR0001 \033[38;5;214mready\033[0m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("color completions = %#v, want %#v", got, want)
	}
}
