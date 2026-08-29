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
	if len(document.Models) != 6 || len(document.Enums) != 1 || len(document.Aliases) != 0 {
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
				model := parsed.Specification.Components.Schemas["GameServerListItem"].Value
				model.Properties["name"] = &openapi3.SchemaRef{Value: openapi3.NewObjectSchema()}
			},
			message: "anonymous object schemas are not supported",
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
