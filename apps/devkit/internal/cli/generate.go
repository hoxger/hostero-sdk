package cli

import "github.com/spf13/cobra"

func newGenerateCommand() *cobra.Command {
	return unavailableCommand("generate", "Generate SDK source from a pinned API contract.")
}
