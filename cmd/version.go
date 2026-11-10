package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is a hardcoded constant, not build-info/ldflags-derived, per
// implementation/CLI.md's Version reporting section: there's no
// release/tagging pipeline yet for either of those to reflect, and a
// constant needs no toolchain support to print exactly what it's set to.
// Bump by hand alongside each release.
const version = "0.1.2"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print tour's version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), version)
		return nil
	},
}
