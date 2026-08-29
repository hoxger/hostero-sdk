package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
)

type Document struct {
	Bytes   []byte
	Release string
	SHA256  string
}

func Resolve(projectDirectory string, openAPI config.OpenAPI) (Document, error) {
	if openAPI.Source.Kind != config.SourceKindFile {
		return Document{}, fmt.Errorf("unsupported OpenAPI source kind %q", openAPI.Source.Kind)
	}
	if err := config.ValidateProjectPath(openAPI.Source.Path); err != nil {
		return Document{}, fmt.Errorf("validate OpenAPI source path: %w", err)
	}

	projectRoot, err := filepath.Abs(projectDirectory)
	if err != nil {
		return Document{}, fmt.Errorf("resolve project directory: %w", err)
	}

	candidate := filepath.Join(projectRoot, openAPI.Source.Path)
	relative, err := filepath.Rel(projectRoot, candidate)
	if err != nil {
		return Document{}, fmt.Errorf("resolve OpenAPI source path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return Document{}, fmt.Errorf("OpenAPI source path escapes the project directory")
	}

	current := projectRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return Document{}, fmt.Errorf("inspect OpenAPI source: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return Document{}, fmt.Errorf("OpenAPI source must not contain symlinks")
		}
	}

	info, err := os.Lstat(candidate)
	if err != nil {
		return Document{}, fmt.Errorf("inspect OpenAPI source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("OpenAPI source must be a regular file")
	}

	contents, err := os.ReadFile(candidate)
	if err != nil {
		return Document{}, fmt.Errorf("read OpenAPI source: %w", err)
	}

	digest := sha256.Sum256(contents)
	return Document{
		Bytes:   contents,
		Release: openAPI.Release,
		SHA256:  hex.EncodeToString(digest[:]),
	}, nil
}
