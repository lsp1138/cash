// Package config manages user-level settings stored in ~/.cash/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the freelancer's own details used on invoice PDFs.
type Config struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	TaxID          string `json:"tax_id"`
	PaymentDetails string `json:"payment_details"`
}

// ApplySettings overlays DB-backed settings onto the config.
func (c *Config) ApplySettings(settings map[string]string) {
	if v := settings["name"]; v != "" {
		c.Name = v
	}
	if v := settings["email"]; v != "" {
		c.Email = v
	}
	if v := settings["phone"]; v != "" {
		c.Phone = v
	}
	if v := settings["address"]; v != "" {
		c.Address = v
	}
	if v := settings["tax_id"]; v != "" {
		c.TaxID = v
	}
	if v := settings["payment_details"]; v != "" {
		c.PaymentDetails = v
	}
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
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

func filePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
