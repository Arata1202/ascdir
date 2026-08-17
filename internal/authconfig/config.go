package authconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Arata1202/ascdir/internal/atomicfile"
)

type Config struct {
	IssuerID       string `json:"issuer_id"`
	KeyID          string `json:"key_id"`
	PrivateKeyPath string `json:"private_key_path"`
}

func Path() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(directory, "ascdir", "credentials.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, errors.New("credentials are not configured; run 'ascdir auth login' or set ASC_ISSUER_ID, ASC_KEY_ID, and ASC_PRIVATE_KEY_PATH")
		}
		return Config{}, fmt.Errorf("read credentials config: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse credentials config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid credentials config: %w", err)
	}
	return config, nil
}

func Save(config Config) (string, error) {
	config.IssuerID = strings.TrimSpace(config.IssuerID)
	config.KeyID = strings.TrimSpace(config.KeyID)
	config.PrivateKeyPath = strings.TrimSpace(config.PrivateKeyPath)
	if err := config.Validate(); err != nil {
		return "", err
	}
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create credentials directory: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode credentials config: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write credentials config: %w", err)
	}
	return path, nil
}

func Remove() (string, bool, error) {
	path, err := Path()
	if err != nil {
		return "", false, err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		return path, false, fmt.Errorf("remove credentials config: %w", err)
	}
	return path, true, nil
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.IssuerID) == "" || strings.TrimSpace(config.KeyID) == "" || strings.TrimSpace(config.PrivateKeyPath) == "" {
		return errors.New("issuer ID, key ID, and private key path are required")
	}
	info, err := os.Stat(config.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("access private key: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("private key path must point to a regular file")
	}
	return nil
}
