package itestenv

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	probeSamples     = 5
	probePingTimeout = 2 * time.Second
	probeBudget      = 15 * time.Second
	degradedMedian   = 500 * time.Millisecond

	defaultDockerSocket = "/var/run/docker.sock"

	pingPath     = "/_ping"
	pingOKBody   = "OK"
	tcpScheme    = "tcp"
	httpScheme   = "http"
	dockerAPIURL = "http://docker" + pingPath
)

var errProbeUnsupported = festerrors.New("Docker transport cannot be probed")

// ProbeResult is one preflight measurement of the daemon's responsiveness.
type ProbeResult struct {
	DockerHost string
	Samples    []time.Duration
	Median     time.Duration
	Err        error
}

// Probe measures a handful of Docker API round trips before a run commits to
// creating containers.
func Probe(ctx context.Context, dockerHost string) ProbeResult {
	result := ProbeResult{DockerHost: dockerHost}
	ctx, cancel := context.WithTimeout(ctx, probeBudget)
	defer cancel()

	for range probeSamples {
		start := time.Now()
		if err := Ping(ctx, dockerHost); err != nil {
			result.Err = err
			break
		}
		result.Samples = append(result.Samples, time.Since(start))
	}
	if len(result.Samples) > 0 {
		sorted := slices.Clone(result.Samples)
		slices.Sort(sorted)
		result.Median = sorted[len(sorted)/2]
	}
	return result
}

// Degraded reports whether a run should refuse to start, and why.
func (r ProbeResult) Degraded() (bool, string) {
	switch {
	case errors.Is(r.Err, errProbeUnsupported):
		return false, "daemon responsiveness not measured: " + r.Err.Error()
	case r.Err != nil && len(r.Samples) == 0:
		return true, "the Docker daemon at " + r.hostLabel() + " did not answer: " + r.Err.Error()
	case r.Err != nil:
		return true, "the Docker daemon at " + r.hostLabel() + " stopped answering after " +
			itoaSamples(len(r.Samples)) + ": " + r.Err.Error()
	case r.Median > degradedMedian:
		return true, "the Docker daemon at " + r.hostLabel() + " is answering in " +
			r.Median.Round(time.Millisecond).String() + " (healthy is single-digit ms); " +
			"it is out of headroom before this run even starts"
	default:
		return false, ""
	}
}

// Line renders the probe for a log or a doctor report.
func (r ProbeResult) Line() string {
	if len(r.Samples) == 0 {
		reason := "no response"
		if r.Err != nil {
			reason = r.Err.Error()
		}
		return "daemon probe: " + r.hostLabel() + " " + reason
	}
	return "daemon probe: " + r.hostLabel() + " median " +
		r.Median.Round(time.Microsecond).String() + " over " +
		itoaSamples(len(r.Samples))
}

func (r ProbeResult) hostLabel() string {
	if strings.TrimSpace(r.DockerHost) == "" {
		return "Docker's default socket"
	}
	return r.DockerHost
}

func itoaSamples(n int) string {
	if n == 1 {
		return "1 round trip"
	}
	return strconv.Itoa(n) + " round trips"
}

// Ping performs one Docker API round trip against dockerHost.
func Ping(ctx context.Context, dockerHost string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, endpoint, err := pingClient(dockerHost)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, probePingTimeout)
		defer cancel()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return festerrors.Wrap(err, "build Docker ping request")
	}
	response, err := client.Do(request)
	if err != nil {
		return festerrors.Wrap(err, "ping the Docker daemon")
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return festerrors.Wrap(err, "read the Docker ping response")
	}
	if response.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != pingOKBody {
		return festerrors.New("Docker ping returned " + response.Status + ": " +
			strings.TrimSpace(string(body)))
	}
	return nil
}

func pingClient(dockerHost string) (*http.Client, string, error) {
	host := strings.TrimSpace(dockerHost)
	if host == "" {
		host = unixScheme + "://" + defaultDockerSocket
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, "", festerrors.Wrapf(err, "parse Docker host %q", dockerHost)
	}
	switch u.Scheme {
	case unixScheme:
		socket := u.Path
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, unixScheme, socket)
			},
		}
		return &http.Client{Transport: transport}, dockerAPIURL, nil
	case tcpScheme, httpScheme:
		return &http.Client{Transport: &http.Transport{}},
			httpScheme + "://" + u.Host + pingPath, nil
	default:
		return nil, "", festerrors.Wrapf(errProbeUnsupported, "scheme %q", u.Scheme)
	}
}
