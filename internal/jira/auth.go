package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Credentials holds Jira API authentication settings.
type Credentials struct {
	Token string
	User  string
	Mode  string // auto, basic, bearer
}

// LoadCredentials reads Jira auth from the environment.
// For issues.redhat.com, set JIRA_USER (or JIRA_EMAIL) and JIRA_TOKEN.
func LoadCredentials() (Credentials, error) {
	token := strings.TrimSpace(os.Getenv("JIRA_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("JIRA_API_TOKEN"))
	}
	if token == "" {
		return Credentials{}, fmt.Errorf("JIRA_TOKEN or JIRA_API_TOKEN is required to fetch Jira issues")
	}
	user := strings.TrimSpace(os.Getenv("JIRA_USER"))
	if user == "" {
		user = strings.TrimSpace(os.Getenv("JIRA_EMAIL"))
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("JIRA_AUTH")))
	if mode == "" {
		mode = "auto"
	}
	return Credentials{Token: token, User: user, Mode: mode}, nil
}

func (c Credentials) authHeaders() []string {
	switch c.Mode {
	case "basic":
		return []string{c.basicHeader()}
	case "bearer":
		return []string{c.bearerHeader()}
	default:
		if c.User != "" {
			return []string{c.basicHeader(), c.bearerHeader()}
		}
		return []string{c.bearerHeader(), c.basicHeader()}
	}
}

func (c Credentials) basicHeader() string {
	user := c.User
	if user == "" {
		user = c.Token
	}
	raw := user + ":" + c.Token
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

func (c Credentials) bearerHeader() string {
	return "Bearer " + c.Token
}

func (c Credentials) get(ctx context.Context, url string) (int, []byte, error) {
	return c.do(ctx, http.MethodGet, url, nil)
}

func (c Credentials) post(ctx context.Context, url string, body []byte) (int, []byte, error) {
	return c.do(ctx, http.MethodPost, url, body)
}

func (c Credentials) do(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var lastStatus int
	var lastBody []byte

	for _, header := range c.authHeaders() {
		cloned := req.Clone(ctx)
		if len(body) > 0 {
			cloned.Body = io.NopCloser(bytes.NewReader(body))
			cloned.ContentLength = int64(len(body))
		}
		cloned.Header.Set("Authorization", header)
		cloned.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			cloned.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(cloned)
		if err != nil {
			return 0, nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.StatusCode, body, nil
		}
		lastStatus = resp.StatusCode
		lastBody = body
		if resp.StatusCode != http.StatusUnauthorized &&
			resp.StatusCode != http.StatusForbidden &&
			resp.StatusCode != http.StatusNotFound {
			return resp.StatusCode, body, nil
		}
	}
	return lastStatus, lastBody, fmt.Errorf("%s", formatJiraHTTPError(lastStatus, lastBody))
}

func formatJiraHTTPError(status int, body []byte) string {
	msg := strings.TrimSpace(string(body))
	if status == http.StatusNotFound {
		return fmt.Sprintf("jira request failed (%d): %s (check issue key, JIRA_USER, and token permissions)", status, msg)
	}
	return fmt.Sprintf("jira request failed (%d): %s", status, msg)
}
