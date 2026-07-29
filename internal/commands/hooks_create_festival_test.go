package commands

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

func TestFestivalOptsFromSharedMapsProjectAndSeed(t *testing.T) {
	t.Parallel()
	opts := &shared.CreateFestivalOpts{
		Name:       "checkout-hardening",
		Goal:       "Harden checkout",
		Tags:       "payments",
		Type:       "standard",
		VarsFile:   "/tmp/vars.json",
		Project:    "projects/demo-app",
		Seed:       "prior art from support",
		SeedFile:   "/tmp/seed.md",
		JSONOutput: true,
		Dest:       "planning",
	}
	got := festivalOptsFromShared(opts)
	if got.Name != opts.Name || got.Goal != opts.Goal || got.Tags != opts.Tags || got.Type != opts.Type {
		t.Fatalf("core fields not mapped: %+v", got)
	}
	if got.Project != opts.Project {
		t.Fatalf("Project = %q, want %q", got.Project, opts.Project)
	}
	if got.Seed != opts.Seed {
		t.Fatalf("Seed = %q, want %q", got.Seed, opts.Seed)
	}
	if got.SeedFile != opts.SeedFile {
		t.Fatalf("SeedFile = %q, want %q", got.SeedFile, opts.SeedFile)
	}
	if got.VarsFile != opts.VarsFile || got.Dest != opts.Dest || got.JSONOutput != opts.JSONOutput {
		t.Fatalf("secondary fields not mapped: %+v", got)
	}
}

func TestFestivalOptsFromSharedNilSafe(t *testing.T) {
	t.Parallel()
	got := festivalOptsFromShared(nil)
	if got == nil {
		t.Fatal("expected non-nil options")
	}
}
