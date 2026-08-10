package browserauth

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
)

func TestFindExecutableAcceptsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome.exe")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := FindExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path = %q", got)
	}
}

func TestParseJWT(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, now.Add(time.Hour).Unix())))
	token := "header." + payload + ".signature"
	info, err := ParseJWT(token, now)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry = %s", info.ExpiresAt)
	}
}

func TestParseJWTRejectsExpiredToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, now.Add(-time.Minute).Unix())))
	if _, err := ParseJWT("header."+payload+".signature", now); err == nil {
		t.Fatal("expired token should be rejected")
	}
}

func TestNormalizeJSONEncodedBearerToken(t *testing.T) {
	got, err := NormalizeToken(`"Bearer header.payload.signature"`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "header.payload.signature" {
		t.Fatalf("token = %q", got)
	}
}

func TestNormalizeCryptoJSStorageToken(t *testing.T) {
	want := "header.payload.signature"
	encrypted := encryptCryptoJSForTest(t, `"`+want+`"`, "mhrs-secret-key", []byte("12345678"))
	got, err := NormalizeToken(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("token = %q", got)
	}
}

func encryptCryptoJSForTest(t *testing.T, plaintext, passphrase string, salt []byte) string {
	t.Helper()
	key, iv := evpBytesToKey([]byte(passphrase), salt, 32, aes.BlockSize)
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append([]byte(plaintext), make([]byte, padding)...)
	for index := len(plaintext); index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	raw := append(append([]byte("Salted__"), salt...), ciphertext...)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestMHRSHost(t *testing.T) {
	for _, host := range []string{"mhrs.gov.tr", "prd.mhrs.gov.tr"} {
		if !isMHRSHost(host) {
			t.Fatalf("host %q should be accepted", host)
		}
	}
	for _, host := range []string{"example.com", "mhrs.gov.tr.example.com"} {
		if isMHRSHost(host) {
			t.Fatalf("host %q should be rejected", host)
		}
	}
}

func TestTokenFromCookiesOnlyAcceptsMHRSToken(t *testing.T) {
	cookies := []*network.Cookie{
		{Name: "token-v", Value: "wrong", Domain: ".example.com"},
		{Name: "other", Value: "wrong", Domain: ".mhrs.gov.tr"},
		{Name: "token-v", Value: "jwt-value", Domain: ".mhrs.gov.tr"},
	}
	if got := tokenFromCookies(cookies); got != "jwt-value" {
		t.Fatalf("token = %q", got)
	}
}
