package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	const tokenName = "CONSO_DASHBOARD_TEST_TOKEN"
	t.Setenv(tokenName, "")
	if err := os.Unsetenv(tokenName); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# commentaire\n"+tokenName+"='secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(tokenName); got != "secret" {
		t.Fatalf("valeur chargée = %q", got)
	}
}

func TestLoadEnvFilePreservesExistingValue(t *testing.T) {
	const tokenName = "CONSO_DASHBOARD_TEST_PRIORITY"
	t.Setenv(tokenName, "terminal")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(tokenName+"=fichier\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(tokenName); got != "terminal" {
		t.Fatalf("valeur existante remplacée par %q", got)
	}
}

func TestLoadEnvFileAllowsMissingFile(t *testing.T) {
	if err := loadEnvFile(filepath.Join(t.TempDir(), "absent.env")); err != nil {
		t.Fatal(err)
	}
}
