package gotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSend_PostsMessageWithAuthHeaderAndPriority(t *testing.T) {
	var gotKey string
	var gotPath string
	var body struct {
		Message  string `json:"message"`
		Priority int    `json:"priority"`
		Extras   struct {
			ClientDisplay struct {
				ContentType string `json:"contentType"`
			} `json:"client::display"`
		} `json:"extras"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Gotify-Key")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")

	if err := client.Send(context.Background(), "hello world", 5); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotKey != "test-token" {
		t.Errorf("X-Gotify-Key header = %q, want %q", gotKey, "test-token")
	}
	if gotPath != "/message" {
		t.Errorf("path = %q, want /message", gotPath)
	}
	if body.Message != "hello world" {
		t.Errorf("message = %q, want %q", body.Message, "hello world")
	}
	if body.Priority != 5 {
		t.Errorf("priority = %d, want 5", body.Priority)
	}
	if body.Extras.ClientDisplay.ContentType != "text/markdown" {
		t.Errorf("extras contentType = %q, want text/markdown", body.Extras.ClientDisplay.ContentType)
	}
}

func TestSend_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token")

	if err := client.Send(context.Background(), "hello", 0); err == nil {
		t.Fatal("expected an error for a 401 response")
	}
}
