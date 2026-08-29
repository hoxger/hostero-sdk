package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FileName = "hostero.devkit.yaml"
)

type Language string

const LanguagePython Language = "python"

type SourceKind string

const SourceKindURL SourceKind = "url"

const (
	defaultOpenAPIURL      = "http://localhost:8000/v1/openapi.json"
	defaultOpenAPISnapshot = "./openapi/hostero.openapi.json"
)

var supportedLanguages = [...]Language{LanguagePython}

func SupportedLanguages() []Language {
	return append([]Language(nil), supportedLanguages[:]...)
}

func (language Language) IsSupported() bool {
	for _, supported := range supportedLanguages {
		if language == supported {
			return true
		}
	}
	return false
}

type File struct {
	Version int      `yaml:"version"`
	OpenAPI OpenAPI  `yaml:"openapi"`
	Targets []Target `yaml:"targets"`
}

type OpenAPI struct {
	Source   Source `yaml:"source"`
	Snapshot string `yaml:"snapshot"`
}

type Source struct {
	Kind SourceKind `yaml:"kind"`
	URL  string     `yaml:"url"`
}

type Target struct {
	Language Language `yaml:"language"`
	Output   string   `yaml:"output"`
}

func DefaultTarget() Target {
	return Target{
		Language: LanguagePython,
		Output:   "./packages/python/src/hostero",
	}
}

func New(target Target) File {
	return File{
		Version: 1,
		OpenAPI: OpenAPI{
			Source: Source{
				Kind: SourceKindURL,
				URL:  defaultOpenAPIURL,
			},
			Snapshot: defaultOpenAPISnapshot,
		},
		Targets: []Target{target},
	}
}

func WriteNew(path string, file File) error {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(file); err != nil {
		return fmt.Errorf("encode %s: %w", FileName, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close YAML encoder for %s: %w", FileName, err)
	}

	contents := output.Bytes()

	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", FileName, err)
	}

	if _, err := handle.Write(contents); err != nil {
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

func Load(path string) (File, error) {
	handle, err := os.Open(path)
	if err != nil {
		return File{}, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer handle.Close()

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

func (file File) Validate() error {
	if file.Version != 1 {
		return fmt.Errorf("unsupported config version %d", file.Version)
	}
	if file.OpenAPI.Source.Kind != SourceKindURL {
		return fmt.Errorf("unsupported OpenAPI source kind %q", file.OpenAPI.Source.Kind)
	}
	if err := ValidateSourceURL(file.OpenAPI.Source.URL); err != nil {
		return fmt.Errorf("OpenAPI source URL: %w", err)
	}
	if err := ValidateProjectPath(file.OpenAPI.Snapshot); err != nil {
		return fmt.Errorf("OpenAPI snapshot path: %w", err)
	}
	if len(file.Targets) == 0 {
		return errors.New("at least one target is required")
	}

	for index, target := range file.Targets {
		if !target.Language.IsSupported() {
			return fmt.Errorf("target %d uses unsupported language %q", index+1, target.Language)
		}
		if err := ValidateOutput(target.Output); err != nil {
			return fmt.Errorf("target %d: %w", index+1, err)
		}
	}

	return nil
}

func ValidateSourceURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return errors.New("URL host is required")
	}
	if parsed.User != nil {
		return errors.New("URL must not include credentials")
	}
	return nil
}

func ValidateOutput(output string) error {
	if err := ValidateProjectPath(output); err != nil {
		return errors.New("output must stay inside the current project directory")
	}
	return nil
}

func ValidateProjectPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.New("path must stay inside the current project directory")
	}
	return nil
}
