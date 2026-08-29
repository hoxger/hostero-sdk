package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/lock"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestUpdateFetchesAndPinsOpenAPIContract(t *testing.T) {
	rawOpenAPI := readHosteroOpenAPI(t)
	var rawMeta struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(rawOpenAPI, &rawMeta); err != nil {
		t.Fatalf("unmarshal raw openapi: %v", err)
	}
	updatedSpecification := bytes.Replace(
		rawOpenAPI,
		[]byte(`"version": "`+rawMeta.Info.Version+`"`),
		[]byte(`"version": "vtest.1"`),
		1,
	)
	updatedSpecification = bytes.Replace(
		updatedSpecification,
		[]byte(`"version":"`+rawMeta.Info.Version+`"`),
		[]byte(`"version":"vtest.1"`),
		1,
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/openapi.json" {
			t.Errorf("request path = %q, want /v1/openapi.json", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(updatedSpecification)
	}))
	t.Cleanup(server.Close)

	workingDirectory := t.TempDir()
	configuration := config.New(config.DefaultTarget())
	configuration.OpenAPI.Source.URL = server.URL + "/v1/openapi.json"
	if err := config.WriteNew(filepath.Join(workingDirectory, config.FileName), configuration); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	bootstrapDocument := source.Document{Bytes: rawOpenAPI}
	if err := source.WriteSnapshot(workingDirectory, configuration.OpenAPI, bootstrapDocument); err != nil {
		t.Fatalf("write bootstrap snapshot: %v", err)
	}
	pinned, err := source.Resolve(workingDirectory, configuration.OpenAPI)
	if err != nil {
		t.Fatalf("resolve bootstrap snapshot: %v", err)
	}
	if err := lock.WriteNew(filepath.Join(workingDirectory, lock.FileName), lock.New(pinned)); err != nil {
		t.Fatalf("write bootstrap lock: %v", err)
	}

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	var output bytes.Buffer
	command := NewRootCommand("test", strings.NewReader(""), &output, &bytes.Buffer{})
	command.SetArgs([]string{"update"})
	if err := command.Execute(); err != nil {
		t.Fatalf("run update: %v", err)
	}

	updated, err := source.Resolve(workingDirectory, configuration.OpenAPI)
	if err != nil {
		t.Fatalf("resolve updated snapshot: %v", err)
	}
	if updated.Release != "vtest.1" {
		t.Fatalf("updated release = %q, want vtest.1", updated.Release)
	}
	updatedLock, err := lock.Load(filepath.Join(workingDirectory, lock.FileName))
	if err != nil {
		t.Fatalf("load updated lock: %v", err)
	}
	if err := updatedLock.Verify(updated); err != nil {
		t.Fatalf("verify updated lock: %v", err)
	}
	if !strings.Contains(output.String(), "release vtest.1") {
		t.Fatalf("unexpected update output: %q", output.String())
	}
}

func TestUpdateRejectsContractWithoutRequiredPermissions(t *testing.T) {
	rawOpenAPI := readHosteroOpenAPI(t)
	updatedSpecification := bytes.Replace(
		rawOpenAPI,
		[]byte(`"x-hostero-required-permissions"`),
		[]byte(`"x-hostero-required-permissions-removed"`),
		1,
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(updatedSpecification)
	}))
	t.Cleanup(server.Close)

	workingDirectory := t.TempDir()
	configuration := config.New(config.DefaultTarget())
	configuration.OpenAPI.Source.URL = server.URL
	if err := config.WriteNew(filepath.Join(workingDirectory, config.FileName), configuration); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	bootstrapDocument := source.Document{Bytes: rawOpenAPI}
	if err := source.WriteSnapshot(workingDirectory, configuration.OpenAPI, bootstrapDocument); err != nil {
		t.Fatalf("write bootstrap snapshot: %v", err)
	}
	pinned, err := source.Resolve(workingDirectory, configuration.OpenAPI)
	if err != nil {
		t.Fatalf("resolve bootstrap snapshot: %v", err)
	}
	if err := lock.WriteNew(filepath.Join(workingDirectory, lock.FileName), lock.New(pinned)); err != nil {
		t.Fatalf("write bootstrap lock: %v", err)
	}

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	command := NewRootCommand("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"update"})
	err = command.Execute()
	if err == nil || !strings.Contains(err.Error(), "x-hostero-required-permissions is required") {
		t.Fatalf("update error = %v, want missing permissions", err)
	}

	after, err := source.Resolve(workingDirectory, configuration.OpenAPI)
	if err != nil {
		t.Fatalf("resolve snapshot after failed update: %v", err)
	}
	if after.SHA256 != pinned.SHA256 {
		t.Fatalf("snapshot changed after failed update: %s != %s", after.SHA256, pinned.SHA256)
	}
}

func readHosteroOpenAPI(t *testing.T) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "openapi", "hostero.openapi.json"))
	if err != nil {
		t.Fatalf("read hostero.openapi.json: %v", err)
	}
	return contents
}
