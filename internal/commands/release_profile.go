package commands

import "github.com/spf13/cobra"

const (
	annotationReleaseChannel = "release_channel"
	releaseChannelDevOnly    = "dev_only"
)

func markDevOnlyCommand(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationReleaseChannel] = releaseChannelDevOnly
}
