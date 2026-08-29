package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	FileName = "hostero.devkit.yaml"
)

type Language string

const LanguagePython Language = "python"

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
	Source  string `yaml:"source"`
	Release string `yaml:"release"`
}

type Target struct {
	Language Language `yaml:"language"`
	Output   string   `yaml:"output"`
}

func DefaultTarget() Target {
	return Target{
		Language: LanguagePython,
		Output:   "./src/hostero_sdk",
	}
}

func New(target Target) File {
	return File{
		Version: 1,
		OpenAPI: OpenAPI{
			Source:  "hostero",
			Release: "latest",
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
