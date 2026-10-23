package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Show fest version, build information, and runtime details.

With --check, compare this build's Festival suite release against the latest
stable release on github.com/Obedience-Corp/festival and print how to upgrade.
The result is cached for 24 hours in ~/.obey/update-check/suite.json, shared
with camp. fest also mentions a newer release on stderr after human commands
such as fest status and fest go, once per release; set
FESTIVAL_NO_UPDATE_CHECK=1 to turn that off. fest never updates itself.
--check takes precedence over --short.

Examples:
  fest version                  Show full version info
  fest version --short          Show only version number
  fest version --json           Output as JSON
  fest version --check          Check for a newer Festival suite release
  fest version --check --json   Output the check result as JSON`,
	Annotations: map[string]string{
		"scope": "global",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		short, _ := cmd.Flags().GetBool("short")
		jsonOut, _ := cmd.Flags().GetBool("json")
		check, _ := cmd.Flags().GetBool("check")

		info := version.Get()
		out := cmd.OutOrStdout()

		if check {
			rep, err := suiteVersionCheck(cmd.Context())
			if err != nil {
				return err
			}
			return emitVersionCheck(out, rep, jsonOut)
		}

		if short {
			_, _ = fmt.Fprintln(out, info.Version)
			return nil
		}

		if jsonOut {
			encoded, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				return nil
			}
			_, _ = fmt.Fprintln(out, string(encoded))
			return nil
		}

		writeVersionInfo(out, info)
		return nil
	},
}

// writeVersionInfo renders the human-readable version report. The bundle line
// appears only for binaries stamped by a festival release build.
func writeVersionInfo(w io.Writer, info version.Info) {
	_, _ = fmt.Fprintf(w, "fest %s\n", info.Version)
	if info.Bundle != "" {
		_, _ = fmt.Fprintf(w, "bundle: festival %s\n", info.Bundle)
	}
	_, _ = fmt.Fprintf(w, "commit: %s\n", info.Commit)
	_, _ = fmt.Fprintf(w, "built: %s\n", info.BuildDate)
	_, _ = fmt.Fprintf(w, "go: %s\n", info.GoVersion)
	_, _ = fmt.Fprintf(w, "platform: %s\n", info.Platform)
	_, _ = fmt.Fprintf(w, "profile: %s\n", info.Profile)
}

func init() {
	versionCmd.Flags().BoolP("short", "s", false, "show only version number")
	versionCmd.Flags().Bool("json", false, "output as JSON")
	versionCmd.Flags().Bool("check", false, "check for a newer Festival suite release")
	rootCmd.AddCommand(versionCmd)
	versionCmd.GroupID = "system"
}
