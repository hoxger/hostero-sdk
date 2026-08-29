package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate DevKit configuration.",
		Args:  cobra.NoArgs,
		RunE:  runValidate,
	}
}

func runValidate(cmd *cobra.Command, _ []string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	if _, err := config.Load(filepath.Join(workingDirectory, config.FileName)); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Valid %s\n", config.FileName)
	return nil
}
