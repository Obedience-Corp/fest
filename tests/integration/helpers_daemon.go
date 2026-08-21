//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func prepareDaemon(ctx context.Context) (*itestenv.Suite, error) {
	suite, err := itestenv.OpenSuite(ctx, itestenv.Options{AutoStart: true, Out: os.Stdout})
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, suite.Resolution.Line())
	if suite.Resolution.Source == itestenv.SourceFallback {
		fmt.Println(suite.Resolution.Line())
	}
	return suite, nil
}

func probeDaemon(ctx context.Context, dockerHost string) error {
	result := itestenv.Probe(ctx, dockerHost)
	fmt.Fprintln(os.Stderr, result.Line())
	if degraded, reason := result.Degraded(); degraded {
		return festerrors.New(reason)
	}
	return nil
}

func reportInfrastructureRefusal(reason string) {
	fmt.Println(itestenv.RefusalLine(reason))
	fmt.Println(itestenv.RefusalRecovery())
}

func isCodeFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "fest binary not found") ||
		strings.Contains(msg, "failed to build fest binary")
}
