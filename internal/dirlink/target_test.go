package dirlink

import "testing"

func TestIsUNCPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\server\share\config`, true},
		{`\\?\UNC\server\share`, true},
		{`C:\local\config`, false},
		{`/unix/style`, false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isUNCPath(tt.path); got != tt.want {
			t.Errorf("isUNCPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestClassifyAbsTarget(t *testing.T) {
	if classifyAbsTarget(`C:\fest\config`) != targetLocal {
		t.Fatal("expected local drive path to be local")
	}
	if classifyAbsTarget(`\\fileserver\fest\config`) != targetNetwork {
		t.Fatal("expected UNC path to be network")
	}
}
