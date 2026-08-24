package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCreatesPrivateConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save(Config{Name: "Example Freelancer", PaymentDetails: "Example payment details"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(filepath.Join(home, ".cash", "config.json"))
	if err != nil {
		t.Fatalf("Stat config.json: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config.json mode: got %04o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Join(home, ".cash"))
	if err != nil {
		t.Fatalf("Stat .cash: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf(".cash mode: got %04o, want 0700", got)
	}
}
