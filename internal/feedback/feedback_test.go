package feedback

import (
	"context"
	"strings"
	"testing"
)

func TestStoreInitNormalizesCriteria(t *testing.T) {
	store := NewStore(t.TempDir())

	config, err := store.Init(context.Background(), []string{
		"Onboarding friction, especially copied commands",
		" ",
		"onboarding friction, especially copied commands",
		"Release blockers",
	})
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if len(config.Criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(config.Criteria))
	}
	if got := config.Criteria[0].Name; got != "Onboarding friction, especially copied commands" {
		t.Fatalf("criteria[0] = %q", got)
	}
}

func TestStoreReplaceCriteriaPreservesObservations(t *testing.T) {
	ctx := context.Background()
	store := NewStore(t.TempDir())

	if _, err := store.Init(ctx, []string{"Original"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if err := store.AddObservation(ctx, &Observation{
		Criteria:    "Original",
		Observation: "something to preserve",
	}); err != nil {
		t.Fatalf("AddObservation() error: %v", err)
	}

	config, err := store.ReplaceCriteria(ctx, []string{"Replacement"})
	if err != nil {
		t.Fatalf("ReplaceCriteria() error: %v", err)
	}
	if len(config.Criteria) != 1 || config.Criteria[0].Name != "Replacement" {
		t.Fatalf("criteria = %#v, want Replacement", config.Criteria)
	}

	observations, err := store.ListObservations(ctx, "", "")
	if err != nil {
		t.Fatalf("ListObservations() error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("observations count = %d, want 1", len(observations))
	}
}

func TestStoreAddCriteriaAppendsUniqueNames(t *testing.T) {
	ctx := context.Background()
	store := NewStore(t.TempDir())

	if _, err := store.Init(ctx, []string{"Usability"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	config, err := store.AddCriteria(ctx, []string{
		"usability",
		"Docs, examples, and copied commands",
	})
	if err != nil {
		t.Fatalf("AddCriteria() error: %v", err)
	}

	if len(config.Criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(config.Criteria))
	}
	if got := config.Criteria[1].Name; got != "Docs, examples, and copied commands" {
		t.Fatalf("criteria[1] = %q", got)
	}
}

func TestStoreUnknownCriteriaListsValidNames(t *testing.T) {
	ctx := context.Background()
	store := NewStore(t.TempDir())

	if _, err := store.Init(ctx, []string{"Usability", "Release blockers"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	err := store.AddObservation(ctx, &Observation{
		Criteria:    "Missing",
		Observation: "cannot classify",
	})
	if err == nil {
		t.Fatal("AddObservation() error = nil, want validation error")
	}

	msg := err.Error()
	for _, want := range []string{"unknown criteria", "Usability", "Release blockers", "use one of"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}
