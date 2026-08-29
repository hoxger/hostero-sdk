package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
)

const maxDocumentSize = 16 << 20

var httpClient = &http.Client{Timeout: 15 * time.Second}

type Document struct {
	Bytes   []byte
	Release string
	SHA256  string
}

func Resolve(projectDirectory string, openAPI config.OpenAPI) (Document, error) {
	candidate, relative, err := snapshotPath(projectDirectory, openAPI.Snapshot)
	if err != nil {
		return Document{}, err
	}
	if err := inspectPath(candidate, relative); err != nil {
		return Document{}, err
	}

	contents, err := os.ReadFile(candidate)
	if err != nil {
		return Document{}, fmt.Errorf("read OpenAPI snapshot: %w", err)
	}
	return documentFromBytes(contents)
}

func Fetch(openAPI config.OpenAPI) (Document, error) {
	if openAPI.Source.Kind != config.SourceKindURL {
		return Document{}, fmt.Errorf("unsupported OpenAPI source kind %q", openAPI.Source.Kind)
	}
	if err := config.ValidateSourceURL(openAPI.Source.URL); err != nil {
		return Document{}, fmt.Errorf("validate OpenAPI source URL: %w", err)
	}

	request, err := http.NewRequest(http.MethodGet, openAPI.Source.URL, nil)
	if err != nil {
		return Document{}, fmt.Errorf("create OpenAPI request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return Document{}, fmt.Errorf("fetch OpenAPI source: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Document{}, fmt.Errorf("fetch OpenAPI source: unexpected HTTP status %s", response.Status)
	}

	contents, err := io.ReadAll(io.LimitReader(response.Body, maxDocumentSize+1))
	if err != nil {
		return Document{}, fmt.Errorf("read OpenAPI source: %w", err)
	}
	if len(contents) > maxDocumentSize {
		return Document{}, fmt.Errorf("read OpenAPI source: document exceeds %d MiB limit", maxDocumentSize>>20)
	}
	return documentFromBytes(contents)
}

func WriteSnapshot(projectDirectory string, openAPI config.OpenAPI, document Document) error {
	candidate, relative, err := snapshotPath(projectDirectory, openAPI.Snapshot)
	if err != nil {
		return err
	}
	if err := inspectExistingParents(candidate, relative); err != nil {
		return err
	}

	directory := filepath.Dir(candidate)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create OpenAPI snapshot directory: %w", err)
	}
	if err := inspectExistingParents(candidate, relative); err != nil {
		return err
	}
	if info, err := os.Lstat(candidate); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("OpenAPI snapshot must not be a symlink")
		}
		if !info.Mode().IsRegular() {
			return errors.New("OpenAPI snapshot must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect OpenAPI snapshot: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".hostero.openapi-*")
	if err != nil {
		return fmt.Errorf("create OpenAPI snapshot temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set OpenAPI snapshot permissions: %w", err)
	}
	if _, err := temporary.Write(document.Bytes); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write OpenAPI snapshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close OpenAPI snapshot: %w", err)
	}
	if err := os.Rename(temporaryPath, candidate); err != nil {
		return fmt.Errorf("replace OpenAPI snapshot: %w", err)
	}
	return nil
}

func documentFromBytes(contents []byte) (Document, error) {
	var metadata struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(contents, &metadata); err != nil {
		return Document{}, fmt.Errorf("decode OpenAPI metadata: %w", err)
	}
	release := strings.TrimSpace(metadata.Info.Version)
	if release == "" {
		return Document{}, errors.New("OpenAPI info.version is required")
	}

	digest := sha256.Sum256(contents)
	return Document{
		Bytes:   contents,
		Release: release,
		SHA256:  hex.EncodeToString(digest[:]),
	}, nil
}

func snapshotPath(projectDirectory string, snapshot string) (string, string, error) {
	if err := config.ValidateProjectPath(snapshot); err != nil {
		return "", "", fmt.Errorf("validate OpenAPI snapshot path: %w", err)
	}
	projectRoot, err := filepath.Abs(projectDirectory)
	if err != nil {
		return "", "", fmt.Errorf("resolve project directory: %w", err)
	}
	candidate := filepath.Join(projectRoot, snapshot)
	relative, err := filepath.Rel(projectRoot, candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve OpenAPI snapshot path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("OpenAPI snapshot path escapes the project directory")
	}
	return candidate, relative, nil
}

func inspectPath(candidate string, relative string) error {
	if err := inspectExistingParents(candidate, relative); err != nil {
		return err
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return fmt.Errorf("inspect OpenAPI snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("OpenAPI snapshot must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return errors.New("OpenAPI snapshot must be a regular file")
	}
	return nil
}

func inspectExistingParents(candidate string, relative string) error {
	current := filepath.Dir(candidate)
	for range strings.Split(relative, string(filepath.Separator)) {
		if current == filepath.Dir(current) {
			break
		}
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return fmt.Errorf("inspect OpenAPI snapshot directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("OpenAPI snapshot must not contain symlinks")
		}
		current = filepath.Dir(current)
	}
	return nil
}
