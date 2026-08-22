package itestenv

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	// ProfileName is the Colima profile the integration suite owns. Residents
	// stay on the default profile: the point of a second VM is that nothing
	// else lands in it.
	ProfileName = "fest-itest"

	// ProfileCPUs and ProfileMemoryGiB size the profile at creation. Six CPUs
	// match camp's dedicated daemon so the two suites have the same capacity
	// model when they are not sharing a VM.
	ProfileCPUs      = 6
	ProfileMemoryGiB = 8

	// ProfileDisabled is the FEST_ITEST_PROFILE value that turns isolation off
	// and sends the run at whatever DOCKER_HOST already names. Spelled rather
	// than empty because an unset variable and a variable set to nothing are
	// indistinguishable through os.Getenv.
	ProfileDisabled = "off"

	// EnvProfile overrides the profile name (or disables isolation entirely).
	EnvProfile = "FEST_ITEST_PROFILE"
	// EnvDockerHost hands the suite a daemon directly and skips all Colima
	// handling: the escape hatch for a remote daemon, a native Linux one, or a
	// machine with no Colima at all.
	EnvDockerHost = "FEST_ITEST_DOCKER_HOST"

	// DockerHostVar is Docker's own environment variable, exported so the
	// runner and the suite publish and read the resolved daemon by one name.
	DockerHostVar = "DOCKER_HOST"

	// SocketOverrideEnv tells testcontainers where the daemon socket lives
	// inside the VM. Colima is reached on the host at
	// ~/.colima/<profile>/docker.sock, but containers see /var/run/docker.sock.
	SocketOverrideEnv = "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE"
	InVMDockerSocket  = "/var/run/docker.sock"

	// RyukDisabledEnv stops testcontainers from launching a Ryuk sidecar. The
	// suite owns cleanup; Ryuk's extra Docker traffic is how a wedged daemon
	// used to look like a product failure.
	RyukDisabledEnv   = "TESTCONTAINERS_RYUK_DISABLED"
	RyukDisabledValue = "true"

	// StartCommand is the recovery step named in every message about a profile
	// that is not up.
	StartCommand = "just test daemon-start"

	// DoctorCommand reports the daemon's state without changing it.
	DoctorCommand = "just test integration-doctor"

	colimaHomeDir    = ".colima"
	dockerSocketName = "docker.sock"
	unixScheme       = "unix"

	// defaultProfileName is Colima's own profile, used only as the fallback
	// target when isolation is unavailable.
	defaultProfileName = "default"

	// profileStartTimeout bounds a cold VM boot.
	profileStartTimeout = 10 * time.Minute
)

// Source records how the daemon was chosen, so callers can render one honest
// line instead of inferring intent from a socket path.
type Source string

const (
	// SourceProfile means the run owns a dedicated Colima profile.
	SourceProfile Source = "dedicated"
	// SourceOverride means an operator named the daemon explicitly.
	SourceOverride Source = "override"
	// SourceFallback means isolation was unavailable and the run is sharing a
	// daemon with whatever else is on it.
	SourceFallback Source = "shared"
)

// Resolution is the answer to "which daemon does this run use, and how sure
// are we that it is ours".
type Resolution struct {
	// DockerHost is the value to publish as DOCKER_HOST. Empty means "leave
	// DOCKER_HOST alone and let Docker's own default apply".
	DockerHost string
	// Profile is the Colima profile backing DockerHost, empty when the daemon
	// is not a Colima profile this package manages.
	Profile string
	// Source is how DockerHost was chosen.
	Source Source
	// Started is true when this call booted the profile rather than finding it
	// running.
	Started bool
	// Reason explains a fallback: why isolation could not be used.
	Reason string
}

// Line renders the one-line announcement a run prints before it starts. The
// fallback case is deliberately shouty: a run that quietly lands on the shared
// daemon is exactly the failure mode this package exists to end.
func (r Resolution) Line() string {
	switch r.Source {
	case SourceProfile:
		line := "integration daemon: Colima profile " + r.Profile + " (" + r.DockerHost + ")"
		if r.Started {
			line += " [started for this run]"
		}
		return line
	case SourceOverride:
		return "integration daemon: " + EnvDockerHost + "=" + r.DockerHost
	default:
		host := r.DockerHost
		if host == "" {
			host = "Docker's default socket"
		}
		return "WARNING integration daemon: no dedicated profile, sharing " + host +
			" (" + r.Reason + "). Co-tenant load on that daemon can collapse this run."
	}
}

// Options configures Resolve. The zero value is valid and resolves against the
// real environment; fields exist so the decision table can be tested without a
// Colima on the machine running the tests.
type Options struct {
	// Getenv reads environment variables (defaults to os.Getenv).
	Getenv func(string) string
	// Home is the user's home directory (defaults to userHome).
	Home string
	// Colima talks to the Colima CLI (defaults to the real one).
	Colima Colima
	// AutoStart allows Resolve to boot a stopped or absent profile. Report-only
	// callers (the doctor check) set it false so inspecting the environment
	// cannot change it.
	AutoStart bool
	// Out receives progress lines for slow operations, chiefly a VM boot.
	Out io.Writer
}

func (o Options) normalize() (Options, error) {
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.Home == "" {
		home, err := userHome()
		if err != nil {
			return o, festerrors.Wrap(err, "resolve home directory for the integration daemon")
		}
		o.Home = home
	}
	if o.Colima == nil {
		o.Colima = NewColima()
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	return o, nil
}

func userHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", festerrors.Wrap(err, "cannot determine home directory")
	}
	if strings.TrimSpace(home) == "" {
		return "", festerrors.New("cannot determine home directory: $HOME is not set")
	}
	return home, nil
}

// Resolve decides which Docker daemon the integration suite runs against,
// starting the dedicated profile when AutoStart is set and it is not already
// up. Falling back to a shared daemon is a result, not an error: a machine
// without Colima must still be able to run the suite, as long as it is told.
func Resolve(ctx context.Context, opts Options) (Resolution, error) {
	o, err := opts.normalize()
	if err != nil {
		return Resolution{}, err
	}
	if err := ctx.Err(); err != nil {
		return Resolution{}, err
	}

	if host := strings.TrimSpace(o.Getenv(EnvDockerHost)); host != "" {
		return Resolution{DockerHost: host, Source: SourceOverride}, nil
	}

	profile := ConfiguredProfile(o.Getenv)
	if profile == ProfileDisabled {
		return fallback(o, EnvProfile+"="+ProfileDisabled), nil
	}

	socket := ProfileDockerHost(o.Home, profile)
	ambient := strings.TrimSpace(o.Getenv(DockerHostVar))
	// The dashboard runner resolves first and hands the answer to the test
	// binary through DOCKER_HOST. Recognising it here keeps the child from
	// shelling out to Colima again for an answer it was already given.
	if sameDockerHost(ambient, socket) && socketExists(socket) {
		return Resolution{DockerHost: socket, Profile: profile, Source: SourceProfile}, nil
	}
	// A DOCKER_HOST that is not a Colima socket at all is somebody's decision.
	// Only the Colima case is ours to redirect.
	if ambient != "" && !isColimaSocket(o.Home, ambient) {
		return Resolution{DockerHost: ambient, Source: SourceOverride}, nil
	}

	status, err := o.Colima.Status(ctx, profile)
	if err != nil {
		return fallback(o, "Colima is unavailable: "+err.Error()), nil
	}
	if status.Running {
		return Resolution{DockerHost: socket, Profile: profile, Source: SourceProfile}, nil
	}
	if !o.AutoStart {
		return fallback(o, "profile "+profile+" is "+status.State()+
			" (start it with: "+StartCommand+")"), nil
	}
	if err := startProfile(ctx, o, profile); err != nil {
		return Resolution{}, festerrors.Wrap(err,
			"could not start the dedicated integration daemon")
	}
	return Resolution{DockerHost: socket, Profile: profile, Source: SourceProfile, Started: true}, nil
}

// ConfiguredProfile returns the Colima profile the suite uses, honouring an
// EnvProfile override (including the value that disables isolation).
func ConfiguredProfile(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	if profile := strings.TrimSpace(getenv(EnvProfile)); profile != "" {
		return profile
	}
	return ProfileName
}

func startProfile(ctx context.Context, o Options, profile string) error {
	lockPath, err := profileLockPath(o.Home, profile)
	if err != nil {
		return err
	}
	lock, err := Acquire(ctx, lockPath, LockOptions{
		Wait:  profileStartTimeout,
		Out:   o.Out,
		Label: "Colima profile " + profile,
	})
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	current, err := o.Colima.Status(ctx, profile)
	if err != nil {
		return festerrors.Wrapf(err, "confirm the state of profile %s before starting it", profile)
	}
	if current.Running {
		return nil
	}

	spec := StartSpec{Profile: profile}
	if !current.Exists {
		spec.CPUs = ProfileCPUs
		spec.MemoryGiB = ProfileMemoryGiB
	}
	writeLine(o.Out, "starting Colima profile "+profile+describeSpec(spec)+
		"; creating one from nothing provisions a VM and takes a few minutes")

	startCtx, cancel := context.WithTimeout(ctx, profileStartTimeout)
	defer cancel()
	return o.Colima.Start(startCtx, spec, o.Out)
}

func describeSpec(spec StartSpec) string {
	if spec.CPUs == 0 {
		return ""
	}
	return " (" + strconv.Itoa(spec.CPUs) + " CPUs, " + strconv.Itoa(spec.MemoryGiB) + " GiB)"
}

func fallback(o Options, reason string) Resolution {
	host := strings.TrimSpace(o.Getenv(DockerHostVar))
	if host == "" {
		if candidate := ProfileDockerHost(o.Home, defaultProfileName); socketExists(candidate) {
			host = candidate
		}
	}
	return Resolution{DockerHost: host, Source: SourceFallback, Reason: reason}
}

// ProfileDockerHost returns the DOCKER_HOST URL for a Colima profile's socket.
func ProfileDockerHost(home, profile string) string {
	return unixScheme + "://" + filepath.Join(home, colimaHomeDir, profile, dockerSocketName)
}

// SocketPath returns the filesystem path behind a unix:// Docker host, or ""
// when the host is not a unix socket.
func SocketPath(dockerHost string) string {
	u, err := url.Parse(dockerHost)
	if err != nil || u.Scheme != unixScheme {
		return ""
	}
	return filepath.Clean(u.Path)
}

func isColimaSocket(home, dockerHost string) bool {
	path := SocketPath(dockerHost)
	if path == "" {
		return false
	}
	colimaRoot := filepath.Join(home, colimaHomeDir) + string(filepath.Separator)
	return strings.HasPrefix(path, colimaRoot) && filepath.Base(path) == dockerSocketName
}

// colimaDockerSocket reports whether dockerHost is a Colima Docker socket on
// any home directory. Socket override is about the in-VM path, so another
// user's Colima still needs it; isColimaSocket is the narrower "ours to
// redirect" check.
func colimaDockerSocket(dockerHost string) bool {
	path := SocketPath(dockerHost)
	if path == "" || filepath.Base(path) != dockerSocketName {
		return false
	}
	return strings.Contains(filepath.ToSlash(path), "/"+colimaHomeDir+"/")
}

func (r Resolution) needsInVMSocketOverride() bool {
	return r.Source == SourceProfile || colimaDockerSocket(r.DockerHost)
}

func socketExists(dockerHost string) bool {
	path := SocketPath(dockerHost)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func sameDockerHost(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if pa, pb := SocketPath(a), SocketPath(b); pa != "" && pb != "" {
		return pa == pb
	}
	return a == b
}

func writeLine(out io.Writer, line string) {
	if out == nil {
		return
	}
	_, _ = io.WriteString(out, line+"\n")
}
