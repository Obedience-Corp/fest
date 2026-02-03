//go:build integration
// +build integration

package lifecycle

import (
	"os"
	"testing"
)

var sharedContainer *TestContainer

func TestMain(m *testing.M) {
	// Disable ryuk reaper for Colima compatibility
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	var err error
	sharedContainer, err = NewSharedContainer()
	if err != nil {
		os.Stderr.WriteString("Failed to create shared container: " + err.Error() + "\n")
		os.Exit(1)
	}

	code := m.Run()

	sharedContainer.Cleanup()
	os.Exit(code)
}
