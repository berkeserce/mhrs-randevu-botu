package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	const tokenKey = "MHRS_TEST_ENVFILE_TOKEN"
	const chatKey = "MHRS_TEST_ENVFILE_CHAT"
	t.Setenv(tokenKey, "")
	t.Setenv(chatKey, "")
	_ = os.Unsetenv(tokenKey)
	_ = os.Unsetenv(chatKey)

	path := filepath.Join(t.TempDir(), ".env")
	contents := "# test\n" + tokenKey + "='secret:value'\nexport " + chatKey + "=12345\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !loaded {
		t.Fatalf("loaded = %v, err = %v", loaded, err)
	}
	if os.Getenv(tokenKey) != "secret:value" || os.Getenv(chatKey) != "12345" {
		t.Fatalf("token = %q, chat = %q", os.Getenv(tokenKey), os.Getenv(chatKey))
	}
}

func TestLoadKeepsExistingEnvironment(t *testing.T) {
	const key = "MHRS_TEST_ENVFILE_EXISTING"
	t.Setenv(key, "environment")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(key+"=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(key) != "environment" {
		t.Fatalf("existing environment was overwritten: %q", os.Getenv(key))
	}
}

func TestLoadMissingFileIsOptional(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil || loaded {
		t.Fatalf("loaded = %v, err = %v", loaded, err)
	}
}
