package python

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var runRuffFormat = runRuffFormatCommand

func formatGeneratedDirectory(projectRoot string, outputRoot string, generatedRoot string) error {
	pythonProjectRoot, found, err := findPythonProjectRoot(projectRoot, outputRoot)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := runRuffFormat(pythonProjectRoot, generatedRoot); err != nil {
		return fmt.Errorf("format generated Python source: %w", err)
	}
	return nil
}

func findPythonProjectRoot(projectRoot string, outputRoot string) (string, bool, error) {
	for current := outputRoot; ; current = filepath.Dir(current) {
		configuration := filepath.Join(current, "pyproject.toml")
		info, err := os.Lstat(configuration)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return "", false, fmt.Errorf("Python project configuration must be a regular file")
			}
			return current, true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("inspect Python project configuration: %w", err)
		}
		if current == projectRoot {
			return "", false, nil
		}
	}
}

func runRuffFormatCommand(projectRoot string, generatedRoot string) error {
	commands := [][]string{
		{"ruff", "check", "--select", "I", "--fix", generatedRoot},
		{"ruff", "format", generatedRoot},
	}
	for _, arguments := range commands {
		command := exec.Command("uv", append([]string{"run", "--project", projectRoot}, arguments...)...)
		command.Dir = projectRoot
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("run Ruff %s: %w\n%s", arguments[1], err, output)
		}
	}
	return nil
}
