//go:build dev

package release_test

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/release"
	_ "github.com/Obedience-Corp/fest/internal/commands/tui" // registers shared.NewTUICommand
	"github.com/spf13/cobra"
)

func TestRegisterDev_ExploreCommandRegistered(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.AddGroup(&cobra.Group{ID: "navigation", Title: "Navigation"})

	release.Register(root)

	cmd, _, err := root.Find([]string{"explore"})
	if err != nil {
		t.Fatalf("expected explore command in dev profile: %v", err)
	}
	if cmd == nil || cmd.Name() != "explore" {
		t.Fatalf("expected explore command, got %#v", cmd)
	}
	if got := cmd.Annotations[release.AnnotationReleaseChannel]; got != release.ReleaseChannelDevOnly {
		t.Fatalf("explore release_channel = %q, want %q", got, release.ReleaseChannelDevOnly)
	}
}

func TestRegisterDev_TuiCommandRegistered(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.AddGroup(&cobra.Group{ID: "navigation", Title: "Navigation"})
	root.AddGroup(&cobra.Group{ID: "creation", Title: "Creation"})

	release.Register(root)

	cmd, _, err := root.Find([]string{"tui"})
	if err != nil {
		t.Fatalf("expected tui command in dev profile: %v", err)
	}
	if cmd == nil || cmd.Name() != "tui" {
		t.Fatalf("expected tui command, got %#v", cmd)
	}
	if got := cmd.Annotations[release.AnnotationReleaseChannel]; got != release.ReleaseChannelDevOnly {
		t.Fatalf("tui release_channel = %q, want %q", got, release.ReleaseChannelDevOnly)
	}
}
