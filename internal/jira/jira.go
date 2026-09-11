package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Issue is a normalized Jira card used as an input test case.
type Issue struct {
	Key                string   `json:"key"`
	Summary            string   `json:"summary"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptance_criteria,omitempty"`
	IssueType          string   `json:"issue_type,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	Status             string   `json:"status,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	Components         []string `json:"components,omitempty"`
	URL                string   `json:"url,omitempty"`
	ParentKey          string   `json:"parent_key,omitempty"`
	Children           []Issue  `json:"children,omitempty"`
}

// ResolveOptions controls how issues are fetched and expanded.
type ResolveOptions struct {
	IncludeChildren bool
	MaxIssues       int
}

// DefaultResolveOptions enables child expansion with a sensible cap.
func DefaultResolveOptions() ResolveOptions {
	return ResolveOptions{
		IncludeChildren: true,
		MaxIssues:       50,
	}
}

var keyPattern = regexp.MustCompile(`(?i)([A-Z][A-Z0-9]+-\d+)`)

// ParseKey extracts a Jira issue key from a raw key or browse URL.
func ParseKey(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty jira input")
	}
	m := keyPattern.FindStringSubmatch(input)
	if len(m) < 2 {
		return "", fmt.Errorf("could not parse jira key from %q", input)
	}
	return strings.ToUpper(m[1]), nil
}

// LoadIssues reads one or more issues from a JSON file.
// The file may contain a single issue object or {"issues":[...]}.
// Nested "children" arrays are flattened when includeChildren is true.
func LoadIssues(path string, includeChildren bool) ([]Issue, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var single Issue
	if err := json.Unmarshal(b, &single); err == nil && single.Key != "" {
		return flattenIssues([]Issue{normalizeIssue(single)}, includeChildren), nil
	}
	var wrapper struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, fmt.Errorf("parse jira file %s: %w", path, err)
	}
	if len(wrapper.Issues) == 0 {
		return nil, fmt.Errorf("no issues found in %s", path)
	}
	out := make([]Issue, 0, len(wrapper.Issues))
	for _, issue := range wrapper.Issues {
		out = append(out, normalizeIssue(issue))
	}
	return flattenIssues(out, includeChildren), nil
}

func flattenIssues(issues []Issue, includeChildren bool) []Issue {
	if !includeChildren {
		return issues
	}
	var out []Issue
	var walk func(issue Issue, parentKey string)
	walk = func(issue Issue, parentKey string) {
		issue.ParentKey = parentKey
		children := issue.Children
		issue.Children = nil
		out = append(out, issue)
		for _, child := range children {
			walk(normalizeIssue(child), issue.Key)
		}
	}
	for _, issue := range issues {
		walk(issue, "")
	}
	return out
}

// FetchIssue retrieves an issue from the Jira REST API.
func FetchIssue(baseURL, key string, creds Credentials) (Issue, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return Issue{}, fmt.Errorf("jira base_url is required to fetch %s", key)
	}

	fields := "summary,description,issuetype,priority,status,labels,components,subtasks,parent"
	apiURL := fmt.Sprintf("%s%s/issue/%s?fields=%s", baseURL, apiPrefix(baseURL), key, fields)
	status, body, err := creds.get(context.Background(), apiURL)
	if err != nil {
		return Issue{}, fmt.Errorf("jira fetch %s: %w", key, err)
	}
	if status < 200 || status >= 300 {
		return Issue{}, fmt.Errorf("jira fetch %s failed (%d): %s", key, status, strings.TrimSpace(string(body)))
	}

	var raw apiIssue
	if err := json.Unmarshal(body, &raw); err != nil {
		return Issue{}, err
	}
	return normalizeIssue(raw.toIssue(baseURL)), nil
}

// ResolveIssues fetches remote issues and/or loads local fixture files.
// When IncludeChildren is enabled, subtasks and linked child issues are fetched recursively.
func ResolveIssues(opts ResolveOptions, baseURL string, creds Credentials, keys []string, files []string) ([]Issue, error) {
	if opts.MaxIssues <= 0 {
		opts.MaxIssues = 50
	}

	var out []Issue
	seen := map[string]bool{}

	appendIssue := func(issue Issue) {
		if seen[issue.Key] || len(out) >= opts.MaxIssues {
			return
		}
		out = append(out, issue)
		seen[issue.Key] = true
	}

	for _, keyInput := range keys {
		ref, err := ParseIssueRef(keyInput)
		if err != nil {
			return nil, err
		}
		site, err := ResolveBaseURL(ref, baseURL)
		if err != nil {
			return nil, err
		}
		if err := fetchIssueTree(opts, site, creds, ref.Key, "", seen, &out); err != nil {
			return nil, err
		}
	}

	for _, path := range files {
		issues, err := LoadIssues(path, opts.IncludeChildren)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if seen[issue.Key] {
				continue
			}
			if len(out) >= opts.MaxIssues {
				break
			}
			appendIssue(issue)
		}
	}
	return out, nil
}

func fetchIssueTree(opts ResolveOptions, baseURL string, creds Credentials, key, parentKey string, seen map[string]bool, out *[]Issue) error {
	if seen[key] || len(*out) >= opts.MaxIssues {
		return nil
	}

	issue, err := FetchIssue(baseURL, key, creds)
	if err != nil {
		return err
	}
	issue.ParentKey = parentKey
	*out = append(*out, issue)
	seen[key] = true

	if !opts.IncludeChildren || len(*out) >= opts.MaxIssues {
		return nil
	}

	childKeys, err := discoverChildKeys(baseURL, creds, key, issue)
	if err != nil {
		return err
	}
	for _, childKey := range childKeys {
		if err := fetchIssueTree(opts, baseURL, creds, childKey, key, seen, out); err != nil {
			return err
		}
	}
	return nil
}

func discoverChildKeys(baseURL string, creds Credentials, issueKey string, issue Issue) ([]string, error) {
	keys := childKeysFromIssue(issue)
	for _, jql := range childSearchJQL(baseURL, issueKey) {
		found, err := searchIssueKeys(baseURL, creds, jql)
		if err != nil {
			continue
		}
		keys = append(keys, found...)
	}
	return uniqueKeys(keys), nil
}

func childKeysFromIssue(issue Issue) []string {
	var keys []string
	for _, child := range issue.Children {
		if child.Key != "" {
			keys = append(keys, child.Key)
		}
	}
	return keys
}

func searchIssueKeys(baseURL string, creds Credentials, jql string) ([]string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	ctx := context.Background()

	var status int
	var body []byte
	var err error
	if isCloud(baseURL) {
		payload, err := json.Marshal(map[string]interface{}{
			"jql":        jql,
			"maxResults": 50,
			"fields":     []string{"summary"},
		})
		if err != nil {
			return nil, err
		}
		apiURL := baseURL + searchEndpoint(baseURL)
		status, body, err = creds.post(ctx, apiURL, payload)
	} else {
		apiURL := fmt.Sprintf("%s%s?jql=%s&maxResults=50&fields=summary",
			baseURL, searchEndpoint(baseURL), url.QueryEscape(jql))
		status, body, err = creds.get(ctx, apiURL)
	}
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("jira search failed (%d): %s", status, strings.TrimSpace(string(body)))
	}

	var result searchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	var keys []string
	for _, issue := range result.Issues {
		if issue.Key != "" {
			keys = append(keys, issue.Key)
		}
	}
	return keys, nil
}

func uniqueKeys(keys []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, key := range keys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func normalizeIssue(issue Issue) Issue {
	issue.Key = strings.ToUpper(strings.TrimSpace(issue.Key))
	issue.Summary = strings.TrimSpace(issue.Summary)
	issue.Description = strings.TrimSpace(issue.Description)
	if issue.AcceptanceCriteria == "" {
		issue.AcceptanceCriteria = extractAcceptanceCriteria(issue.Description)
	}
	if issue.URL == "" && issue.Key != "" {
		issue.URL = issue.Key
	}
	for i := range issue.Children {
		issue.Children[i] = normalizeIssue(issue.Children[i])
		if issue.Children[i].ParentKey == "" {
			issue.Children[i].ParentKey = issue.Key
		}
	}
	return issue
}

func extractAcceptanceCriteria(description string) string {
	if description == "" {
		return ""
	}
	lines := strings.Split(description, "\n")
	var out []string
	inSection := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "acceptance criteria") || lower == "ac:" || strings.HasPrefix(lower, "acceptance:") {
			inSection = true
			if strings.Contains(lower, ":") {
				after := strings.TrimSpace(strings.SplitN(trim, ":", 2)[1])
				if after != "" {
					out = append(out, after)
				}
			}
			continue
		}
		if inSection {
			if trim == "" && len(out) > 0 {
				break
			}
			if trim != "" {
				out = append(out, trim)
			}
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

type searchResult struct {
	Issues []struct {
		Key string `json:"key"`
	} `json:"issues"`
}

type apiIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		IssueType   struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Labels     []string `json:"labels"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
		Subtasks []struct {
			Key string `json:"key"`
		} `json:"subtasks"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent"`
	} `json:"fields"`
}

func (a apiIssue) toIssue(baseURL string) Issue {
	components := make([]string, 0, len(a.Fields.Components))
	for _, c := range a.Fields.Components {
		if c.Name != "" {
			components = append(components, c.Name)
		}
	}
	children := make([]Issue, 0, len(a.Fields.Subtasks))
	for _, sub := range a.Fields.Subtasks {
		if sub.Key != "" {
			children = append(children, Issue{Key: sub.Key, ParentKey: a.Key})
		}
	}
	parentKey := ""
	if a.Fields.Parent != nil {
		parentKey = a.Fields.Parent.Key
	}
	return Issue{
		Key:         a.Key,
		Summary:     a.Fields.Summary,
		Description: descriptionText(a.Fields.Description),
		IssueType:   a.Fields.IssueType.Name,
		Priority:    a.Fields.Priority.Name,
		Status:      a.Fields.Status.Name,
		Labels:      a.Fields.Labels,
		Components:  components,
		ParentKey:   parentKey,
		Children:    children,
		URL:         fmt.Sprintf("%s/browse/%s", strings.TrimRight(baseURL, "/"), a.Key),
	}
}

// Text returns the combined natural-language content used for analysis.
func (i Issue) Text() string {
	var parts []string
	if i.Summary != "" {
		parts = append(parts, i.Summary)
	}
	if i.Description != "" {
		parts = append(parts, i.Description)
	}
	if i.AcceptanceCriteria != "" && !strings.Contains(i.Description, i.AcceptanceCriteria) {
		parts = append(parts, "Acceptance Criteria:\n"+i.AcceptanceCriteria)
	}
	return strings.Join(parts, "\n\n")
}
