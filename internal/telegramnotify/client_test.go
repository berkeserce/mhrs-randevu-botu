package telegramnotify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewFromEnvironmentDisabledWhenEmpty(t *testing.T) {
	t.Setenv(botTokenEnvironment, "")
	t.Setenv(chatIDEnvironment, "")
	client, err := NewFromEnvironment()
	if err != nil || client != nil {
		t.Fatalf("client = %#v, err = %v", client, err)
	}
}

func TestNewFromEnvironmentRequiresBothValues(t *testing.T) {
	t.Setenv(botTokenEnvironment, "secret")
	t.Setenv(chatIDEnvironment, "")
	if _, err := NewFromEnvironment(); err == nil {
		t.Fatal("missing chat ID should be rejected")
	}
}

func TestSendPostsMessage(t *testing.T) {
	var method, path, chatID, text string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method, path = request.Method, request.URL.Path
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		chatID, text = request.Form.Get("chat_id"), request.Form.Get("text")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer server.Close()

	client := newClient(server.URL, "123:secret", "456", server.Client())
	if err := client.Send(context.Background(), "Randevu bulundu"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || !strings.HasSuffix(path, "/sendMessage") {
		t.Fatalf("method = %q, path = %q", method, path)
	}
	if chatID != "456" || text != "Randevu bulundu" {
		t.Fatalf("chat_id = %q, text = %q", chatID, text)
	}
}

func TestSendDoesNotLeakTokenOnConnectionError(t *testing.T) {
	client := newClient("http://127.0.0.1:1", "123:very-secret-token", "456", http.DefaultClient)
	err := client.Send(context.Background(), "test")
	if err == nil {
		t.Fatal("connection error expected")
	}
	if strings.Contains(err.Error(), "very-secret-token") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func TestSendDoesNotLeakTokenFromAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(writer, `{"ok":false,"description":"bad 123:very-secret-token"}`)
	}))
	defer server.Close()

	client := newClient(server.URL, "123:very-secret-token", "456", server.Client())
	err := client.Send(context.Background(), "test")
	if err == nil {
		t.Fatal("API error expected")
	}
	if strings.Contains(err.Error(), "very-secret-token") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func TestDiscoverChatIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/getUpdates") {
			t.Errorf("path = %q", request.URL.Path)
		}
		_, _ = io.WriteString(writer, `{"ok":true,"result":[{"message":{"chat":{"id":123}}},{"message":{"chat":{"id":123}}},{"message":{"chat":{"id":456}}}]}`)
	}))
	defer server.Close()

	chatIDs, err := discoverChatIDs(context.Background(), server.URL, "123:secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(chatIDs) != 2 || chatIDs[0] != "123" || chatIDs[1] != "456" {
		t.Fatalf("chat IDs = %#v", chatIDs)
	}
}
