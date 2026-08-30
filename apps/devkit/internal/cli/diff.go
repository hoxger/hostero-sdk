package cli

import "github.com/spf13/cobra"

func newDiffCommand() *cobra.Command {
	return unavailableCommand("diff", "Compare the pinned contract with a newer release.")
}
