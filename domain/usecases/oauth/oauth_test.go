package oauth

import (
	"strings"
	"testing"
)

func TestNormalizeRequestedScopesPreservesOrderAndRejectsUnknown(t *testing.T) {
	scopes, derr := normalizeRequestedScopes("write read read follow", defaultScopes)
	if derr != nil {
		t.Fatalf("normalize scopes: %v", derr)
	}
	if scopes != "write read follow" {
		t.Fatalf("expected scopes to preserve request order and dedupe, got %q", scopes)
	}

	if _, derr := normalizeRequestedScopes("read admin", defaultScopes); derr == nil {
		t.Fatalf("expected unsupported scope to fail")
	}
}

func TestEnsureScopesAllowedAllowsRegisteredParentScope(t *testing.T) {
	if derr := ensureScopesAllowed("read:statuses write:media", "read write"); derr != nil {
		t.Fatalf("parent scopes should allow narrower scopes: %v", derr)
	}

	derr := ensureScopesAllowed("write", "read follow")
	if derr == nil || !strings.Contains(derr.Message, "not registered") {
		t.Fatalf("expected unregistered write scope to fail, got %v", derr)
	}
}
