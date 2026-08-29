package cli

import "github.com/spf13/cobra"

func newInitCommand() *cobra.Command {
	return unavailableCommand("init", "Create a DevKit project configuration.")
}
