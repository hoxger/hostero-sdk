package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	python "github.com/hoxger/hostero-sdk/apps/devkit/internal/generator/python"
	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate SDK source from the configured API contract.",
		Args:  cobra.NoArgs,
		RunE:  runGenerate,
	}
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	pinned, err := loadPinnedContract(workingDirectory)
	if err != nil {
		return err
	}

	for _, target := range pinned.Configuration.Targets {
		if target.Language != config.LanguagePython {
			return fmt.Errorf("generate target language %q is not supported", target.Language)
		}
		pythonDocument, err := python.Build(pinned.Contract)
		if err != nil {
			return fmt.Errorf("build Python SDK: %w", err)
		}
		files, err := python.Render(pythonDocument, python.GenerationMetadata{
			DevKitVersion: cmd.Root().Version,
			OpenAPISource: pinned.Configuration.OpenAPI.Source.URL,
			Release:       pinned.Source.Release,
			SHA256:        pinned.Source.SHA256,
		})
		if err != nil {
			return fmt.Errorf("render Python SDK: %w", err)
		}
		if err := python.Write(workingDirectory, target.Output, files); err != nil {
			return fmt.Errorf("write Python SDK: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generated Python SDK in %s/_generated\n", filepath.ToSlash(target.Output))
	}
	return nil
}
