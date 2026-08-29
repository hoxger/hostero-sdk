package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/bootstrap"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestBuildAndRenderFixture(t *testing.T) {
	document, err := Build(fixtureContract(t))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(document.Modules) != 3 {
		t.Fatalf("module count = %d, want 3", len(document.Modules))
	}
	models := document.Modules[2].Models
	if models[1].Name != "GameServerListItem" || models[1].Fields[0].Type != "str | None" || !models[1].Fields[0].Required {
		t.Fatalf("unexpected nullable model field: %#v", models[1].Fields[0])
	}
	if models[1].Fields[len(models[1].Fields)-1].Type != "GameServerStatus" {
		t.Fatalf("enum field was not resolved: %#v", models[1].Fields)
	}
	modelImports := document.Modules[2].Imports
	if len(modelImports) != 3 || modelImports[1].Module != "dataclasses" || strings.Join(modelImports[1].Names, ",") != "dataclass" {
		t.Fatalf("model imports must contain only used dependencies: %#v", modelImports)
	}

	files, err := Render(document, fixtureMetadata)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, path := range []string{"__init__.py", "enums.py", "models.py"} {
		want, err := os.ReadFile(filepath.Join("testdata", "fixture", path))
		if err != nil {
			t.Fatalf("read golden file %q: %v", path, err)
		}
		if got := string(files[path]); got != string(want) {
			t.Errorf("rendered %s does not match golden file\nwant:\n%s\ngot:\n%s", path, want, got)
		}
	}
}

func TestBuildMapsNamesAndRejectsCollisions(t *testing.T) {
	if name, err := className("APIKey"); err != nil || name != "ApiKey" {
		t.Fatalf("className(APIKey) = %q, %v", name, err)
	}
	if name, err := fieldName("apiURL"); err != nil || name != "api_url" {
		t.Fatalf("fieldName(apiURL) = %q, %v", name, err)
	}

	document, err := Build(contract.Document{Models: []contract.Model{{
		Name: "server-record",
		Fields: []contract.Field{
			{Name: "class", Required: true, Type: contract.Type{Kind: contract.KindString}},
			{Name: "display-name", Required: false, Type: contract.Type{Kind: contract.KindString}},
		},
	}}})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	model := document.Modules[2].Models[0]
	if model.Name != "ServerRecord" || model.Fields[0].Name != "class_" || model.Fields[0].JSONName != "class" || model.Fields[1].Name != "display_name" || model.Fields[1].Type != "str | None" {
		t.Fatalf("unexpected Python name mapping: %#v", model)
	}
	if imports := document.Modules[2].Imports; len(imports) != 1 || imports[0].Module != "dataclasses" || strings.Join(imports[0].Names, ",") != "dataclass,field" {
		t.Fatalf("renamed fields must request dataclasses.field: %#v", imports)
	}
	files, err := Render(document, fixtureMetadata)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered := string(files["models.py"]); !strings.Contains(rendered, "from dataclasses import dataclass, field") || !strings.Contains(rendered, "class_: str = field(metadata={\"hostero.json_name\": \"class\"})") {
		t.Fatalf("renamed field was not rendered with its required import: %s", rendered)
	}

	_, err = Build(contract.Document{Models: []contract.Model{
		{Name: "game-server"},
		{Name: "game_server"},
	}})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("Build() error = %v, want class collision", err)
	}

	_, err = Build(contract.Document{Models: []contract.Model{{
		Name: "Server",
		Fields: []contract.Field{
			{Name: "server-id", Type: contract.Type{Kind: contract.KindString}},
			{Name: "server_id", Type: contract.Type{Kind: contract.KindString}},
		},
	}}})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("Build() error = %v, want field collision", err)
	}
}

func TestBuildRendersJSONTypeAliases(t *testing.T) {
	jsonValue := contract.Type{Kind: contract.KindAlias, Name: "JSONValue"}
	document, err := Build(contract.Document{
		Aliases: []contract.Alias{
			{
				Name: "JSONScalar",
				Type: contract.Type{
					Kind:     contract.KindUnion,
					Nullable: true,
					Values: []contract.Type{
						{Kind: contract.KindString},
						{Kind: contract.KindInteger},
						{Kind: contract.KindBoolean},
					},
				},
			},
			{
				Name: "JSONValue",
				Type: contract.Type{Kind: contract.KindUnion, Values: []contract.Type{
					{Kind: contract.KindAlias, Name: "JSONScalar"},
					{Kind: contract.KindMap, Items: &jsonValue},
					{Kind: contract.KindArray, Items: &jsonValue},
				}},
			},
		},
		Models: []contract.Model{{
			Name: "Envelope",
			Fields: []contract.Field{
				{Name: "payload", Required: true, Type: jsonValue},
				{Name: "metadata", Required: false, Type: contract.Type{Kind: contract.KindMap, Items: &contract.Type{Kind: contract.KindAny}}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	files, err := Render(document, fixtureMetadata)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	types := string(files["types.py"])
	if !strings.Contains(types, "from typing import TypeAlias") || !strings.Contains(types, "JsonScalar: TypeAlias = str | int | bool | None") || !strings.Contains(types, "JsonValue: TypeAlias = \"JsonScalar | dict[str, JsonValue] | list[JsonValue]\"") {
		t.Fatalf("unexpected rendered type aliases: %s", types)
	}
	models := string(files["models.py"])
	if !strings.Contains(models, "from typing import Any") || !strings.Contains(models, "from .types import JsonValue") || strings.Contains(models, "JsonScalar") || !strings.Contains(models, "metadata: dict[str, Any] | None = None") {
		t.Fatalf("unexpected rendered model types: %s", models)
	}
	init := string(files["__init__.py"])
	if !strings.Contains(init, "from .types import JsonScalar, JsonValue") || !strings.Contains(init, `"JsonValue",`) {
		t.Fatalf("generated aliases are not exported: %s", init)
	}
}

func TestRenderRejectsInvalidGenerationMetadata(t *testing.T) {
	_, err := Render(Document{}, GenerationMetadata{DevKitVersion: "test"})
	if err == nil || !strings.Contains(err.Error(), "OpenAPI source") {
		t.Fatalf("Render() error = %v, want missing metadata rejection", err)
	}
	_, err = Render(Document{}, GenerationMetadata{
		DevKitVersion: "test\ninvalid",
		OpenAPISource: "https://api.example.test/openapi.json",
		Release:       "mvp",
		SHA256:        "abc",
	})
	if err == nil || !strings.Contains(err.Error(), "DevKit version") {
		t.Fatalf("Render() error = %v, want newline metadata rejection", err)
	}
}

func fixtureContract(t *testing.T) contract.Document {
	t.Helper()
	parsed, err := openapi.Parse(source.Document{Bytes: bootstrap.OpenAPI()})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	document, err := contract.Build(parsed)
	if err != nil {
		t.Fatalf("build fixture contract: %v", err)
	}
	return document
}

var fixtureMetadata = GenerationMetadata{
	DevKitVersion: "test",
	OpenAPISource: "https://api.example.test/openapi.json",
	Release:       "mvp",
	SHA256:        "03c46244b097e10690b86591c5619747855cc1a6cb7f76214c99daeb8648f4d4",
}
