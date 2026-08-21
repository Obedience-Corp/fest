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

Examples:
  fest version           Show full version info
  fest version --short   Show only version number
  fest version --json    Output as JSON`,
	Annotations: map[string]string{
		"scope": "global",
	},
	Run: func(cmd *cobra.Command, args []string) {
		short, _ := cmd.Flags().GetBool("short")
		jsonOut, _ := cmd.Flags().GetBool("json")

		info := version.Get()
		out := cmd.OutOrStdout()

		if short {
			_, _ = fmt.Fprintln(out, info.Version)
			return
		}

		if jsonOut {
			encoded, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				return
			}
			_, _ = fmt.Fprintln(out, string(encoded))
			return
		}

		writeVersionInfo(out, info)
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
	rootCmd.AddCommand(versionCmd)
	versionCmd.GroupID = "system"
}
