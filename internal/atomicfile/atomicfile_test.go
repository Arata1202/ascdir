package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteReplacesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "value.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("contents = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not implement Unix permission bits. The write and replace
	// behavior above is still exercised there.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestPrepareDoesNotReplaceUntilCommit(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "value.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	pending, err := Prepare(path, []byte("new"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Cleanup()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("file changed before commit: %q", data)
	}
	if err := pending.Commit(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("file after commit = %q", data)
	}
}

func TestPendingCleanupDiscardsStagedFile(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "value.txt")
	pending, err := Prepare(path, []byte("new"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	pending.Cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("destination exists after cleanup: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".value.txt-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %#v", matches)
	}
}
