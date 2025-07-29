package gcrypto

// PublicKey is a key that can be represented as PEM and can be used
// to verify signatures
type PublicKey interface {
	// ToPEM returns a PEM representation of the public key
	ToPEM() []byte
	// VerifySignature verifies the signature of data
	VerifySignature(data []byte, sig []byte) error
}

// PrivateKey is a key that can be represented as PEM and can be used
// to sign data, which can then be verified with a public key
type PrivateKey interface {
	// ToPEM returns a PEM representation of the public key
	ToPEM() []byte
	// Sign signs data
	Sign(data []byte) ([]byte, error)
}

// PKeyPair represents a pair of public and private key
type PKeyPair interface {
	PrivateKey() PrivateKey
	PublicKey() PublicKey
}

// PKeyManager provides capabilities to create a key pair or construct private/public
// keys from their PEM representations
type PKeyManager interface {
	CreatePKeyPair(email string) (PKeyPair, error)
	PublicKeyFromPEM(pemStr string) (PublicKey, error)
	PrivateKeyFromPEM(pemStr string) (PrivateKey, error)
}
