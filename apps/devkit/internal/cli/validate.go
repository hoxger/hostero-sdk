package cli

import (
	"fmt"
	"os"

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

	pinned, err := loadPinnedContract(workingDirectory)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"Valid %s (release %s, sha256 %s)\n",
		config.FileName,
		pinned.Source.Release,
		pinned.Source.SHA256[:12],
	)
	return nil
}
