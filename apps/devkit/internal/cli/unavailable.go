package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func unavailableCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s is not available yet", cmd.CommandPath())
		},
	}
}
