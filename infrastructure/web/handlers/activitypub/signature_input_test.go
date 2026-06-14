package activitypub

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSignatureVerificationInputAcceptsAuthorizationSignature(t *testing.T) {
	app := fiber.New()
	app.Post("/inbox", func(c *fiber.Ctx) error {
		input := signatureVerificationInput(c, []byte(`{}`), "https://remote.example/users/bob", nil, true)
		if input.Headers["signature"] != `keyId="https://remote.example/users/bob#main-key"` {
			t.Fatalf("signature = %q", input.Headers["signature"])
		}
		return c.SendStatus(fiber.StatusAccepted)
	})

	req := httptest.NewRequest("POST", "/inbox", nil)
	req.Header.Set("Authorization", `Signature keyId="https://remote.example/users/bob#main-key"`)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
