package lock

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
	"gopkg.in/yaml.v3"
)

const FileName = "hostero.devkit.lock.yaml"

type File struct {
	Version int     `yaml:"version"`
	OpenAPI OpenAPI `yaml:"openapi"`
}

type OpenAPI struct {
	Release string `yaml:"release"`
	SHA256  string `yaml:"sha256"`
}

func New(document source.Document) File {
	return File{
		Version: 1,
		OpenAPI: OpenAPI{
			Release: document.Release,
			SHA256:  document.SHA256,
		},
	}
}

func WriteNew(path string, file File) error {
	if err := file.Validate(); err != nil {
		return fmt.Errorf("validate %s: %w", FileName, err)
	}

	output, err := encode(file)
	if err != nil {
		return err
	}

	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", FileName, err)
	}
	if _, err := handle.Write(output); err != nil {
		_ = handle.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", FileName, err)
	}
	if err := handle.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", FileName, err)
	}
	return nil
}

func Replace(path string, file File) error {
	if err := file.Validate(); err != nil {
		return fmt.Errorf("validate %s: %w", FileName, err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s must not be a symlink", FileName)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular file", FileName)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", FileName, err)
	}

	output, err := encode(file)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hostero.devkit.lock-*")
	if err != nil {
		return fmt.Errorf("create %s temporary file: %w", FileName, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set %s permissions: %w", FileName, err)
	}
	if _, err := temporary.Write(output); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s temporary file: %w", FileName, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s temporary file: %w", FileName, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", FileName, err)
	}
	return nil
}

func encode(file File) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(file); err != nil {
		return nil, fmt.Errorf("encode %s: %w", FileName, err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML encoder for %s: %w", FileName, err)
	}
	return output.Bytes(), nil
}

func Load(path string) (File, error) {
	handle, err := os.Open(path)
	if err != nil {
		return File{}, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			fmt.Printf("close %s: %v\n", filepath.Base(path), err)
		}
	}()

	decoder := yaml.NewDecoder(handle)
	decoder.KnownFields(true)
	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return File{}, fmt.Errorf("decode %s: expected one YAML document", filepath.Base(path))
		}
		return File{}, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	if err := file.Validate(); err != nil {
		return File{}, fmt.Errorf("validate %s: %w", filepath.Base(path), err)
	}
	return file, nil
}

func (file File) Verify(document source.Document) error {
	if file.OpenAPI.Release != document.Release {
		return fmt.Errorf("OpenAPI release %q does not match locked release %q", document.Release, file.OpenAPI.Release)
	}
	if file.OpenAPI.SHA256 != document.SHA256 {
		return fmt.Errorf("OpenAPI SHA-256 %s does not match lock %s", document.SHA256, file.OpenAPI.SHA256)
	}
	return nil
}

func (file File) Validate() error {
	if file.Version != 1 {
		return fmt.Errorf("unsupported lock version %d", file.Version)
	}
	if strings.TrimSpace(file.OpenAPI.Release) == "" {
		return errors.New("OpenAPI release is required")
	}
	if len(file.OpenAPI.SHA256) != 64 || strings.ToLower(file.OpenAPI.SHA256) != file.OpenAPI.SHA256 {
		return errors.New("OpenAPI SHA-256 must be a lowercase SHA-256 digest")
	}
	if _, err := hex.DecodeString(file.OpenAPI.SHA256); err != nil {
		return errors.New("OpenAPI SHA-256 must be a lowercase SHA-256 digest")
	}
	return nil
}
