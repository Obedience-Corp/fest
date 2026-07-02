package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCreateScopeFlagHints(t *testing.T) {
	create := &cobra.Command{Use: "create"}
	seq := &cobra.Command{
		Use:  "sequence",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	seq.Flags().String("name", "", "")
	create.AddCommand(seq)
	setCreateScopeFlagHints(create)

	tests := []struct {
		name     string
		args     []string
		wantHint string
	}{
		{"phase flag teaches cwd model", []string{"sequence", "--phase", "003_IMPLEMENT"}, "cd 003_IMPLEMENT first"},
		{"festival flag teaches navigation", []string{"sequence", "--festival", "x"}, "fest go"},
		{"unrelated unknown flag stays bare", []string{"sequence", "--bogus"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			create.SetArgs(tt.args)
			seq.SilenceErrors = true
			seq.SilenceUsage = true
			err := create.Execute()
			if err == nil {
				t.Fatal("expected an unknown-flag error")
			}
			if tt.wantHint == "" {
				if strings.Contains(err.Error(), "Hint:") {
					t.Fatalf("unrelated flag should not get a scope hint, got: %v", err)
				}
				return
			}
			if !strings.Contains(err.Error(), tt.wantHint) {
				t.Fatalf("error missing hint %q, got: %v", tt.wantHint, err)
			}
		})
	}
}
