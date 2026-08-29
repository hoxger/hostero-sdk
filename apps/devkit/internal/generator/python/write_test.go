package python

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReplacesOnlyGeneratedDirectory(t *testing.T) {
	project := t.TempDir()
	output := "./packages/python/src/hostero"
	manualFile := filepath.Join(project, "packages", "python", "src", "hostero", "__init__.py")
	if err := os.MkdirAll(filepath.Dir(manualFile), 0o755); err != nil {
		t.Fatalf("create manual package directory: %v", err)
	}
	if err := os.WriteFile(manualFile, []byte("manual = True\n"), 0o644); err != nil {
		t.Fatalf("write manual file: %v", err)
	}

	if err := Write(project, output, map[string][]byte{
		"old.py":      []byte("old = True\n"),
		"__init__.py": []byte("first = True\n"),
	}); err != nil {
		t.Fatalf("write first generation: %v", err)
	}
	if err := Write(project, output, map[string][]byte{"__init__.py": []byte("second = True\n")}); err != nil {
		t.Fatalf("write replacement generation: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(project, "packages", "python", "src", "hostero", "_generated", "__init__.py"))
	if err != nil || string(contents) != "second = True\n" {
		t.Fatalf("generated file = %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(project, "packages", "python", "src", "hostero", "_generated", "old.py")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale generated file still exists: %v", err)
	}
	contents, err = os.ReadFile(manualFile)
	if err != nil || string(contents) != "manual = True\n" {
		t.Fatalf("manual package file changed: %q, %v", contents, err)
	}
}

func TestWriteRejectsGeneratedDirectorySymlink(t *testing.T) {
	project := t.TempDir()
	outputRoot := filepath.Join(project, "output")
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(outputRoot, generatedDirectory)); err != nil {
		t.Fatalf("create generated symlink: %v", err)
	}

	err := Write(project, "./output", map[string][]byte{"__init__.py": []byte("")})
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("Write() error = %v, want symlink rejection", err)
	}
}
