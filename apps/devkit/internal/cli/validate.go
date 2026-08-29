package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
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

	file, err := config.Load(filepath.Join(workingDirectory, config.FileName))
	if err != nil {
		return err
	}
	document, err := source.Resolve(workingDirectory, file.OpenAPI)
	if err != nil {
		return err
	}
	if _, err := openapi.Parse(document); err != nil {
		return err
	}

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"Valid %s (release %s, sha256 %s)\n",
		config.FileName,
		document.Release,
		document.SHA256[:12],
	)
	return nil
}
