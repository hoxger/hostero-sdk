package cli

import (
	"path/filepath"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/lock"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

type pinnedContract struct {
	Configuration config.File
	Source        source.Document
	Contract      contract.Document
}

func loadPinnedContract(workingDirectory string) (pinnedContract, error) {
	file, err := config.Load(filepath.Join(workingDirectory, config.FileName))
	if err != nil {
		return pinnedContract{}, err
	}
	document, err := source.Resolve(workingDirectory, file.OpenAPI)
	if err != nil {
		return pinnedContract{}, err
	}
	lockFile, err := lock.Load(filepath.Join(workingDirectory, lock.FileName))
	if err != nil {
		return pinnedContract{}, err
	}
	if err := lockFile.Verify(document); err != nil {
		return pinnedContract{}, err
	}
	parsed, err := openapi.Parse(document)
	if err != nil {
		return pinnedContract{}, err
	}
	contractDocument, err := contract.Build(parsed)
	if err != nil {
		return pinnedContract{}, err
	}
	return pinnedContract{Configuration: file, Source: document, Contract: contractDocument}, nil
}
