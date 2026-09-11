package jira

import (
	"strings"
	"testing"
)

func TestParseIssueRefFromBrowseURL(t *testing.T) {
	ref, err := ParseIssueRef("https://redhat.atlassian.net/browse/SDCICD-1915")
	if err != nil {
		t.Fatalf("ParseIssueRef: %v", err)
	}
	if ref.Key != "SDCICD-1915" {
		t.Fatalf("unexpected key: %s", ref.Key)
	}
	if ref.BaseURL != "https://redhat.atlassian.net" {
		t.Fatalf("unexpected base url: %s", ref.BaseURL)
	}
}

func TestAPIPrefixForAtlassianCloud(t *testing.T) {
	if apiPrefix("https://redhat.atlassian.net") != "/rest/api/3" {
		t.Fatal("expected api v3 for atlassian cloud")
	}
	if apiPrefix("https://issues.redhat.com") != "/rest/api/2" {
		t.Fatal("expected api v2 for issues.redhat.com")
	}
}

func TestChildSearchJQLEscapesIssueKey(t *testing.T) {
	queries := childSearchJQL("https://redhat.atlassian.net", "SDCICD-1911")
	if len(queries) == 0 {
		t.Fatal("expected child search queries")
	}
	for _, q := range queries {
		if !strings.Contains(q, "\"SDCICD-1911\"") {
			t.Fatalf("expected quoted issue key in %q", q)
		}
	}
}

func TestSearchEndpointForAtlassianCloud(t *testing.T) {
	if searchEndpoint("https://redhat.atlassian.net") != "/rest/api/3/search/jql" {
		t.Fatal("expected new jql search endpoint for atlassian cloud")
	}
	if searchEndpoint("https://issues.redhat.com") != "/rest/api/2/search" {
		t.Fatal("expected legacy search endpoint for issues.redhat.com")
	}
}
