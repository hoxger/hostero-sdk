package python

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFormatsStagedGenerationBeforeReplacingOutput(t *testing.T) {
	project := t.TempDir()
	output := "./packages/python/src/hostero"
	packageRoot := filepath.Join(project, "packages", "python")
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		t.Fatalf("create Python package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "pyproject.toml"), []byte("[project]\nname = \"hostero\"\n"), 0o644); err != nil {
		t.Fatalf("write Python package configuration: %v", err)
	}

	previousRunner := runRuffFormat
	runRuffFormat = func(projectRoot string, generatedRoot string) error {
		if projectRoot != packageRoot {
			t.Errorf("Ruff project root = %q, want %q", projectRoot, packageRoot)
		}
		if filepath.Base(generatedRoot) != generatedDirectory {
			t.Errorf("Ruff generated root = %q", generatedRoot)
		}
		return os.WriteFile(filepath.Join(generatedRoot, "__init__.py"), []byte("formatted = True\n"), 0o644)
	}
	t.Cleanup(func() { runRuffFormat = previousRunner })

	if err := Write(project, output, map[string][]byte{"__init__.py": []byte("unformatted=True\n")}); err != nil {
		t.Fatalf("write formatted generation: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(packageRoot, "src", "hostero", generatedDirectory, "__init__.py"))
	if err != nil || string(contents) != "formatted = True\n" {
		t.Fatalf("formatted generated file = %q, %v", contents, err)
	}
}

func TestWritePreservesPreviousGenerationWhenFormattingFails(t *testing.T) {
	project := t.TempDir()
	output := "./packages/python/src/hostero"
	packageRoot := filepath.Join(project, "packages", "python")
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		t.Fatalf("create Python package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "pyproject.toml"), []byte("[project]\nname = \"hostero\"\n"), 0o644); err != nil {
		t.Fatalf("write Python package configuration: %v", err)
	}

	previousRunner := runRuffFormat
	runRuffFormat = func(string, string) error { return nil }
	if err := Write(project, output, map[string][]byte{"__init__.py": []byte("old = True\n")}); err != nil {
		t.Fatalf("write initial generation: %v", err)
	}
	runRuffFormat = func(string, string) error { return errors.New("Ruff unavailable") }
	t.Cleanup(func() { runRuffFormat = previousRunner })

	err := Write(project, output, map[string][]byte{"__init__.py": []byte("new = True\n")})
	if err == nil {
		t.Fatal("Write() error = nil, want formatting failure")
	}
	contents, readErr := os.ReadFile(filepath.Join(packageRoot, "src", "hostero", generatedDirectory, "__init__.py"))
	if readErr != nil || string(contents) != "old = True\n" {
		t.Fatalf("previous generated file = %q, %v", contents, readErr)
	}
}
