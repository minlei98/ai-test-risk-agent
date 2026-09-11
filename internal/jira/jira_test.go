package jira

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	cases := map[string]string{
		"OHSS-12345": "OHSS-12345",
		"https://issues.redhat.com/browse/OHSS-12345": "OHSS-12345",
		"jira://PROJ-9": "PROJ-9",
	}
	for input, want := range cases {
		got, err := ParseKey(input)
		if err != nil {
			t.Fatalf("ParseKey(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLoadIssues(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "jira", "tenant-isolation.json")
	issues, err := LoadIssues(path, true)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Key != "OHSS-12345" {
		t.Fatalf("unexpected key: %s", issues[0].Key)
	}
	if issues[0].AcceptanceCriteria == "" {
		t.Fatal("expected acceptance criteria to be extracted")
	}
}

func TestLoadIssuesExpandsChildren(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "jira", "epic-with-children.json")
	issues, err := LoadIssues(path, true)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	if len(issues) != 4 {
		t.Fatalf("expected 4 flattened issues (epic + 3 children), got %d", len(issues))
	}
	if issues[0].Key != "OHSS-1000" {
		t.Fatalf("expected epic first, got %s", issues[0].Key)
	}
	if issues[1].ParentKey != "OHSS-1000" {
		t.Fatalf("expected story parent OHSS-1000, got %s", issues[1].ParentKey)
	}
	if issues[2].ParentKey != "OHSS-1001" {
		t.Fatalf("expected sub-task parent OHSS-1001, got %s", issues[2].ParentKey)
	}
}

func TestResolveOptionsFlattenMultipleKeys(t *testing.T) {
	opts := ResolveOptions{IncludeChildren: false, MaxIssues: 10}
	issues, err := ResolveIssues(opts, "", Credentials{Token: "test"}, []string{}, []string{
		filepath.Join("..", "..", "testdata", "jira", "tenant-isolation.json"),
		filepath.Join("..", "..", "testdata", "jira", "epic-with-children.json"),
	})
	if err != nil {
		t.Fatalf("ResolveIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 top-level issues with children disabled, got %d", len(issues))
	}
}

func TestIssueTextIncludesAcceptanceCriteria(t *testing.T) {
	issue := Issue{
		Key:                "ABC-1",
		Summary:            "Do the thing",
		Description:        "Details only",
		AcceptanceCriteria: "Must pass",
	}
	text := issue.Text()
	if !strings.Contains(text, "Must pass") {
		t.Fatalf("expected acceptance criteria in text: %q", text)
	}
}
