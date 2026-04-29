package progress

import "context"

// Gate enforces lifecycle rules on task mutations.
//
// The interface lives in the progress package to keep progress free of
// any dependency on the lifecycle package, while still letting Manager
// consult the gate inside its mutation methods. Implementations live in
// internal/lifecycle/.
type Gate interface {
	// EnforceForTask returns a non-nil error if the task's festival
	// status forbids the mutation. A nil return allows the mutation
	// to proceed.
	EnforceForTask(ctx context.Context, taskID string) error
}

// NoopGate is a Gate that always allows. It is the default when a
// Manager is constructed via NewManager (i.e. without explicit
// enforcement). CLI mutation entry points use NewManagerWithGate to
// install a real gate.
type NoopGate struct{}

// EnforceForTask always returns nil.
func (NoopGate) EnforceForTask(_ context.Context, _ string) error { return nil }
