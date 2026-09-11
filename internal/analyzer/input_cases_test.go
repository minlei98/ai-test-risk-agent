package analyzer

import (
	"testing"

	"github.com/minlei98/ai-test-risk-agent/internal/jira"
)

func TestAnalyzeInputCasesDetectsGaps(t *testing.T) {
	res := &Result{
		Repos: []RepoResult{
			{
				Name:         "argya",
				Role:         "application",
				Files:        100,
				TestFiles:    0,
				CrossCutting: map[string]int{},
				TestLevels:   map[string]int{},
			},
		},
	}
	issue := jira.Issue{
		Key:     "OHSS-12345",
		Summary: "Validate tenant RBAC isolation and negative security tests",
		Description: "Acceptance Criteria:\n* tenant-a cannot access tenant-b secrets\n* denied responses are explicit",
		AcceptanceCriteria: "tenant-a cannot access tenant-b secrets\n* denied responses are explicit",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].CoverageStatus == "COVERED" {
		t.Fatalf("expected gap coverage, got %s", out[0].CoverageStatus)
	}
	if out[0].RequirementAnalysis == "" {
		t.Fatal("expected requirement analysis")
	}
	if out[0].RepoTraceability == "" {
		t.Fatal("expected repository traceability")
	}
	if len(out[0].RequirementItems) == 0 {
		t.Fatal("expected requirement items from acceptance criteria")
	}
	if len(out[0].MissingScenarios) == 0 {
		t.Fatal("expected missing scenarios")
	}
}
