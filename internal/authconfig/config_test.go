package authconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	writtenPath, err := Save(Config{IssuerID: " issuer ", KeyID: " key ", PrivateKeyPath: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(writtenPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode = %o, want 600", info.Mode().Perm())
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IssuerID != "issuer" || loaded.KeyID != "key" || loaded.PrivateKeyPath != keyPath {
		t.Fatalf("loaded config = %#v", loaded)
	}
}

func TestLoadMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	if _, err := Load(); err == nil {
		t.Fatal("missing config loaded successfully")
	}
}
