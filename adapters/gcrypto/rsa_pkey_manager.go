package gcrypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"

	ports "github.com/myfedi/gargoyle/domain/ports/gcrypto"
)

const keyBits = 2048

type PublicRSAKey struct {
	publicKey *rsa.PublicKey
}

func (p PublicRSAKey) ToPEM() []byte {
	derBytes, err := x509.MarshalPKIXPublicKey(p.publicKey)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	})
}

func (p PublicRSAKey) VerifySignature(data []byte, sig []byte) error {
	hashed := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(p.publicKey, crypto.SHA256, hashed[:], sig)
}

// type check public key
var _ ports.PublicKey = PublicRSAKey{}

type PrivateRSAKey struct {
	privateKey *rsa.PrivateKey
}

func (p PrivateRSAKey) ToPEM() []byte {
	derBytes := x509.MarshalPKCS1PrivateKey(p.privateKey)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derBytes,
	})
}

func (p PrivateRSAKey) Sign(data []byte) ([]byte, error) {
	hashed := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, hashed[:])
}

// type check private key
var _ ports.PrivateKey = PrivateRSAKey{}

type RsaPKey struct {
	privateKey PrivateRSAKey
	publicKey  PublicRSAKey
}

func (k RsaPKey) PrivateKey() ports.PrivateKey {
	return k.privateKey
}
func (k RsaPKey) PublicKey() ports.PublicKey {
	return k.publicKey
}

// type check key pair
var _ ports.PKeyPair = &RsaPKey{}

type RsaPKeyManager struct{}

func NewRsaPKeyManager() RsaPKeyManager {
	return RsaPKeyManager{}
}

// type check rsa key manager
var _ ports.PKeyManager = RsaPKeyManager{}

func (m RsaPKeyManager) CreatePKeyPair(email string) (ports.PKeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, err
	}

	return RsaPKey{
		privateKey: PrivateRSAKey{privateKey: key},
		publicKey:  PublicRSAKey{publicKey: &key.PublicKey},
	}, nil
}

func (m RsaPKeyManager) PublicKeyFromPEM(pemStr string) (ports.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid PEM block for public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return PublicRSAKey{publicKey: rsaPub}, nil
}

func (m RsaPKeyManager) PrivateKeyFromPEM(pemStr string) (ports.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("invalid PEM block for private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return PrivateRSAKey{privateKey: key}, nil
}
