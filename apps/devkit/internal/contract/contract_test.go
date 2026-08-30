package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestBuildPinnedPublicSnapshotOperations(t *testing.T) {
	document, err := contract.Build(parseFixture(t))
	if err != nil {
		t.Fatalf("build pinned public contract: %v", err)
	}
	if len(document.Operations) < 80 {
		t.Fatalf("public operation count = %d, want at least 80", len(document.Operations))
	}

	attachment := findOperation(t, document.Operations, "tickets_messages_attachments_create")
	if attachment.RequestBody == nil || attachment.RequestBody.ContentType != "multipart/form-data" || attachment.RequestBody.Type.Kind != contract.KindModel || attachment.Success.Status != 201 || attachment.Success.Type == nil || attachment.Success.Type.Name != "TicketAttachmentResource" {
		t.Fatalf("unexpected attachment operation: %#v", attachment)
	}
	if attachment.ClientMetadata.Method != "create" || strings.Join(attachment.ClientMetadata.Group, ".") != "tickets.messages.attachments" {
		t.Fatalf("unexpected attachment client metadata: %#v", attachment.ClientMetadata)
	}

	restart := findOperation(t, document.Operations, "servers_power_restart_create")
	if restart.ClientMetadata.Method != "restart" || strings.Join(restart.ClientMetadata.Group, ".") != "servers.power" {
		t.Fatalf("unexpected restart client metadata: %#v", restart.ClientMetadata)
	}

	if len(document.Resources) != 2 {
		t.Fatalf("resource count = %d, want 2", len(document.Resources))
	}
	for _, resource := range document.Resources {
		if resource.Kind != "game_server" || resource.IDField != "id" || resource.PathParameter != "server_id" || strings.Join(resource.Group, ".") != "servers" {
			t.Fatalf("unexpected resource metadata: %#v", resource)
		}
	}

	download := findOperation(t, document.Operations, "servers_backups_download_list")
	if download.Success.Status != 302 || download.Success.Type != nil || strings.Join(download.Permissions, ",") != "game_servers.backups.download" {
		t.Fatalf("unexpected backup download operation: %#v", download)
	}
	if len(download.Errors) != 6 || download.Errors[0].Status != 401 || download.Errors[len(download.Errors)-1].Status != 429 {
		t.Fatalf("unexpected standard errors: %#v", download.Errors)
	}
}

func TestBuildRejectsUnsupportedContracts(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*openapi.Document)
		message string
	}{
		{
			name: "anonymous object field",
			mutate: func(parsed *openapi.Document) {
				model := parsed.Specification.Components.Schemas["TicketCreateRequest"].Value
				model.Properties["name"] = &openapi3.SchemaRef{Value: openapi3.NewObjectSchema()}
			},
			message: "anonymous object schemas are not supported",
		},
		{
			name: "resource has invalid id field",
			mutate: func(parsed *openapi.Document) {
				schema := parsed.Specification.Components.Schemas["GameServerListItemResource"].Value
				schema.Extensions = map[string]any{"x-hostero-client-resource": map[string]any{
					"kind": "game_server", "id_field": "missing", "group": []any{"servers"}, "path_parameter": "server_id",
				}}
			},
			message: "id_field \"missing\" is not a model field",
		},
		{
			name: "resource has unbindable path parameter",
			mutate: func(parsed *openapi.Document) {
				metadata := map[string]any{
					"kind": "game_server", "id_field": "id", "group": []any{"servers"}, "path_parameter": "unknown_id",
				}
				parsed.Specification.Components.Schemas["GameServerListItemResource"].Value.Extensions = map[string]any{"x-hostero-client-resource": metadata}
				parsed.Specification.Components.Schemas["GameServerDetailResource"].Value.Extensions = map[string]any{"x-hostero-client-resource": metadata}
			},
			message: "has no matching resource-scoped operation",
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

func TestBuildSupportsJSONAliasesAndClosedModels(t *testing.T) {
	parsed, err := openapi.Parse(source.Document{Bytes: []byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "v1"},
  "servers": [{"url": "https://api.example.test/v1"}],
  "paths": {},
  "components": {
    "securitySchemes": {"ApiKey": {"type": "http", "scheme": "bearer"}},
    "schemas": {
      "JSONScalar": {"anyOf": [{"type": "string"}, {"type": "integer"}, {"type": "boolean"}, {"type": "null"}]},
      "JSONValue": {"anyOf": [
        {"$ref": "#/components/schemas/JSONScalar"},
        {"type": "object", "additionalProperties": {"$ref": "#/components/schemas/JSONValue"}},
        {"type": "array", "items": {"$ref": "#/components/schemas/JSONValue"}}
      ]},
      "JSONObject": {"type": "object", "additionalProperties": {"$ref": "#/components/schemas/JSONValue"}},
      "Envelope": {
        "type": "object",
        "additionalProperties": false,
        "required": ["payload"],
        "properties": {
          "payload": {"$ref": "#/components/schemas/JSONValue"},
          "attributes": {"$ref": "#/components/schemas/JSONObject"},
          "metadata": {"type": "object", "additionalProperties": true},
          "detail": {"anyOf": [{}, {"type": "null"}]}
        }
      }
    }
  }
}`)})
	if err != nil {
		t.Fatalf("parse JSON alias fixture: %v", err)
	}

	document, err := contract.Build(parsed)
	if err != nil {
		t.Fatalf("build JSON alias fixture: %v", err)
	}
	if len(document.Aliases) != 3 || len(document.Models) != 1 {
		t.Fatalf("unexpected JSON alias contract: %#v", document)
	}
	if scalar := findAlias(t, document.Aliases, "JSONScalar"); scalar.Type.Kind != contract.KindUnion || !scalar.Type.Nullable || len(scalar.Type.Values) != 3 {
		t.Fatalf("unexpected JSONScalar alias: %#v", scalar)
	}
	if value := findAlias(t, document.Aliases, "JSONValue"); value.Type.Kind != contract.KindUnion || len(value.Type.Values) != 3 || value.Type.Values[1].Kind != contract.KindMap || value.Type.Values[2].Kind != contract.KindArray {
		t.Fatalf("unexpected JSONValue alias: %#v", value)
	}
	envelope := findModel(t, document.Models, "Envelope")
	if field := findField(t, envelope, "payload"); field.Type.Kind != contract.KindAlias || field.Type.Name != "JSONValue" {
		t.Fatalf("unexpected payload field: %#v", field)
	}
	if field := findField(t, envelope, "metadata"); field.Type.Kind != contract.KindMap || field.Type.Items == nil || field.Type.Items.Kind != contract.KindAny {
		t.Fatalf("unexpected metadata field: %#v", field)
	}
	if field := findField(t, envelope, "detail"); field.Type.Kind != contract.KindAny || !field.Type.Nullable {
		t.Fatalf("unexpected detail field: %#v", field)
	}
}

func parseFixture(t *testing.T) openapi.Document {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "openapi", "hostero.openapi.json"))
	if err != nil {
		t.Fatalf("read hostero.openapi.json: %v", err)
	}
	parsed, err := openapi.Parse(source.Document{Bytes: contents})
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

func findAlias(t *testing.T, aliases []contract.Alias, name string) contract.Alias {
	t.Helper()
	for _, alias := range aliases {
		if alias.Name == name {
			return alias
		}
	}
	t.Fatalf("alias %q not found", name)
	return contract.Alias{}
}

func findOperation(t *testing.T, operations []contract.Operation, id string) contract.Operation {
	t.Helper()
	for _, operation := range operations {
		if operation.ID == id {
			return operation
		}
	}
	t.Fatalf("operation %q not found", id)
	return contract.Operation{}
}
