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
	"strconv"
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

func DiscoverChatIDsFromEnvironment(ctx context.Context) ([]string, error) {
	token := strings.TrimSpace(os.Getenv(botTokenEnvironment))
	if token == "" {
		return nil, fmt.Errorf("%s ayarlanmamis", botTokenEnvironment)
	}
	return discoverChatIDs(ctx, productionBaseURL, token, &http.Client{Timeout: 15 * time.Second})
}

func discoverChatIDs(ctx context.Context, baseURL, token string, httpClient *http.Client) ([]string, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/bot" + url.PathEscape(token) + "/getUpdates"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("Telegram istegi olusturulamadi")
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, errors.New("Telegram servisine baglanilamadi")
	}
	defer response.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      []struct {
			Message *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("Telegram gecersiz yanit verdi (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !result.OK {
		return nil, telegramAPIError(response.StatusCode, result.Description, token)
	}

	seen := make(map[int64]bool)
	chatIDs := make([]string, 0)
	for _, update := range result.Result {
		if update.Message == nil || seen[update.Message.Chat.ID] {
			continue
		}
		seen[update.Message.Chat.ID] = true
		chatIDs = append(chatIDs, strconv.FormatInt(update.Message.Chat.ID, 10))
	}
	return chatIDs, nil
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
		return telegramAPIError(response.StatusCode, result.Description, c.botToken)
	}
	return nil
}

func telegramAPIError(status int, description, token string) error {
	description = strings.TrimSpace(description)
	if description == "" {
		description = "istek reddedildi"
	}
	description = strings.ReplaceAll(description, token, "[gizlendi]")
	return fmt.Errorf("Telegram istegi reddedildi (HTTP %d): %s", status, description)
}
