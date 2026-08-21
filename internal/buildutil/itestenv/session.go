package itestenv

import (
	"context"
	"os"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

// Suite is a run's claim on a Docker daemon: which one, and the lock that
// says this run has it.
type Suite struct {
	Resolution Resolution
	lock       *Lock
}

// OpenSuite resolves the daemon, publishes DOCKER_HOST, and takes the suite
// lock. The caller must Close it so the next run is not waiting on a process
// that has already exited.
func OpenSuite(ctx context.Context, opts Options) (*Suite, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	resolution, err := Resolve(ctx, o)
	if err != nil {
		return nil, err
	}
	if resolution.DockerHost != "" {
		if err := os.Setenv(DockerHostVar, resolution.DockerHost); err != nil {
			return nil, festerrors.Wrapf(err, "publish %s", DockerHostVar)
		}
	}
	lock, err := Acquire(ctx, SuiteLockPath(o.Home, resolution.DockerHost), LockOptions{
		Wait:  LockWait(o.Getenv),
		Out:   o.Out,
		Label: "fest integration suite on " + daemonLabel(resolution),
	})
	if err != nil {
		return nil, err
	}
	return &Suite{Resolution: resolution, lock: lock}, nil
}

// Close drops the suite lock. Failing to release is reported by the caller;
// the kernel releases the flock when this process exits either way.
func (s *Suite) Close() error {
	if s == nil || s.lock == nil {
		return nil
	}
	err := s.lock.Release()
	s.lock = nil
	return err
}

func daemonLabel(resolution Resolution) string {
	if resolution.Profile != "" {
		return resolution.Profile
	}
	if resolution.DockerHost != "" {
		return resolution.DockerHost
	}
	return "the default Docker daemon"
}

// RefusalLine is the banner the dashboard and a human reading raw output
// both match on: the run did not happen, and this is not a test failure.
func RefusalLine(reason string) string {
	return "INFRASTRUCTURE FAILURE (not a test failure): " + reason
}

// RefusalRecovery is the one next-step sentence that follows a refusal.
func RefusalRecovery() string {
	return "The integration run did not happen: no test executed. " +
		"Repair the daemon with '" + StartCommand + "', inspect it with '" +
		DoctorCommand + "', or set " + EnvDockerHost +
		" to point the suite somewhere else."
}

// NonRunLine is the dashboard verdict for a run that never started.
func NonRunLine(reason string) string {
	return "✗ NON-RUN  integration run did not happen: " + reason
}
