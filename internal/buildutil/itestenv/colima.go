package itestenv

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	colimaBinary   = "colima"
	colimaRunning  = "running"
	stateAbsent    = "absent"
	stateStopped   = "stopped"
	activateFalse  = "--activate=false"
	cpusFlag       = "--cpus"
	memoryFlag     = "--memory"
	jsonOutputFlag = "--json"
)

// ProfileStatus is what Colima reports about one profile.
type ProfileStatus struct {
	Name        string
	Exists      bool
	Running     bool
	CPUs        int
	MemoryBytes int64
	Status      string
}

// State renders the status for a human sentence: a profile that was never
// created and one that is merely stopped need different recovery steps.
func (s ProfileStatus) State() string {
	switch {
	case !s.Exists:
		return stateAbsent
	case s.Running:
		return colimaRunning
	case s.Status != "":
		return strings.ToLower(s.Status)
	default:
		return stateStopped
	}
}

// StartSpec describes a profile start. Zero CPUs and memory mean "start the
// profile with whatever configuration it already has".
type StartSpec struct {
	Profile   string
	CPUs      int
	MemoryGiB int
}

// Colima is the slice of the Colima CLI this package needs.
type Colima interface {
	Status(ctx context.Context, profile string) (ProfileStatus, error)
	Start(ctx context.Context, spec StartSpec, out io.Writer) error
}

type commandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, out io.Writer, name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, festerrors.Wrapf(err, "%s %s: %s", name,
			strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (execRunner) Run(ctx context.Context, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return festerrors.Wrapf(err, "%s %s", name, strings.Join(args, " "))
	}
	return nil
}

type colimaCLI struct {
	run commandRunner
}

// NewColima returns an adapter over the installed Colima CLI.
func NewColima() Colima { return colimaCLI{run: execRunner{}} }

func newColimaWith(runner commandRunner) Colima { return colimaCLI{run: runner} }

func (c colimaCLI) Status(ctx context.Context, profile string) (ProfileStatus, error) {
	if err := ctx.Err(); err != nil {
		return ProfileStatus{Name: profile}, err
	}
	if _, err := exec.LookPath(colimaBinary); err != nil {
		return ProfileStatus{Name: profile}, festerrors.Wrap(err, "find the colima CLI")
	}
	out, err := c.run.Output(ctx, colimaBinary, "list", jsonOutputFlag)
	if err != nil {
		return ProfileStatus{Name: profile}, festerrors.Wrap(err, "list Colima profiles")
	}
	return parseColimaList(out, profile), nil
}

func (c colimaCLI) Start(ctx context.Context, spec StartSpec, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := []string{"start", spec.Profile, activateFalse}
	if spec.CPUs > 0 {
		args = append(args, cpusFlag, strconv.Itoa(spec.CPUs))
	}
	if spec.MemoryGiB > 0 {
		args = append(args, memoryFlag, strconv.Itoa(spec.MemoryGiB))
	}
	if err := c.run.Run(ctx, out, colimaBinary, args...); err != nil {
		return festerrors.Wrapf(err, "start Colima profile %s", spec.Profile)
	}
	return nil
}

func parseColimaList(data []byte, profile string) ProfileStatus {
	status := ProfileStatus{Name: profile}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			CPUs   int    `json:"cpus"`
			Memory int64  `json:"memory"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Name != profile {
			continue
		}
		status.Exists = true
		status.Status = entry.Status
		status.Running = strings.EqualFold(entry.Status, colimaRunning)
		status.CPUs = entry.CPUs
		status.MemoryBytes = entry.Memory
		return status
	}
	return status
}
