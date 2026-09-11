package jira

import "testing"

func TestDiscoverChildKeysUsesSubtasksFromIssue(t *testing.T) {
	issue := Issue{
		Key: "SDCICD-1911",
		Children: []Issue{
			{Key: "SDCICD-1912"},
			{Key: "SDCICD-1913"},
		},
	}
	keys, err := discoverChildKeys("https://redhat.atlassian.net", Credentials{Token: "x"}, issue.Key, issue)
	if err != nil {
		t.Fatalf("discoverChildKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 subtask keys, got %v", keys)
	}
}

func TestChildSearchJQLQuotesParentKey(t *testing.T) {
	queries := childSearchJQL("https://redhat.atlassian.net", "SDCICD-1911")
	if len(queries) < 2 {
		t.Fatal("expected cloud child search queries")
	}
	for _, q := range queries {
		if !containsAll(q, "\"SDCICD-1911\"") {
			t.Fatalf("expected quoted key in %q", q)
		}
	}
}

func containsAll(s, part string) bool {
	return len(part) == 0 || (len(s) >= len(part) && indexOfSubstring(s, part) >= 0)
}

func indexOfSubstring(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
