package cli

import "github.com/spf13/cobra"

func newUpdateCommand() *cobra.Command {
	return unavailableCommand("update", "Resolve and pin a newer Hostero API contract.")
}
