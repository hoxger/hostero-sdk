package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/lock"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a DevKit project configuration.",
		Args:  cobra.NoArgs,
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	configPath := filepath.Join(workingDirectory, config.FileName)
	if err := ensureNewFile(configPath, config.FileName); err != nil {
		return err
	}
	lockPath := filepath.Join(workingDirectory, lock.FileName)
	if err := ensureNewFile(lockPath, lock.FileName); err != nil {
		return err
	}

	configuration := config.New(config.DefaultTarget())
	fixturePath := filepath.Join(workingDirectory, configuration.OpenAPI.Snapshot)
	if err := ensureNewFile(fixturePath, configuration.OpenAPI.Snapshot); err != nil {
		return err
	}
	if err := ensureDirectory(filepath.Dir(fixturePath), "OpenAPI directory"); err != nil {
		return err
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	writer := cmd.OutOrStdout()
	target := config.DefaultTarget()
	output, err := promptOutput(reader, writer, target.Output)
	if err != nil {
		return err
	}
	target.Output = output

	sourceDocument, err := source.Fetch(configuration.OpenAPI)
	if err != nil {
		return fmt.Errorf("fetch OpenAPI contract: %w", err)
	}
	if err := source.WriteSnapshot(workingDirectory, configuration.OpenAPI, sourceDocument); err != nil {
		return err
	}
	configuration.Targets[0] = target
	if err := config.WriteNew(configPath, configuration); err != nil {
		_ = os.Remove(fixturePath)
		return err
	}
	if err := lock.WriteNew(lockPath, lock.New(sourceDocument)); err != nil {
		_ = os.Remove(configPath)
		_ = os.Remove(fixturePath)
		return err
	}

	fmt.Fprintf(
		writer,
		"\nCreated %s\nCreated %s\nCreated %s\n\nNext: hostero-devkit update && hostero-devkit generate\n",
		config.FileName,
		configuration.OpenAPI.Snapshot,
		lock.FileName,
	)
	return nil
}

func ensureNewFile(path string, label string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s already exists", label)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	return nil
}

func ensureDirectory(path string, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", label)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s must be a directory", label)
	}
	return nil
}

func promptOutput(reader *bufio.Reader, writer io.Writer, defaultValue string) (string, error) {
	for {
		output, err := prompt(reader, writer, "Output directory", defaultValue)
		if err != nil {
			return "", err
		}

		if err := config.ValidateOutput(output); err == nil {
			return output, nil
		}

		fmt.Fprintln(writer, "Output must stay inside the current project directory.")
	}
}

func prompt(reader *bufio.Reader, writer io.Writer, label string, defaultValue string) (string, error) {
	fmt.Fprintf(writer, "? %s [%s]: ", label, defaultValue)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read %s: %w", strings.ToLower(label), err)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}
