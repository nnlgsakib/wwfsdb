package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// GenerateKeyPair creates a new ECDSA private and public key pair.
func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// EncodePrivateKey converts an ECDSA private key to a hex string.
func EncodePrivateKey(priv *ecdsa.PrivateKey) string {
	return hex.EncodeToString(priv.D.Bytes())
}

// DecodePrivateKey converts a hex string to an ECDSA private key.
func DecodePrivateKey(hexKey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	priv := new(ecdsa.PrivateKey)
	priv.D = new(big.Int).SetBytes(b)
	priv.PublicKey.Curve = elliptic.P256()
	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(priv.D.Bytes())
	return priv, nil
}

// EncodePublicKey converts an ECDSA public key to a hex string.
func EncodePublicKey(pub *ecdsa.PublicKey) string {
	return hex.EncodeToString(elliptic.Marshal(pub.Curve, pub.X, pub.Y))
}

// DecodePublicKey converts a hex string to an ECDSA public key.
func DecodePublicKey(hexKey string) (*ecdsa.PublicKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), b)
	if x == nil {
		return nil, fmt.Errorf("invalid public key")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// Sign creates a signature for a message using a private key.
func Sign(privHex string, message string) (string, error) {
	priv, err := DecodePrivateKey(privHex)
	if err != nil {
		return "", fmt.Errorf("could not decode private key: %w", err)
	}

	hash := sha256.Sum256([]byte(message))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return "", err
	}

	// Ensure r and s are 32 bytes each
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	if len(rBytes) < 32 {
		rBytes = append(make([]byte, 32-len(rBytes)), rBytes...)
	}
	if len(sBytes) < 32 {
		sBytes = append(make([]byte, 32-len(sBytes)), sBytes...)
	}

	sig := append(rBytes, sBytes...)
	return hex.EncodeToString(sig), nil
}

// Verify checks a signature against a message and a public key.
func Verify(pubHex string, message string, sigHex string) (bool, error) {
	pub, err := DecodePublicKey(pubHex)
	if err != nil {
		return false, fmt.Errorf("could not decode public key: %w", err)
	}

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("could not decode signature: %w", err)
	}
	if len(sig) != 64 {
		return false, fmt.Errorf("invalid signature length")
	}

	hash := sha256.Sum256([]byte(message))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])

	return ecdsa.Verify(pub, hash[:], r, s), nil
}
