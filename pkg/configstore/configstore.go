// Package configstore manages the CLI's persisted configuration.
// The config file lives at $XDG_CONFIG_HOME/kleido/config.yaml
// (or ~/.config/kleido/config.yaml if XDG_CONFIG_HOME is not set).
// File permissions are enforced to 0600 after every write.
package configstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is what is persisted to disk.
// All fields are optional — a fresh config file is valid.
type Config struct {
	APIURL      string    `yaml:"api_url"`
	AccessToken string    `yaml:"access_token"`
	ExpiresAt   time.Time `yaml:"expires_at"`
}

// Dir returns the platform config directory for kleido.
// Uses $XDG_CONFIG_HOME if set, otherwise ~/.config.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("configstore: get home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "kleido"), nil
}

// Path returns the full path to the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config file from disk.
// Returns an empty Config (not an error) if the file does not exist.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // path is user-controlled intentionally
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("configstore: read: %w", err)
	}
	var cfg Config
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("configstore: unmarshal: %w", unmarshalErr)
	}
	return &cfg, nil
}

// Save writes cfg to disk and sets file permissions to 0600.
// Creates the config directory (mode 0700) if it does not exist.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("configstore: mkdir: %w", err)
	}
	p := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(cfg) //nolint:gosec // G117: marshaling user config intentionally
	if err != nil {
		return fmt.Errorf("configstore: marshal: %w", err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil { //nolint:gosec // 0600 is intentional
		return fmt.Errorf("configstore: write: %w", err)
	}
	// Explicitly chmod after write to override any umask interference.
	if err := os.Chmod(p, 0600); err != nil { //nolint:gosec // 0600 is intentional
		return fmt.Errorf("configstore: chmod: %w", err)
	}
	return nil
}

// ClearToken zeroes the token fields and saves the config file.
func ClearToken() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.AccessToken = ""
	cfg.ExpiresAt = time.Time{}
	return Save(cfg)
}
