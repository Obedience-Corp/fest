package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
	"github.com/Obedience-Corp/obey-shared/buildutil"
	"github.com/Obedience-Corp/obey-shared/buildutil/ui"
)

func main() {
	args := os.Args[1:]
	ctx := context.Background()

	switch requestedCommand(args) {
	case "integration-doctor":
		ui.Init(false)
		if err := integrationDoctor(ctx, doctorStartRequested(args)); err != nil {
			reportNonRun(err)
			os.Exit(1)
		}
		return
	case "integration", "all":
		if err := prepareIntegrationDaemon(ctx, itestenv.Options{}); err != nil {
			reportNonRun(err)
			os.Exit(1)
		}
	}

	buildutil.Run(args, buildutil.BuildConfig{
		BinaryName:  "fest",
		MainPath:    "./cmd/fest",
		SectionName: "Fest CLI",
		LDFlags:     ldflags,
		IntegrationBuildEnv: func() []string {
			return []string{
				"GOOS=linux",
				"GOARCH=" + runtime.GOARCH,
			}
		},
	})
}

func requestedCommand(args []string) string {
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}

	return ""
}

func ldflags() string {
	versionPkg := "github.com/Obedience-Corp/fest/internal/version"
	version := resolvedVersion()
	commit := cmdOutputOr("unknown", "git", "rev-parse", "--short", "HEAD")
	date := time.Now().UTC().Format(time.RFC3339)

	parts := []string{
		fmt.Sprintf("-X %s.Version=%s", versionPkg, version),
		fmt.Sprintf("-X %s.Commit=%s", versionPkg, commit),
		fmt.Sprintf("-X %s.BuildDate=%s", versionPkg, date),
	}
	return strings.Join(parts, " ")
}

func resolvedVersion() string {
	if version := strings.TrimSpace(os.Getenv("VERSION")); version != "" {
		return version
	}

	return cmdOutputOr("dev", "git", "describe", "--tags", "--always", "--dirty")
}

func cmdOutputOr(fallback, name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return fallback
	}

	value := strings.TrimSpace(string(out))
	if value == "" {
		return fallback
	}

	return value
}
