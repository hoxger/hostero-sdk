package contract_test

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/bootstrap"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestBuildFixture(t *testing.T) {
	parsed := parseFixture(t)

	document, err := contract.Build(parsed)
	if err != nil {
		t.Fatalf("build fixture contract: %v", err)
	}

	if document.Title != "Hostero API" || document.Version != "mvp" || document.ServerURL != "https://api.hostero.gg/v1" {
		t.Fatalf("unexpected document identity: %#v", document)
	}
	if len(document.Models) != 6 || len(document.Enums) != 1 || len(document.Operations) != 3 {
		t.Fatalf("unexpected contract sizes: %#v", document)
	}

	status := document.Enums[0]
	if status.Name != "GameServerStatus" || strings.Join(status.Values, ",") != "error,install_failed,installed,installing,restoring,running,starting,stopped,stopping,suspended" {
		t.Fatalf("unexpected status enum: %#v", status)
	}

	server := findModel(t, document.Models, "GameServerListItem")
	if field := findField(t, server, "expires_at"); field.Type.Kind != contract.KindString || !field.Type.Nullable || field.Type.Format != "date-time" {
		t.Fatalf("unexpected expires_at field: %#v", field)
	}
	if field := findField(t, server, "primary_allocation"); field.Type.Kind != contract.KindModel || field.Type.Name != "PrimaryAllocation" || !field.Type.Nullable {
		t.Fatalf("unexpected primary_allocation field: %#v", field)
	}

	operations := document.Operations
	if operations[0].ID != "listGameServers" || operations[1].ID != "getServersOverview" || operations[2].ID != "restartGameServer" {
		t.Fatalf("operations are not deterministic: %#v", operations)
	}
	if len(operations[0].Parameters) != 2 || operations[0].Parameters[0].Name != "limit" || operations[0].Parameters[0].Default != float64(20) || operations[0].Parameters[1].Name != "offset" || operations[0].Parameters[1].Default != float64(0) {
		t.Fatalf("unexpected listGameServers parameters: %#v", operations[0].Parameters)
	}
	if operations[2].Method != "POST" || operations[2].Response.Status != 204 || operations[2].Response.Type != nil || strings.Join(operations[2].Scopes, ",") != "servers:power" {
		t.Fatalf("unexpected restartGameServer operation: %#v", operations[2])
	}
}

func TestBuildRejectsUnsupportedContracts(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*openapi.Document)
		message string
	}{
		{
			name: "missing required scopes",
			mutate: func(parsed *openapi.Document) {
				delete(parsed.Specification.Paths.Value("/servers").Get.Extensions, "x-hostero-required-scopes")
			},
			message: "x-hostero-required-scopes is required",
		},
		{
			name: "anonymous object field",
			mutate: func(parsed *openapi.Document) {
				model := parsed.Specification.Components.Schemas["GameServerListItem"].Value
				model.Properties["name"] = &openapi3.SchemaRef{Value: openapi3.NewObjectSchema()}
			},
			message: "anonymous object schemas are not supported",
		},
		{
			name: "duplicate operation id",
			mutate: func(parsed *openapi.Document) {
				parsed.Specification.Paths.Value("/servers/overview").Get.OperationID = "listGameServers"
			},
			message: "operationId \"listGameServers\" is duplicated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseFixture(t)
			test.mutate(&parsed)

			_, err := contract.Build(parsed)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Build() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}

func parseFixture(t *testing.T) openapi.Document {
	t.Helper()
	parsed, err := openapi.Parse(source.Document{Bytes: bootstrap.OpenAPI()})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return parsed
}

func findModel(t *testing.T, models []contract.Model, name string) contract.Model {
	t.Helper()
	for _, model := range models {
		if model.Name == name {
			return model
		}
	}
	t.Fatalf("model %q not found", name)
	return contract.Model{}
}

func findField(t *testing.T, model contract.Model, name string) contract.Field {
	t.Helper()
	for _, field := range model.Fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("field %q not found in model %q", name, model.Name)
	return contract.Field{}
}
