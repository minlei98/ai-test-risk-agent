package jira

import (
	"strings"
	"testing"
)

func TestLoadCredentialsRequiresToken(t *testing.T) {
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_API_TOKEN", "")
	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("expected error when token is missing")
	}
}

func TestCredentialsAuthHeadersAutoPrefersBasicWithUser(t *testing.T) {
	creds := Credentials{Token: "secret", User: "milei@redhat.com", Mode: "auto"}
	headers := creds.authHeaders()
	if len(headers) == 0 || !strings.HasPrefix(headers[0], "Basic ") {
		t.Fatalf("expected basic auth first, got %v", headers)
	}
}

func TestCredentialsAuthHeadersBearerMode(t *testing.T) {
	creds := Credentials{Token: "secret", User: "milei@redhat.com", Mode: "bearer"}
	headers := creds.authHeaders()
	if len(headers) != 1 || headers[0] != "Bearer secret" {
		t.Fatalf("unexpected bearer headers: %v", headers)
	}
}

func TestLoadCredentialsFromEnv(t *testing.T) {
	t.Setenv("JIRA_TOKEN", "abc123")
	t.Setenv("JIRA_USER", "user@example.com")
	t.Setenv("JIRA_AUTH", "basic")
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if creds.Token != "abc123" || creds.User != "user@example.com" {
		t.Fatalf("unexpected creds: %+v", creds)
	}
}
