package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/bootstrap"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/config"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/lock"
	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestUpdateFetchesAndPinsOpenAPIContract(t *testing.T) {
	updatedSpecification := bytes.Replace(
		bootstrap.OpenAPI(),
		[]byte(`"version": "mvp"`),
		[]byte(`"version": "vtest.1"`),
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
	bootstrapDocument := source.Document{Bytes: bootstrap.OpenAPI()}
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
