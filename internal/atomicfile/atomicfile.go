package atomicfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Pending struct {
	temporaryPath string
	destination   string
}

func Prepare(path string, data []byte, mode os.FileMode) (_ *Pending, err error) {
	return PrepareReader(path, bytes.NewReader(data), mode)
}

func PrepareReader(path string, source io.Reader, mode os.FileMode) (_ *Pending, err error) {
	directory := filepath.Dir(filepath.Clean(path))
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err != nil {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return nil, fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := io.Copy(temporary, source); err != nil {
		return nil, fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return nil, fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return nil, fmt.Errorf("close temporary file: %w", err)
	}
	return &Pending{temporaryPath: temporaryPath, destination: path}, nil
}

func (pending *Pending) Commit() error {
	if pending.temporaryPath == "" {
		return nil
	}
	if err := replace(pending.temporaryPath, pending.destination); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	pending.temporaryPath = ""
	return nil
}

func (pending *Pending) Cleanup() {
	if pending.temporaryPath != "" {
		_ = os.Remove(pending.temporaryPath)
		pending.temporaryPath = ""
	}
}

func Write(path string, data []byte, mode os.FileMode) error {
	pending, err := Prepare(path, data, mode)
	if err != nil {
		return err
	}
	defer pending.Cleanup()
	return pending.Commit()
}
