// Package gotify sends a push notification via a Gotify server's REST API.
package gotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const messagePath = "/message"

// Client sends messages to a Gotify server on behalf of an application
// token.
type Client struct {
	URL        string
	Token      string
	HTTPClient *http.Client
}

// NewClient returns a Client with a sane default HTTP timeout.
func NewClient(url, token string) *Client {
	return &Client{
		URL:        url,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Send posts message to Gotify at the given priority. Markdown rendering is
// requested via the extras field, so report.Build's "**bold**"/"`code`"
// formatting renders as intended.
func (c *Client) Send(ctx context.Context, message string, priority int) error {
	body, err := json.Marshal(map[string]any{
		"message":  message,
		"priority": priority,
		"extras": map[string]any{
			"client::display": map[string]string{"contentType": "text/markdown"},
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.URL, "/")+messagePath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Gotify-Key", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gotify API POST %s: %s: %s", messagePath, resp.Status, strings.TrimSpace(string(respBody)))
	}

	return nil
}
