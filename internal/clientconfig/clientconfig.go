// Package clientconfig reads and writes the desktop client's connection
// settings: which server to talk to and the bearer token it accepts. The
// file lives in the platform user-config directory and is private to the
// user.
package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the on-disk client.json. An empty ServerURL means "run the
// server inside this process".
type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

// DataDir is the per-user directory holding the database and this config.
func DataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "存号网络"), nil
}

// DefaultPath is where client.json lives for this user.
func DefaultPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.json"), nil
}

// Load reads the config. A missing file is not an error: it means the
// defaults (embedded server, no token).
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	c.ServerURL = strings.TrimSpace(c.ServerURL)
	c.Token = strings.TrimSpace(c.Token)
	return c, nil
}

// Save writes the config atomically with owner-only permissions.
func Save(path string, c Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(Config{
		ServerURL: strings.TrimSpace(c.ServerURL),
		Token:     strings.TrimSpace(c.Token),
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".client-*.json")
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
