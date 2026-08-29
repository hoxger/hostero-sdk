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
	if _, err := os.Lstat(configPath); err == nil {
		return fmt.Errorf("%s already exists", config.FileName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", config.FileName, err)
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	writer := cmd.OutOrStdout()
	target := config.DefaultTarget()
	output, err := promptOutput(reader, writer, target.Output)
	if err != nil {
		return err
	}
	target.Output = output

	if err := config.WriteNew(configPath, config.New(target)); err != nil {
		return err
	}

	fmt.Fprintf(writer, "\nCreated %s\n\nNext: hostero-devkit generate\n", config.FileName)
	return nil
}

func promptOutput(reader *bufio.Reader, writer io.Writer, defaultValue string) (string, error) {
	for {
		output, err := prompt(reader, writer, "Output directory", defaultValue)
		if err != nil {
			return "", err
		}

		if err := validateOutput(output); err == nil {
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

func validateOutput(output string) error {
	cleaned := filepath.Clean(output)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.New("output escapes the project directory")
	}
	return nil
}
