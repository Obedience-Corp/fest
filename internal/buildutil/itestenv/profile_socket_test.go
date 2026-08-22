package itestenv

import (
	"path/filepath"
	"testing"
)

func TestSocketPathAndSameDockerHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a, b     string
		wantSame bool
		wantPath string
	}{
		{name: "identical unix hosts", a: "unix:///a/docker.sock", b: "unix:///a/docker.sock", wantSame: true, wantPath: "/a/docker.sock"},
		{name: "redundant separators normalize", a: "unix:///a//docker.sock", b: "unix:///a/docker.sock", wantSame: true, wantPath: "/a/docker.sock"},
		{name: "different profiles differ", a: "unix:///a/default/docker.sock", b: "unix:///a/fest-itest/docker.sock", wantPath: "/a/default/docker.sock"},
		{name: "tcp compares literally", a: "tcp://host:2375", b: "tcp://host:2375", wantSame: true},
		{name: "empty is never the same", a: "", b: "unix:///a/docker.sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sameDockerHost(tt.a, tt.b); got != tt.wantSame {
				t.Errorf("sameDockerHost(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.wantSame)
			}
			if tt.wantPath != "" {
				if got := SocketPath(tt.a); got != tt.wantPath {
					t.Errorf("SocketPath(%q) = %q, want %q", tt.a, got, tt.wantPath)
				}
			}
		})
	}
}

func TestIsColimaSocket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dockerHost string
		want       bool
	}{
		{name: "default profile", dockerHost: "unix:///home/u/.colima/default/docker.sock", want: true},
		{name: "fest dedicated profile", dockerHost: "unix:///home/u/.colima/fest-itest/docker.sock", want: true},
		{name: "camp dedicated profile is still Colima", dockerHost: "unix:///home/u/.colima/camp-itest/docker.sock", want: true},
		{name: "legacy top level socket", dockerHost: "unix:///home/u/.colima/docker.sock", want: true},
		{name: "another user's colima", dockerHost: "unix:///home/other/.colima/default/docker.sock"},
		{name: "native Docker", dockerHost: "unix:///var/run/docker.sock"},
		{name: "Docker Desktop", dockerHost: "unix:///home/u/.docker/run/docker.sock"},
		{name: "remote daemon", dockerHost: "tcp://remote:2375"},
		{name: "not a docker socket", dockerHost: "unix:///home/u/.colima/default/containerd.sock"},
		{name: "unset", dockerHost: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isColimaSocket("/home/u", tt.dockerHost); got != tt.want {
				t.Errorf("isColimaSocket(%q) = %v, want %v", tt.dockerHost, got, tt.want)
			}
		})
	}
}

func TestProfileDockerHost(t *testing.T) {
	t.Parallel()

	got := ProfileDockerHost("/home/u", ProfileName)
	want := "unix://" + filepath.Join("/home/u", ".colima", ProfileName, "docker.sock")
	if got != want {
		t.Fatalf("ProfileDockerHost() = %q, want %q", got, want)
	}
}

func TestNeedsInVMSocketOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    Resolution
		want bool
	}{
		{
			name: "dedicated Colima profile",
			r: Resolution{
				DockerHost: "unix:///home/u/.colima/fest-itest/docker.sock",
				Profile:    ProfileName,
				Source:     SourceProfile,
			},
			want: true,
		},
		{
			name: "SourceProfile even if the host looks native",
			r:    Resolution{DockerHost: "unix:///var/run/docker.sock", Source: SourceProfile},
			want: true,
		},
		{
			name: "fallback Colima default",
			r: Resolution{
				DockerHost: "unix:///home/u/.colima/default/docker.sock",
				Source:     SourceFallback,
			},
			want: true,
		},
		{
			name: "another user's Colima still needs the in-VM path",
			r: Resolution{
				DockerHost: "unix:///home/other/.colima/default/docker.sock",
				Source:     SourceOverride,
			},
			want: true,
		},
		{name: "remote override", r: Resolution{DockerHost: "tcp://remote:2375", Source: SourceOverride}},
		{name: "native Docker", r: Resolution{DockerHost: "unix:///var/run/docker.sock", Source: SourceOverride}},
		{name: "empty fallback", r: Resolution{Source: SourceFallback}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.r.needsInVMSocketOverride(); got != tt.want {
				t.Errorf("needsInVMSocketOverride(%+v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
