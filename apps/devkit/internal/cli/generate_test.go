package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesCompilablePythonSources(t *testing.T) {
	workingDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	var initOutput bytes.Buffer
	initCommand := NewRootCommand("test", strings.NewReader("\n"), &initOutput, &bytes.Buffer{})
	initCommand.SetArgs([]string{"init"})
	if err := initCommand.Execute(); err != nil {
		t.Fatalf("run init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDirectory, "hostero.devkit.lock.yaml")); err != nil {
		t.Fatalf("inspect generated lockfile: %v", err)
	}

	var generateOutput bytes.Buffer
	generateCommand := NewRootCommand("test", strings.NewReader(""), &generateOutput, &bytes.Buffer{})
	generateCommand.SetArgs([]string{"generate"})
	if err := generateCommand.Execute(); err != nil {
		t.Fatalf("run generate: %v", err)
	}
	if !strings.Contains(generateOutput.String(), "packages/python/src/hostero/_generated") {
		t.Fatalf("unexpected generate output: %q", generateOutput.String())
	}

	generatedDirectory := filepath.Join(workingDirectory, "packages", "python", "src", "hostero", "_generated")
	paths, err := filepath.Glob(filepath.Join(generatedDirectory, "*.py"))
	if err != nil || len(paths) != 3 {
		t.Fatalf("generated Python files = %v, %v", paths, err)
	}

	command := exec.Command("python3", append([]string{"-m", "py_compile"}, paths...)...)
	command.Dir = workingDirectory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile generated Python: %v\n%s", err, output)
	}
}
