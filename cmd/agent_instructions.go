package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"tour/agentinstructions"
)

func init() {
	rootCmd.AddCommand(agentInstructionsCmd)
}

var agentInstructionsCmd = &cobra.Command{
	Use:   "agent-instructions",
	Short: "Print the .tour file format spec, for an agent authoring a tour",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(cmd.OutOrStdout(), agentinstructions.Text)
		return nil
	},
}
