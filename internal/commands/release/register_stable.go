//go:build !dev

package release

import "github.com/spf13/cobra"

func registerDev(_ *cobra.Command) {}
