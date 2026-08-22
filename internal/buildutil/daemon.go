package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func prepareIntegrationDaemon(ctx context.Context, opts itestenv.Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	opts.AutoStart = true
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	resolution, err := itestenv.Resolve(ctx, opts)
	if err != nil {
		return festerrors.Wrap(err, "resolve the integration Docker daemon")
	}
	announceResolution(resolution)
	if resolution.DockerHost != "" {
		if err := os.Setenv(itestenv.DockerHostVar, resolution.DockerHost); err != nil {
			return festerrors.Wrapf(err, "publish %s for the integration run", itestenv.DockerHostVar)
		}
	}
	if err := itestenv.PublishRuntimeEnv(resolution); err != nil {
		return festerrors.Wrap(err, "publish the integration testcontainers env")
	}
	return nil
}

func announceResolution(resolution itestenv.Resolution) {
	if resolution.Source == itestenv.SourceFallback {
		fmt.Fprintln(os.Stderr, resolution.Line())
	}
	fmt.Println(resolution.Line())
}

func reportNonRun(err error) {
	fmt.Fprintln(os.Stderr, itestenv.NonRunLine(err.Error()))
	fmt.Fprintln(os.Stderr, itestenv.RefusalRecovery())
}

func doctorStartRequested(args []string) bool {
	seenCommand := false
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		if stringsHasPrefixDash(arg) {
			continue
		}
		if !seenCommand {
			seenCommand = true
			continue
		}
		return arg == "start"
	}
	return false
}

func stringsHasPrefixDash(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}
