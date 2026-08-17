package appstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
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

func TestCredentialsFromEnv(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASC_ISSUER_ID", " issuer ")
	t.Setenv("ASC_KEY_ID", " key-id ")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	credentials, err := CredentialsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if credentials.IssuerID != "issuer" || credentials.KeyID != "key-id" || credentials.Key == nil {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestCredentialsFromEnvRejectsMissingAndInvalidKeys(t *testing.T) {
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	if _, err := CredentialsFromEnv(); err == nil {
		t.Fatal("missing credentials succeeded")
	}
	t.Setenv("ASC_ISSUER_ID", "issuer")
	t.Setenv("ASC_KEY_ID", "key")
	keyPath := filepath.Join(t.TempDir(), "invalid.p8")
	if err := os.WriteFile(keyPath, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	if _, err := CredentialsFromEnv(); err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrivateKeyPermissionWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, []byte("key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if warning := PrivateKeyPermissionWarning(keyPath); !strings.Contains(warning, "0600") {
		t.Fatalf("warning = %q", warning)
	}
	if err := os.Chmod(keyPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if warning := PrivateKeyPermissionWarning(keyPath); warning != "" {
		t.Fatalf("warning = %q", warning)
	}
}
