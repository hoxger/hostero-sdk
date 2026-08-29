package python

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const generatedDirectory = "_generated"

func Write(projectDirectory string, output string, files map[string][]byte) error {
	projectRoot, err := filepath.Abs(projectDirectory)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	outputRoot, err := resolveOutputRoot(projectRoot, output)
	if err != nil {
		return err
	}
	if err := ensureSafeDirectory(projectRoot, outputRoot); err != nil {
		return err
	}

	generatedRoot := filepath.Join(outputRoot, generatedDirectory)
	if err := ensureReplaceableGeneratedDirectory(generatedRoot); err != nil {
		return err
	}

	stageRoot, err := os.MkdirTemp(outputRoot, ".hostero-devkit-stage-")
	if err != nil {
		return fmt.Errorf("create generated source staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(stageRoot); err != nil {
			fmt.Printf("remove generated source staging directory: %v\n", err)
		}
	}()

	stagedGeneratedRoot := filepath.Join(stageRoot, generatedDirectory)
	if err := writeFiles(stagedGeneratedRoot, files); err != nil {
		return err
	}
	if err := formatGeneratedDirectory(projectRoot, outputRoot, stagedGeneratedRoot); err != nil {
		return err
	}
	if err := replaceGeneratedDirectory(outputRoot, generatedRoot, stagedGeneratedRoot); err != nil {
		return err
	}
	return nil
}

func resolveOutputRoot(projectRoot string, output string) (string, error) {
	if filepath.IsAbs(output) {
		return "", fmt.Errorf("generated output must stay inside the project directory")
	}
	outputRoot := filepath.Clean(filepath.Join(projectRoot, output))
	relative, err := filepath.Rel(projectRoot, outputRoot)
	if err != nil {
		return "", fmt.Errorf("resolve generated output: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generated output must stay inside the project directory")
	}
	return outputRoot, nil
}

func ensureSafeDirectory(projectRoot string, directory string) error {
	info, err := os.Lstat(projectRoot)
	if err != nil {
		return fmt.Errorf("inspect project directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("project directory must be a non-symlink directory")
	}

	relative, err := filepath.Rel(projectRoot, directory)
	if err != nil {
		return fmt.Errorf("resolve generated output: %w", err)
	}
	current := projectRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("create generated output directory: %w", err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect generated output directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("generated output path must contain only non-symlink directories")
		}
	}
	return nil
}

func ensureReplaceableGeneratedDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect generated directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("generated directory must not be a symlink")
	}
	if !info.IsDir() {
		return fmt.Errorf("generated directory must be a directory")
	}
	return nil
}

func writeFiles(directory string, files map[string][]byte) error {
	if len(files) == 0 {
		return fmt.Errorf("generated Python files are required")
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		return fmt.Errorf("create generated directory: %w", err)
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := validateGeneratedPath(path); err != nil {
			return err
		}
		filePath := filepath.Join(directory, path)
		if err := os.WriteFile(filePath, files[path], 0o644); err != nil {
			return fmt.Errorf("write generated file %q: %w", path, err)
		}
	}
	return nil
}

func validateGeneratedPath(path string) error {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || filepath.Dir(cleaned) != "." {
		return fmt.Errorf("generated path %q is invalid", path)
	}
	return nil
}

func replaceGeneratedDirectory(outputRoot string, generatedRoot string, stagedGeneratedRoot string) error {
	backupRoot, err := os.MkdirTemp(outputRoot, ".hostero-devkit-backup-")
	if err != nil {
		return fmt.Errorf("create generated source backup path: %w", err)
	}
	if err := os.Remove(backupRoot); err != nil {
		return fmt.Errorf("prepare generated source backup path: %w", err)
	}

	hadPreviousGeneration := false
	if _, err := os.Lstat(generatedRoot); err == nil {
		if err := os.Rename(generatedRoot, backupRoot); err != nil {
			return fmt.Errorf("back up generated source: %w", err)
		}
		hadPreviousGeneration = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect generated source before replacement: %w", err)
	}

	if err := os.Rename(stagedGeneratedRoot, generatedRoot); err != nil {
		if hadPreviousGeneration {
			if restoreErr := os.Rename(backupRoot, generatedRoot); restoreErr != nil {
				return fmt.Errorf("replace generated source: %w (restore previous generation: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("replace generated source: %w", err)
	}

	if hadPreviousGeneration {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("remove generated source backup: %w", err)
		}
	}
	return nil
}
