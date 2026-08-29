package contract

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
)

func Build(parsed openapi.Document) (Document, error) {
	specification := parsed.Specification
	if specification == nil {
		return Document{}, fmt.Errorf("OpenAPI specification is required")
	}

	document, err := buildDocumentIdentity(specification)
	if err != nil {
		return Document{}, err
	}

	classes, err := classifyComponentSchemas(specification.Components)
	if err != nil {
		return Document{}, err
	}
	models, enums, err := buildComponentSchemas(specification.Components, classes)
	if err != nil {
		return Document{}, err
	}
	operations, err := buildOperations(specification, classes)
	if err != nil {
		return Document{}, err
	}

	document.Models = models
	document.Enums = enums
	document.Operations = operations
	return document, nil
}

func buildDocumentIdentity(specification *openapi3.T) (Document, error) {
	if specification.Info == nil || strings.TrimSpace(specification.Info.Title) == "" {
		return Document{}, fmt.Errorf("OpenAPI info.title is required")
	}
	if strings.TrimSpace(specification.Info.Version) == "" {
		return Document{}, fmt.Errorf("OpenAPI info.version is required")
	}
	if len(specification.Servers) != 1 || specification.Servers[0] == nil {
		return Document{}, fmt.Errorf("OpenAPI must define exactly one server")
	}
	if strings.TrimSpace(specification.Servers[0].URL) == "" {
		return Document{}, fmt.Errorf("OpenAPI server URL is required")
	}
	if len(specification.Servers[0].Variables) != 0 {
		return Document{}, fmt.Errorf("OpenAPI server variables are not supported")
	}
	if err := validateAPIKeySecurityScheme(specification.Components); err != nil {
		return Document{}, err
	}

	return Document{
		Title:     specification.Info.Title,
		Version:   specification.Info.Version,
		ServerURL: specification.Servers[0].URL,
	}, nil
}

func validateAPIKeySecurityScheme(components *openapi3.Components) error {
	if components == nil {
		return fmt.Errorf("OpenAPI components are required")
	}
	reference := components.SecuritySchemes["ApiKey"]
	if reference == nil || reference.Ref != "" || reference.Value == nil {
		return fmt.Errorf("OpenAPI ApiKey security scheme is required")
	}
	if reference.Value.Type != "http" || reference.Value.Scheme != "bearer" {
		return fmt.Errorf("OpenAPI ApiKey security scheme must use HTTP bearer authentication")
	}
	return nil
}
