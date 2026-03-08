package release

import "github.com/spf13/cobra"

const (
	AnnotationReleaseChannel = "release_channel"
	ReleaseChannelDevOnly    = "dev_only"
)

// MarkDevOnly annotates a command as dev-only for tooling visibility.
// This is descriptive metadata — build tags control actual inclusion.
func MarkDevOnly(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[AnnotationReleaseChannel] = ReleaseChannelDevOnly
}
