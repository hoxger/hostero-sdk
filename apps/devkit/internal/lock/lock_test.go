package lock

import (
	"strings"
	"testing"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/source"
)

func TestVerifyRequiresLockedReleaseAndDigest(t *testing.T) {
	document := source.Document{Release: "mvp", SHA256: strings.Repeat("a", 64)}
	file := New(document)
	if err := file.Verify(document); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if err := file.Verify(source.Document{Release: "next", SHA256: document.SHA256}); err == nil || !strings.Contains(err.Error(), "release") {
		t.Fatalf("Verify() error = %v, want release mismatch", err)
	}
	if err := file.Verify(source.Document{Release: document.Release, SHA256: strings.Repeat("b", 64)}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestValidateRejectsInvalidDigest(t *testing.T) {
	err := (File{Version: 1, OpenAPI: OpenAPI{Release: "mvp", SHA256: "not-a-digest"}}).Validate()
	if err == nil || !strings.Contains(err.Error(), "lowercase SHA-256") {
		t.Fatalf("Validate() error = %v, want digest validation", err)
	}
}
