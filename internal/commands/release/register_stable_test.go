//go:build !dev

package release_test

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/release"
	"github.com/spf13/cobra"
)

func TestRegisterStable_ExploreCommandNotRegistered(t *testing.T) {
	root := &cobra.Command{Use: "test"}

	release.Register(root)

	for _, cmd := range root.Commands() {
		if cmd.Name() == "explore" {
			t.Fatal("explore command should not be registered in stable profile")
		}
	}
}

func TestRegisterStable_TuiCommandNotRegistered(t *testing.T) {
	root := &cobra.Command{Use: "test"}

	release.Register(root)

	for _, cmd := range root.Commands() {
		if cmd.Name() == "tui" {
			t.Fatal("tui command should not be registered in stable profile")
		}
	}
}
