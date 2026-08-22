// Package itestenv owns the environment the integration suite runs against:
// which Docker daemon it uses, who is allowed to use it right now, and whether
// that daemon is healthy enough to start a run at all.
//
// Fest adopts camp's itestenv shape (WI-a56292 Q4) rather than a parallel
// design. The suite's capacity model assumed an idle, exclusively owned daemon
// while the daemon it actually used was Colima's default profile: the machine's
// general-purpose one. That is the same co-tenancy that collapsed camp's suite.
//
// Three pieces answer that, in the order a run needs them:
//
//  1. Resolve picks a dedicated Colima profile (fest-itest) and starts it on
//     demand, so co-tenant load cannot reach the suite. When there is no Colima
//     to isolate with, it falls back to the shared daemon and says so loudly
//     rather than pretending.
//  2. Acquire takes a machine-wide lock keyed by the resolved daemon, so two
//     suites pointed at the same VM serialize instead of collapsing each other.
//     The lock filename prefix is shared with camp (camp-itest-) so a camp run
//     and a fest run targeting one daemon wait on each other.
//  3. Probe measures a daemon round trip before any container is created, so a
//     daemon that is already wedged costs five seconds instead of a wedged run.
//
// The package is shared by the dashboard runner (internal/buildutil) and by
// the suite's own TestMain, so both lanes resolve, lock, and probe the same
// way. Nothing here is product code: it configures how fest is tested.
package itestenv
