package appstore

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

type Credentials struct {
	IssuerID string
	KeyID    string
	Key      *ecdsa.PrivateKey
}

func CredentialsFromEnv() (Credentials, error) {
	issuerID := os.Getenv("ASC_ISSUER_ID")
	keyID := os.Getenv("ASC_KEY_ID")
	keyPath := os.Getenv("ASC_PRIVATE_KEY_PATH")
	if issuerID == "" || keyID == "" || keyPath == "" {
		return Credentials{}, errors.New("ASC_ISSUER_ID, ASC_KEY_ID, and ASC_PRIVATE_KEY_PATH are required")
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return Credentials{}, errors.New("private key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return Credentials{}, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve.Params().Name != "P-256" {
		return Credentials{}, errors.New("private key must be an EC P-256 key")
	}
	return Credentials{IssuerID: issuerID, KeyID: keyID, Key: key}, nil
}

func (c Credentials) Token(now time.Time) (string, error) {
	header := map[string]string{"alg": "ES256", "kid": c.KeyID, "typ": "JWT"}
	claims := map[string]any{
		"iss": c.IssuerID, "iat": now.Unix(), "exp": now.Add(19 * time.Minute).Unix(),
		"aud": "appstoreconnect-v1",
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	unsigned := encodeSegment(headerJSON) + "." + encodeSegment(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, c.Key, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	signature := append(paddedBytes(r, 32), paddedBytes(s, 32)...)
	return unsigned + "." + encodeSegment(signature), nil
}

func encodeSegment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func paddedBytes(value *big.Int, size int) []byte {
	result := make([]byte, size)
	bytes := value.Bytes()
	copy(result[size-len(bytes):], bytes)
	return result
}
