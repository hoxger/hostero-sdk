package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/openapi"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestBuildAndRenderPinnedContract(t *testing.T) {
	document, err := Build(fixtureContract(t))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(document.Modules) != 7 {
		t.Fatalf("module count = %d, want 7", len(document.Modules))
	}

	files, err := Render(document, fixtureMetadata)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	modelsCode := string(files["models.py"])
	if !strings.Contains(modelsCode, "class GameServerListItemResource:") ||
		!strings.Contains(modelsCode, "def _from_dict(cls, data: Mapping[str, Any])") ||
		!strings.Contains(modelsCode, "def _to_dict(self) -> dict[str, Any]:") {
		t.Fatalf("models.py missing expected model or codec: %s", modelsCode)
	}

	servicesCode := string(files["services.py"])
	if !strings.Contains(servicesCode, "class _GeneratedServicesMixin:") ||
		!strings.Contains(servicesCode, "class ServersPowerService:") ||
		!strings.Contains(servicesCode, "class ServersFilesContentsService:") ||
		!strings.Contains(servicesCode, "class TicketsMessagesAttachmentsService:") {
		t.Fatalf("services.py missing expected services: %s", servicesCode)
	}
	resourcesCode := string(files["resources.py"])
	if !strings.Contains(resourcesCode, "class GameServerHandle(Generic[TGameServerData]):") ||
		!strings.Contains(resourcesCode, "class GameServer(GameServerHandle[GameServerDetailResource]):") ||
		!strings.Contains(resourcesCode, "class GameServerListItem(GameServerHandle[GameServerListItemResource]):") ||
		!strings.Contains(resourcesCode, "class GameServerPage:") ||
		!strings.Contains(resourcesCode, "class GameServerSubusers:") ||
		!strings.Contains(resourcesCode, "def subusers(self) -> GameServerSubusers:") ||
		!strings.Contains(resourcesCode, "return self._service.subusers.list(self._resource_id)") ||
		strings.Contains(resourcesCode, "class _BoundService:") ||
		strings.Contains(resourcesCode, "def __getattr__(self, name: str) -> Any:") {
		t.Fatalf("resources.py missing expected resource wrappers: %s", resourcesCode)
	}
	if strings.Count(resourcesCode, "class GameServerSubusers:") != 1 || strings.Count(resourcesCode, "def subusers(self) -> GameServerSubusers:") != 1 {
		t.Fatalf("resources.py duplicated shared game-server handle members: %s", resourcesCode)
	}

	initCode := string(files["__init__.py"])
	if !strings.Contains(initCode, "_GeneratedServicesMixin") ||
		!strings.Contains(initCode, "RedirectResponse") ||
		!strings.Contains(initCode, "GameServerPage") {
		t.Fatalf("__init__.py missing expected exports: %s", initCode)
	}
}

func TestBuildMapsNamesAndRejectsCollisions(t *testing.T) {
	if name, err := className("APIKey"); err != nil || name != "ApiKey" {
		t.Fatalf("className(APIKey) = %q, %v", name, err)
	}
	if name, err := fieldName("apiURL"); err != nil || name != "api_url" {
		t.Fatalf("fieldName(apiURL) = %q, %v", name, err)
	}
	if name, err := methodName("get-active"); err != nil || name != "get_active" {
		t.Fatalf("methodName(get-active) = %q, %v", name, err)
	}
	if name, err := serviceClassName([]string{"servers", "files", "contents"}); err != nil || name != "ServersFilesContentsService" {
		t.Fatalf("serviceClassName = %q, %v", name, err)
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
	if imports := document.Modules[2].Imports; len(imports) < 2 {
		t.Fatalf("expected imports in models.py: %#v", imports)
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
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "openapi", "hostero.openapi.json"))
	if err != nil {
		t.Fatalf("read hostero.openapi.json: %v", err)
	}
	parsed, err := openapi.Parse(source.Document{Bytes: contents})
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
	Release:       "vdev.2026.08.29.214457",
	SHA256:        "0e8cdff76c3568eff2a4d414341c9e052b4d8f943d96295981b2dea8dca03e07",
}
