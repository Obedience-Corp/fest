package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Obedience-Corp/obey-shared/buildutil"
)

func main() {
	args := os.Args[1:]
	if err := configureIntegrationEnvironment(args); err != nil {
		fmt.Fprintf(os.Stderr, "configure integration environment: %v\n", err)
		os.Exit(1)
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

func configureIntegrationEnvironment(args []string) error {
	switch requestedCommand(args) {
	case "integration", "all":
	default:
		return nil
	}

	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return err
	}

	if strings.TrimSpace(os.Getenv("DOCKER_HOST")) != "" {
		return nil
	}

	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return nil
	}

	colimaSocket := filepath.Join(home, ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err != nil {
		return nil
	}

	return os.Setenv("DOCKER_HOST", "unix://"+colimaSocket)
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

	return cmdOutputOr("dev", "git", "describe", "--tags", "--exact-match", "HEAD")
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
