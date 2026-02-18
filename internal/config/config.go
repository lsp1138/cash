// Package config manages user-level settings stored in ~/.cash/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the freelancer's own details used on invoice PDFs.
type Config struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

// Load reads the config file, returning an empty Config if it doesn't exist.
func Load() (Config, error) {
	p, err := filePath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	return c, json.Unmarshal(data, &c)
}

// Save persists the config to disk.
func Save(c Config) error {
	p, err := filePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func filePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cash")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
