package envfile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Load(path string) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ortam dosyasi acilamadi: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, rawValue, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || !validKey(key) {
			return false, fmt.Errorf("ortam dosyasinda gecersiz satir: %d", lineNumber)
		}
		value, err := parseValue(strings.TrimSpace(rawValue))
		if err != nil {
			return false, fmt.Errorf("ortam dosyasinda gecersiz deger, satir %d", lineNumber)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return false, fmt.Errorf("ortam degiskeni ayarlanamadi, satir %d", lineNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("ortam dosyasi okunamadi: %w", err)
	}
	return true, nil
}

func parseValue(value string) (string, error) {
	if value == "" || value[0] != '\'' && value[0] != '"' {
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != value[0] {
		return "", errors.New("kapanmayan tirnak")
	}
	if value[0] == '\'' {
		return value[1 : len(value)-1], nil
	}
	return strconv.Unquote(value)
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for index, character := range key {
		if character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}
