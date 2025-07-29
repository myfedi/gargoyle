package gcrypto

import (
	"testing"
)

func TestRSAKeyPairRoundTrip(t *testing.T) {
	manager := NewRsaPKeyManager()

	pair, err := manager.CreatePKeyPair("test@example.com")
	if err != nil {
		t.Fatalf("failed to create key pair: %v", err)
	}

	priv := pair.PrivateKey()
	pub := pair.PublicKey()

	// Sign and verify
	message := []byte("test message")
	sig, err := priv.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	if err := pub.VerifySignature(message, sig); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

func TestPEMSerialization(t *testing.T) {
	manager := NewRsaPKeyManager()

	pair, err := manager.CreatePKeyPair("test@example.com")
	if err != nil {
		t.Fatalf("failed to create key pair: %v", err)
	}

	origPriv := pair.PrivateKey()
	origPub := pair.PublicKey()

	// Serialize
	privPEM := origPriv.ToPEM()
	pubPEM := origPub.ToPEM()

	// Deserialize
	parsedPriv, err := manager.PrivateKeyFromPEM(string(privPEM))
	if err != nil {
		t.Fatalf("failed to parse private key PEM: %v", err)
	}

	parsedPub, err := manager.PublicKeyFromPEM(string(pubPEM))
	if err != nil {
		t.Fatalf("failed to parse public key PEM: %v", err)
	}

	// Sign and verify using parsed keys
	msg := []byte("hello world")
	sig, err := parsedPriv.Sign(msg)
	if err != nil {
		t.Fatalf("signing failed with parsed key: %v", err)
	}

	if err := parsedPub.VerifySignature(msg, sig); err != nil {
		t.Fatalf("verification failed with parsed key: %v", err)
	}
}

func TestInvalidPEM(t *testing.T) {
	manager := NewRsaPKeyManager()

	_, err := manager.PrivateKeyFromPEM("invalid pem")
	if err == nil {
		t.Fatal("expected error for invalid private key PEM")
	}

	_, err = manager.PublicKeyFromPEM("invalid pem")
	if err == nil {
		t.Fatal("expected error for invalid public key PEM")
	}
}
