package commands

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

// TestCreateFestivalOptsCarryProjectAndSeed documents the shared opts surface
// the TUI must populate so the hook can map into festival.CreateFestivalOptions.
func TestCreateFestivalOptsCarryProjectAndSeed(t *testing.T) {
	t.Parallel()
	opts := &shared.CreateFestivalOpts{
		Name:     "checkout-hardening",
		Goal:     "Harden checkout",
		Tags:     "payments",
		Type:     "standard",
		Project:  "projects/demo-app",
		Seed:     "prior art from support",
		SeedFile: "",
		Dest:     "planning",
	}
	if opts.Project == "" || opts.Seed == "" {
		t.Fatal("Project and Seed must be settable on CreateFestivalOpts for human TUI")
	}
	if opts.Name != "checkout-hardening" || opts.Type != "standard" {
		t.Fatalf("unexpected core fields: %+v", opts)
	}
}
