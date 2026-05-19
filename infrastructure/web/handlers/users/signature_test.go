package users

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/myfedi/gargoyle/adapters/gcrypto"
	"github.com/myfedi/gargoyle/domain/models"
)

func TestSignOutboundRequestSetsSignatureHeaders(t *testing.T) {
	pair, err := gcrypto.NewRsaPKeyManager().CreatePKeyPair("alice@example.org")
	if err != nil {
		t.Fatalf("create key pair: %v", err)
	}
	privateKey := string(pair.PrivateKey().ToPEM())
	body := []byte(`{"type":"Accept"}`)
	req, err := http.NewRequest(http.MethodPost, "https://remote.example/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	signOutboundRequest(req, body, models.Account{
		URI:        "https://example.org/users/alice",
		PrivateKey: &privateKey,
	})

	if req.Header.Get("Signature") == "" {
		t.Fatal("expected Signature header")
	}
	if req.Header.Get("Digest") == "" {
		t.Fatal("expected Digest header")
	}
	if req.Header.Get("Date") == "" {
		t.Fatal("expected Date header")
	}
}
