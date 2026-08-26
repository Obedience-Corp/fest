package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Obedience-Corp/build-util/ui"
	"github.com/Obedience-Corp/fest/internal/buildutil/itestenv"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	bytesPerGiB     = 1 << 30
	dockerPSTimeout = 15 * time.Second
	namesShown      = 2
)

func integrationDoctor(ctx context.Context, start bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	began := time.Now()
	ui.Section("Integration Daemon Doctor")

	resolution, err := itestenv.Resolve(ctx, itestenv.Options{AutoStart: start, Out: os.Stdout})
	if err != nil {
		return festerrors.Wrap(err, "resolve the integration Docker daemon")
	}

	if resolution.Source == itestenv.SourceFallback {
		ui.Warning(resolution.Line())
	} else {
		fmt.Printf("  %s\n", resolution.Line())
	}

	probe := itestenv.Probe(ctx, resolution.DockerHost)
	degraded, reason := probe.Degraded()

	rows := [][]string{
		{"Check", "Result"},
		{"Profile", profileRow(ctx)},
		{"Docker host", hostRow(resolution)},
		{"Daemon probe", probeRow(probe)},
		{"Containers", containersRow(ctx, resolution)},
		{"Suite lock", lockRow(resolution)},
	}
	ui.SummaryCardWithStatus("Integration Daemon", rows,
		fmt.Sprintf("%.2fs", time.Since(began).Seconds()), !degraded,
		"✓ DAEMON READY", "✗ DAEMON NOT USABLE")

	if degraded {
		return festerrors.New("the integration daemon is not usable: " + reason)
	}
	if resolution.Source == itestenv.SourceFallback {
		ui.Warning("no dedicated daemon: run '" + itestenv.StartCommand +
			"' to create or boot the " + itestenv.ProfileName + " profile")
	}
	return nil
}

func hostRow(resolution itestenv.Resolution) string {
	host := shortHome(resolution.DockerHost)
	if host == "" {
		host = "Docker's default socket"
	}
	if resolution.Source == itestenv.SourceFallback {
		return "shared: " + host
	}
	return host
}

func shortHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

func probeRow(probe itestenv.ProbeResult) string {
	row := strings.TrimPrefix(probe.Line(), "daemon probe: ")
	if host := probe.DockerHost; host != "" {
		row = strings.TrimPrefix(row, host+" ")
		row = strings.TrimPrefix(row, "Docker's default socket ")
	}
	if degraded, reason := probe.Degraded(); degraded {
		return row + " - " + reason
	}
	return row
}

func profileRow(ctx context.Context) string {
	profile := itestenv.ConfiguredProfile(nil)
	if profile == itestenv.ProfileDisabled {
		return "isolation disabled by " + itestenv.EnvProfile
	}
	status, err := itestenv.NewColima().Status(ctx, profile)
	if err != nil {
		return profile + ": Colima unavailable (" + err.Error() + ")"
	}
	row := profile + ": " + status.State()
	if status.Exists {
		row += ", " + strconv.Itoa(status.CPUs) + " CPUs, " +
			strconv.FormatFloat(float64(status.MemoryBytes)/bytesPerGiB, 'f', 1, 64) + " GiB"
	}
	if !status.Running {
		row += " (fix: just test daemon-start)"
	}
	return row
}

func containersRow(ctx context.Context, resolution itestenv.Resolution) string {
	ctx, cancel := context.WithTimeout(ctx, dockerPSTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}")
	cmd.Env = os.Environ()
	if resolution.DockerHost != "" {
		cmd.Env = append(cmd.Env, itestenv.DockerHostVar+"="+resolution.DockerHost)
	}
	out, err := cmd.Output()
	if err != nil {
		return "could not list containers: " + err.Error()
	}
	names := strings.Fields(strings.TrimSpace(string(out)))
	if len(names) == 0 {
		return "none running"
	}
	row := strconv.Itoa(len(names)) + " running: " + strings.Join(names[:min(len(names), namesShown)], ", ")
	if len(names) > namesShown {
		row += ", +" + strconv.Itoa(len(names)-namesShown) + " more"
	}
	return row
}

func lockRow(resolution itestenv.Resolution) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "unknown: " + err.Error()
	}
	path := itestenv.SuiteLockPath(home, resolution.DockerHost)
	_, description := itestenv.LockStatus(path)
	return description + " (" + shortHome(path) + ")"
}
