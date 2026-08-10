//go:build windows

package sessioncache

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestEncryptedCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.bin")
	want := "header.payload.signature"
	if err := saveTo(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("token = %q", got)
	}
}

func TestMissingCache(t *testing.T) {
	_, err := loadFrom(filepath.Join(t.TempDir(), "missing.bin"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
