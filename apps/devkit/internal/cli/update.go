package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/lock"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Fetch, validate, and pin the configured Hostero API contract.",
		Args:  cobra.NoArgs,
		RunE:  runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	configuration, err := config.Load(filepath.Join(workingDirectory, config.FileName))
	if err != nil {
		return err
	}
	previous, err := source.Resolve(workingDirectory, configuration.OpenAPI)
	if err != nil {
		return err
	}
	previousLock, err := lock.Load(filepath.Join(workingDirectory, lock.FileName))
	if err != nil {
		return err
	}
	if err := previousLock.Verify(previous); err != nil {
		return err
	}

	document, err := source.Fetch(cmd.Context(), configuration.OpenAPI)
	if err != nil {
		return err
	}
	parsed, err := openapi.Parse(document)
	if err != nil {
		return fmt.Errorf("validate fetched OpenAPI contract: %w", err)
	}
	if _, err := contract.Build(parsed); err != nil {
		return fmt.Errorf("build fetched OpenAPI contract: %w", err)
	}

	if err := source.WriteSnapshot(workingDirectory, configuration.OpenAPI, document); err != nil {
		return err
	}
	if err := lock.Replace(filepath.Join(workingDirectory, lock.FileName), lock.New(document)); err != nil {
		if restoreErr := source.WriteSnapshot(workingDirectory, configuration.OpenAPI, previous); restoreErr != nil {
			return fmt.Errorf("update OpenAPI lock: %w; restore snapshot: %v", err, restoreErr)
		}
		return fmt.Errorf("update OpenAPI lock: %w", err)
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Updated OpenAPI snapshot (release %s, sha256 %s)\n",
		document.Release,
		document.SHA256[:12],
	)
	return nil
}
