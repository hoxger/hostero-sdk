package openapi

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

type Document struct {
	Source        source.Document
	Specification *openapi3.T
}

func Parse(sourceDocument source.Document) (Document, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	specification, err := loader.LoadFromData(sourceDocument.Bytes)
	if err != nil {
		return Document{}, fmt.Errorf("load OpenAPI document: %w", err)
	}
	if specification.OpenAPIMajorMinor() != "3.1" {
		return Document{}, fmt.Errorf("unsupported OpenAPI version %q", specification.OpenAPI)
	}
	if err := specification.Validate(context.Background()); err != nil {
		return Document{}, fmt.Errorf("validate OpenAPI document: %w", err)
	}

	return Document{
		Source:        sourceDocument,
		Specification: specification,
	}, nil
}
