package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingWriter struct {
	err error
}

func (w *failingWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func TestInitCreatesExpectedFiles(t *testing.T) {
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

	var output bytes.Buffer
	command := NewRootCommand("test", strings.NewReader("\n"), &output, &bytes.Buffer{})
	command.SetArgs([]string{"init"})
	if err := command.Execute(); err != nil {
		t.Fatalf("run init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(workingDirectory, "hostero.devkit.yaml")); err != nil {
		t.Fatalf("hostero.devkit.yaml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDirectory, "hostero.devkit.lock.yaml")); err != nil {
		t.Fatalf("hostero.devkit.lock.yaml not created: %v", err)
	}
}

func TestInitReturnsErrorOnWriteFailure(t *testing.T) {
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

	command := NewRootCommand("test", strings.NewReader("\n"), &failingWriter{err: io.ErrClosedPipe}, &bytes.Buffer{})
	command.SetArgs([]string{"init"})
	err = command.Execute()
	if err == nil {
		t.Fatalf("expected write error on init, got nil")
	}
	if !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "closed pipe") && !strings.Contains(err.Error(), "write") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
