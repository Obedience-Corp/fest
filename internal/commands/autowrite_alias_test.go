package commands

import (
	"reflect"
	"testing"
)

func TestNormalizeAutoWriteAlias(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "commit",
			args: []string{"fest", "commit", "-aw", "-m", "msg", "-aw=false"},
			want: []string{"fest", "commit", "--auto-write", "-m", "msg", "-aw=false"},
		},
		{
			name: "commit with global flag",
			args: []string{"fest", "--verbose", "commit", "-aw"},
			want: []string{"fest", "--verbose", "commit", "--auto-write"},
		},
		{
			name: "unrelated command",
			args: []string{"fest", "status", "-aw"},
			want: []string{"fest", "status", "-aw"},
		},
		{
			name: "plugin style command",
			args: []string{"fest", "external-tool", "-aw"},
			want: []string{"fest", "external-tool", "-aw"},
		},
		{
			name: "argument terminator",
			args: []string{"fest", "commit", "--", "-aw"},
			want: []string{"fest", "commit", "--", "-aw"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeAutoWriteAlias(tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("normalizeAutoWriteAlias() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
