package telegramnotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	botTokenEnvironment = "TELEGRAM_BOT_TOKEN"
	chatIDEnvironment   = "TELEGRAM_CHAT_ID"
	productionBaseURL   = "https://api.telegram.org"
	maxMessageRunes     = 4096
)

type Client struct {
	baseURL    string
	botToken   string
	chatID     string
	httpClient *http.Client
}

func NewFromEnvironment() (*Client, error) {
	token := strings.TrimSpace(os.Getenv(botTokenEnvironment))
	chatID := strings.TrimSpace(os.Getenv(chatIDEnvironment))
	if token == "" && chatID == "" {
		return nil, nil
	}
	if token == "" {
		return nil, fmt.Errorf("%s ayarlanmamis", botTokenEnvironment)
	}
	if chatID == "" {
		return nil, fmt.Errorf("%s ayarlanmamis", chatIDEnvironment)
	}
	return newClient(productionBaseURL, token, chatID, &http.Client{Timeout: 15 * time.Second}), nil
}

func newClient(baseURL, token, chatID string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		botToken:   token,
		chatID:     chatID,
		httpClient: httpClient,
	}
}

func (c *Client) Send(ctx context.Context, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Telegram mesaji bos olamaz")
	}
	if len([]rune(message)) > maxMessageRunes {
		return fmt.Errorf("Telegram mesaji %d karakter sinirini asiyor", maxMessageRunes)
	}

	form := url.Values{"chat_id": {c.chatID}, "text": {message}}
	endpoint := c.baseURL + "/bot" + url.PathEscape(c.botToken) + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("Telegram istegi olusturulamadi")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		// HTTP istemcisi hatalari istek URL'sini ve dolayisiyla bot tokenini
		// icerebilir. Hassas bilgiyi ust katmana tasimiyoruz.
		return errors.New("Telegram servisine baglanilamadi")
	}
	defer response.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return fmt.Errorf("Telegram gecersiz yanit verdi (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !result.OK {
		description := strings.TrimSpace(result.Description)
		if description == "" {
			description = "istek reddedildi"
		}
		description = strings.ReplaceAll(description, c.botToken, "[gizlendi]")
		return fmt.Errorf("Telegram bildirimi gonderilemedi (HTTP %d): %s", response.StatusCode, description)
	}
	return nil
}
