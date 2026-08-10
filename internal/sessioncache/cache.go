package sessioncache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotFound = errors.New("kayitli MHRS oturumu yok")

const maxCacheSize = 64 << 10

func Path() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("kullanici onbellek klasoru bulunamadi: %w", err)
	}
	return filepath.Join(cacheDir, "mhrs-randevu-botu", "session.bin"), nil
}

func Load() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return loadFrom(path)
}

func Save(token string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return saveTo(path, token)
}

func Remove() error {
	path, err := Path()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("oturum onbellegi silinemedi: %w", err)
	}
	return nil
}

func loadFrom(path string) (string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("oturum onbellegi okunamadi: %w", err)
	}
	if info.IsDir() || info.Size() <= 0 || info.Size() > maxCacheSize {
		return "", errors.New("oturum onbellegi gecersiz")
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("oturum onbellegi okunamadi: %w", err)
	}
	plaintext, err := unprotect(encrypted)
	if err != nil {
		return "", fmt.Errorf("oturum onbellegi cozulemedi: %w", err)
	}
	token := strings.TrimSpace(string(plaintext))
	if token == "" {
		return "", errors.New("oturum onbellegi bos")
	}
	return token, nil
}

func saveTo(path, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("bos JWT onbellege kaydedilemez")
	}
	encrypted, err := protect([]byte(token))
	if err != nil {
		return fmt.Errorf("JWT korunamadi: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("oturum onbellek klasoru olusturulamadi: %w", err)
	}
	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		return fmt.Errorf("oturum onbellegi yazilamadi: %w", err)
	}
	return nil
}
