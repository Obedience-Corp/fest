package itestenv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func TestProbeResultDegraded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		result       ProbeResult
		wantDegraded bool
		wantReason   string
	}{
		{
			name:         "no answer at all",
			result:       ProbeResult{DockerHost: "unix:///d.sock", Err: festerrors.New("connection refused")},
			wantDegraded: true,
			wantReason:   "did not answer",
		},
		{
			name: "answers then stops",
			result: ProbeResult{
				DockerHost: "unix:///d.sock",
				Samples:    []time.Duration{2 * time.Millisecond, 3 * time.Millisecond},
				Err:        festerrors.New("EOF"),
			},
			wantDegraded: true,
			wantReason:   "stopped answering after 2 round trips",
		},
		{
			name: "slower than the collapse threshold",
			result: ProbeResult{
				DockerHost: "unix:///d.sock",
				Samples:    []time.Duration{degradedMedian + time.Millisecond},
				Median:     degradedMedian + time.Millisecond,
			},
			wantDegraded: true,
			wantReason:   "out of headroom",
		},
		{
			name: "exactly at the threshold still runs",
			result: ProbeResult{
				Samples: []time.Duration{degradedMedian},
				Median:  degradedMedian,
			},
		},
		{
			name: "a healthy daemon is not degraded",
			result: ProbeResult{
				Samples: []time.Duration{2 * time.Millisecond, 3 * time.Millisecond, 2 * time.Millisecond},
				Median:  2 * time.Millisecond,
			},
		},
		{
			name:       "an unmeasurable transport never blocks a run",
			result:     ProbeResult{DockerHost: "ssh://host", Err: festerrors.Wrap(errProbeUnsupported, `scheme "ssh"`)},
			wantReason: "not measured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			degraded, reason := tt.result.Degraded()
			if degraded != tt.wantDegraded {
				t.Errorf("Degraded() = %v (%q), want %v", degraded, reason, tt.wantDegraded)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Errorf("Degraded() reason = %q, want it to contain %q", reason, tt.wantReason)
			}
			if tt.wantReason == "" && reason != "" {
				t.Errorf("Degraded() reason = %q, want it empty", reason)
			}
		})
	}
}

func TestProbeResultLine(t *testing.T) {
	t.Parallel()

	measured := ProbeResult{
		DockerHost: "unix:///d.sock",
		Samples:    []time.Duration{2 * time.Millisecond},
		Median:     2 * time.Millisecond,
	}
	if line := measured.Line(); !strings.Contains(line, "median 2ms") || !strings.Contains(line, "1 round trip") {
		t.Errorf("Line() = %q, want the median and the sample count", line)
	}

	failed := ProbeResult{Err: festerrors.New("connection refused")}
	if line := failed.Line(); !strings.Contains(line, "connection refused") ||
		!strings.Contains(line, "Docker's default socket") {
		t.Errorf("Line() = %q, want the failure and the daemon it applies to", line)
	}
}

func TestPingClientTransports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dockerHost   string
		wantEndpoint string
		wantErr      bool
	}{
		{name: "unix socket", dockerHost: "unix:///var/run/docker.sock", wantEndpoint: dockerAPIURL},
		{name: "empty falls back to Docker's default", wantEndpoint: dockerAPIURL},
		{name: "tcp daemon", dockerHost: "tcp://remote:2375", wantEndpoint: "http://remote:2375/_ping"},
		{name: "ssh is unmeasurable", dockerHost: "ssh://host", wantErr: true},
		{name: "tls is unmeasurable", dockerHost: "https://remote:2376", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, endpoint, err := pingClient(tt.dockerHost)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("pingClient(%q) returned no error", tt.dockerHost)
				}
				if !errors.Is(err, errProbeUnsupported) {
					t.Fatalf("pingClient(%q) error = %v, want an unsupported-transport error", tt.dockerHost, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("pingClient(%q) error = %v", tt.dockerHost, err)
			}
			if client == nil {
				t.Fatalf("pingClient(%q) returned no client", tt.dockerHost)
			}
			if endpoint != tt.wantEndpoint {
				t.Errorf("endpoint = %q, want %q", endpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestPingAgainstStubDaemon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		status  int
		wantErr bool
	}{
		{name: "healthy daemon", body: pingOKBody, status: http.StatusOK},
		{name: "unhealthy body", body: "NOT OK", status: http.StatusOK, wantErr: true},
		{name: "server error", body: pingOKBody, status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			socket := filepath.Join(shortTempDir(t), "d.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatalf("listen on %s: %v", socket, err)
			}
			server := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
				}),
				ReadHeaderTimeout: time.Second,
			}
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() {
				_ = server.Close()
				_ = os.Remove(socket)
			})

			err = Ping(context.Background(), "unix://"+socket)
			if tt.wantErr && err == nil {
				t.Fatal("Ping() reported a healthy daemon")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Ping() error = %v", err)
			}
		})
	}
}

func TestPingHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Ping(ctx, "unix:///nonexistent/docker.sock")
	if err == nil {
		t.Fatal("Ping() with a cancelled context returned no error")
	}
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "fest-itest-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestProbeStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()

	result := Probe(context.Background(), "unix://"+filepath.Join(t.TempDir(), "absent.sock"))
	if result.Err == nil {
		t.Fatal("Probe() against a missing socket reported success")
	}
	if len(result.Samples) != 0 {
		t.Errorf("samples = %d, want 0", len(result.Samples))
	}
	if degraded, reason := result.Degraded(); !degraded {
		t.Errorf("Degraded() = false (%q), want true", reason)
	}
}
