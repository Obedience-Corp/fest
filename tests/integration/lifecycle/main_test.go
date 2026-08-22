//go:build integration
// +build integration

package lifecycle

import (
	"context"
	"os"
	"testing"

	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
)

var sharedContainer *TestContainer

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		os.Stderr.WriteString("failed to disable Ryuk: " + err.Error() + "\n")
		return 1
	}

	ctx := context.Background()
	daemon, err := prepareDaemon(ctx)
	if err != nil {
		reportInfrastructureRefusal(err.Error())
		return 1
	}
	defer func() {
		if releaseErr := daemon.Close(); releaseErr != nil {
			os.Stderr.WriteString("could not release the suite lock: " + releaseErr.Error() + "\n")
		}
	}()

	if err := probeDaemon(ctx, os.Getenv(itestenv.DockerHostVar)); err != nil {
		reportInfrastructureRefusal(err.Error())
		return 1
	}

	sharedContainer, err = NewSharedContainer()
	if err != nil {
		if isCodeFailure(err) {
			os.Stderr.WriteString("Failed to create shared container: " + err.Error() + "\n")
			return 1
		}
		reportInfrastructureRefusal(err.Error())
		return 1
	}

	code := m.Run()
	sharedContainer.Cleanup()
	return code
}
