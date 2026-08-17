package appstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestTokenCreatesValidES256JWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	credentials := Credentials{IssuerID: "issuer", KeyID: "key-id", Key: key}
	token, err := credentials.Token(now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}
	decode := func(value string) []byte {
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	var claims map[string]any
	if err := json.Unmarshal(decode(parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	if claims["iss"] != "issuer" || claims["aud"] != "appstoreconnect-v1" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if got := int64(claims["exp"].(float64) - claims["iat"].(float64)); got != int64(19*time.Minute/time.Second) {
		t.Fatalf("token lifetime = %d seconds", got)
	}
	signature := decode(parts[2])
	if len(signature) != 64 {
		t.Fatalf("signature length = %d, want 64", len(signature))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		t.Fatal("JWT signature did not verify")
	}
}
