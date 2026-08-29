package cli

import "github.com/spf13/cobra"

func newValidateCommand() *cobra.Command {
	return unavailableCommand("validate", "Validate DevKit configuration and generated output.")
}
