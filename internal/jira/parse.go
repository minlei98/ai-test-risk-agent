package jira

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// IssueRef identifies a Jira issue and optional site override.
type IssueRef struct {
	Key     string
	BaseURL string
}

// ParseIssueRef extracts the issue key and optional site from a key or browse URL.
func ParseIssueRef(input string) (IssueRef, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return IssueRef{}, fmt.Errorf("empty jira input")
	}

	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil {
			return IssueRef{}, fmt.Errorf("parse jira url %q: %w", input, err)
		}
		key, err := ParseKey(u.Path)
		if err != nil {
			return IssueRef{}, err
		}
		base := strings.TrimRight(fmt.Sprintf("%s://%s", u.Scheme, u.Host), "/")
		return IssueRef{Key: key, BaseURL: base}, nil
	}

	key, err := ParseKey(input)
	if err != nil {
		return IssueRef{}, err
	}
	return IssueRef{Key: key}, nil
}

// ResolveBaseURL picks the Jira site for an issue.
func ResolveBaseURL(ref IssueRef, configured string) (string, error) {
	if ref.BaseURL != "" {
		return ref.BaseURL, nil
	}
	if env := strings.TrimSpace(os.Getenv("JIRA_BASE_URL")); env != "" {
		return strings.TrimRight(env, "/"), nil
	}
	configured = strings.TrimRight(strings.TrimSpace(configured), "/")
	if configured == "" {
		return "", fmt.Errorf("jira base_url is required for %s (set jira.base_url, JIRA_BASE_URL, or pass a full browse URL)", ref.Key)
	}
	return configured, nil
}

func isCloud(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "atlassian.net")
}

func apiPrefix(baseURL string) string {
	if isCloud(baseURL) {
		return "/rest/api/3"
	}
	return "/rest/api/2"
}

func searchEndpoint(baseURL string) string {
	if isCloud(baseURL) {
		return "/rest/api/3/search/jql"
	}
	return "/rest/api/2/search"
}

// quoteKey wraps an issue key for safe use in JQL (avoids "KEY-123" being parsed as subtraction).
func quoteKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return key
	}
	if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
		return key
	}
	return fmt.Sprintf("\"%s\"", key)
}

func childSearchJQL(baseURL, parentKey string) []string {
	key := quoteKey(parentKey)
	if isCloud(baseURL) {
		return []string{
			fmt.Sprintf("parent = %s", key),
			fmt.Sprintf("issue in childIssuesOf(%s)", key),
		}
	}
	return []string{
		fmt.Sprintf("parent = %s", key),
		fmt.Sprintf("\"Epic Link\" = %s", key),
	}
}
