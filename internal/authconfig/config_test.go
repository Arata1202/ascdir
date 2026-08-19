package authconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	configHome := t.TempDir()
	isolateUserConfigDir(t, configHome)
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
	isolateUserConfigDir(t, t.TempDir())
	if _, err := Load(); err == nil {
		t.Fatal("missing config loaded successfully")
	}
}

func TestRemove(t *testing.T) {
	configHome := t.TempDir()
	isolateUserConfigDir(t, configHome)
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(Config{IssuerID: "issuer", KeyID: "key", PrivateKeyPath: keyPath}); err != nil {
		t.Fatal(err)
	}
	if _, removed, err := Remove(); err != nil || !removed {
		t.Fatalf("Remove() = removed %v, error %v", removed, err)
	}
	if _, removed, err := Remove(); err != nil || removed {
		t.Fatalf("second Remove() = removed %v, error %v", removed, err)
	}
}

func isolateUserConfigDir(t *testing.T, directory string) {
	t.Helper()
	// os.UserConfigDir uses a different environment variable on each OS.
	// Set all supported inputs so tests can never fall back to a real user's
	// credentials directory when they run on another platform.
	t.Setenv("HOME", directory)
	t.Setenv("XDG_CONFIG_HOME", directory)
	t.Setenv("AppData", directory)
}
