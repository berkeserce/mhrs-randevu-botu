package browserauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const LoginURL = "https://mhrs.gov.tr/vatandas/"

const mhrsStoragePassphrase = "mhrs-secret-key"

type JWTInfo struct {
	ExpiresAt time.Time
}

func FindExecutable(preferred string) (string, error) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		if err := validateExecutable(preferred); err != nil {
			return "", err
		}
		return preferred, nil
	}

	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Chromium", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
		}
		for _, candidate := range candidates {
			if filepath.IsAbs(candidate) && validateExecutable(candidate) == nil {
				return candidate, nil
			}
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "chrome", "msedge"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("Chromium tabanli bir tarayici bulunamadi; -chromium-path ile calistirilabilir dosya yolunu verin")
}

func Login(ctx context.Context, executablePath string) (string, error) {
	if err := validateExecutable(executablePath); err != nil {
		return "", err
	}

	profileDir, err := os.MkdirTemp("", "mhrs-browser-auth-")
	if err != nil {
		return "", fmt.Errorf("gecici Chromium profili olusturulamadi: %w", err)
	}
	defer func() { _ = os.RemoveAll(profileDir) }()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(executablePath),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAllocator()
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx, chromedp.Navigate(LoginURL)); err != nil {
		return "", fmt.Errorf("Chromium baslatilamadi: %w", err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("tarayici girisi tamamlanmadi: %w", ctx.Err())
		case <-ticker.C:
			token, err := readMHRSToken(browserCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return "", errors.New("Chromium penceresi giris tamamlanmadan kapandi")
				}
				continue
			}
			if token == "" {
				continue
			}
			token, err = NormalizeToken(token)
			if err != nil {
				return "", fmt.Errorf("MHRS tokeni normalize edilemedi: %w", err)
			}
			if _, err := ParseJWT(token, time.Now()); err != nil {
				return "", fmt.Errorf("MHRS gecersiz bir token olusturdu: %w", err)
			}
			if err := chromedp.Cancel(browserCtx); err != nil && !errors.Is(err, context.Canceled) {
				return "", fmt.Errorf("JWT alindi ancak Chromium duzgun kapatilamadi: %w", err)
			}
			return token, nil
		}
	}
}

func NormalizeToken(raw string) (string, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", errors.New("token bos")
	}
	if strings.HasPrefix(token, "U2FsdGVkX1") {
		decrypted, err := decryptCryptoJSPassphrase(token, mhrsStoragePassphrase)
		if err != nil {
			return "", fmt.Errorf("MHRS storage tokeni cozulemedi: %w", err)
		}
		token = strings.TrimSpace(decrypted)
	}
	if strings.HasPrefix(token, `"`) {
		var decoded string
		if err := json.Unmarshal([]byte(token), &decoded); err != nil {
			return "", errors.New("JSON-string token okunamadi")
		}
		token = strings.TrimSpace(decoded)
	}
	if len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return "", errors.New("normalize edilen token bos")
	}
	return token, nil
}

func decryptCryptoJSPassphrase(encoded, passphrase string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("CryptoJS verisi base64 olarak okunamadi")
	}
	if len(raw) < 16 || string(raw[:8]) != "Salted__" {
		return "", errors.New("CryptoJS OpenSSL salt basligi yok")
	}
	ciphertext := raw[16:]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("CryptoJS AES sifreli veri uzunlugu gecersiz")
	}
	key, iv := evpBytesToKey([]byte(passphrase), raw[8:16], 32, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.New("AES anahtari olusturulamadi")
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	padding := int(plaintext[len(plaintext)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(plaintext) {
		return "", errors.New("CryptoJS PKCS7 dolgusu gecersiz")
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			return "", errors.New("CryptoJS PKCS7 dolgusu bozuk")
		}
	}
	return string(plaintext[:len(plaintext)-padding]), nil
}

func evpBytesToKey(passphrase, salt []byte, keyLength, ivLength int) ([]byte, []byte) {
	derived := make([]byte, 0, keyLength+ivLength)
	var previous []byte
	for len(derived) < keyLength+ivLength {
		hash := md5.New() // CryptoJS/OpenSSL passphrase compatibility; not used for password storage.
		_, _ = hash.Write(previous)
		_, _ = hash.Write(passphrase)
		_, _ = hash.Write(salt)
		previous = hash.Sum(nil)
		derived = append(derived, previous...)
	}
	return derived[:keyLength], derived[keyLength : keyLength+ivLength]
}

func readMHRSToken(ctx context.Context) (string, error) {
	var cookies []*network.Cookie
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().WithURLs([]string{
			"https://mhrs.gov.tr/vatandas/",
			"https://prd.mhrs.gov.tr/api/",
		}).Do(ctx)
		return err
	})); err != nil {
		return "", err
	}
	if token := tokenFromCookies(cookies); token != "" {
		return token, nil
	}

	var location string
	if err := chromedp.Run(ctx, chromedp.Location(&location)); err != nil {
		return "", err
	}
	parsed, err := url.Parse(location)
	if err != nil || !isMHRSHost(parsed.Hostname()) {
		return "", nil
	}

	var token string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`window.localStorage.getItem("token-v-mhrs") || window.localStorage.getItem("token-v") || ""`, &token)); err != nil {
		return "", err
	}
	return strings.TrimSpace(token), nil
}

func tokenFromCookies(cookies []*network.Cookie) string {
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == "token-v" && isMHRSHost(strings.TrimPrefix(cookie.Domain, ".")) {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
}

func isMHRSHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "mhrs.gov.tr" || strings.HasSuffix(host, ".mhrs.gov.tr")
}

func validateExecutable(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("Chromium yolu mutlak bir dosya yolu olmali")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Chromium bulunamadi: %w", err)
	}
	if info.IsDir() {
		return errors.New("Chromium yolu klasor degil chrome.exe dosyasi olmali")
	}
	return nil
}

func ParseJWT(token string, now time.Time) (JWTInfo, error) {
	var err error
	token, err = NormalizeToken(token)
	if err != nil {
		return JWTInfo{}, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTInfo{}, errors.New("JWT uc bolumden olusmuyor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return JWTInfo{}, errors.New("JWT payload base64url olarak okunamadi")
	}
	var claims struct {
		ExpiresAt json.Number `json:"exp"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil {
		return JWTInfo{}, errors.New("JWT payload JSON olarak okunamadi")
	}
	seconds, err := claims.ExpiresAt.Int64()
	if err != nil || seconds <= 0 {
		return JWTInfo{}, errors.New("JWT son kullanma zamani yok")
	}
	expiresAt := time.Unix(seconds, 0)
	if !expiresAt.After(now.Add(15 * time.Second)) {
		return JWTInfo{}, errors.New("JWT suresi dolmus veya dolmak uzere")
	}
	return JWTInfo{ExpiresAt: expiresAt}, nil
}
