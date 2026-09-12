package analyzer

import (
	"testing"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
	"github.com/minlei98/ai-test-risk-agent/internal/jira"
)

func argyaConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Risk.Prioritize = []string{"security", "gitops", "multi_tenancy", "customer_impact", "resilience"}
	cfg.Risk.Deprioritize = []string{"performance"}
	return cfg
}

func TestAnalyzeInputCasesWithoutRepoTestsUsesJiraEvidence(t *testing.T) {
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
		Key:                "SDCICD-1911",
		Summary:            "Argo Progressive delivery pipeline feature testing",
		Description:        "Validate progressive delivery on live ArgoCD hub",
		AcceptanceCriteria: "Rollout pauses on failure\nRollback restores previous version",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	tc := out[0]
	if res.AnalysisMode != AnalysisModeHybrid {
		t.Fatalf("expected hybrid mode, got %s", res.AnalysisMode)
	}
	if tc.PrimaryTestCategory == "performance" {
		t.Fatal("performance should not be primary for progressive delivery card")
	}
	if tc.CoverageStatus == "NOT COVERED" {
		t.Fatalf("Jira-only evidence should not be NOT COVERED, got %s", tc.CoverageStatus)
	}
}

func TestAnalyzeInputCasesUsesRepoEvidenceWhenPresent(t *testing.T) {
	res := &Result{
		Repos: []RepoResult{
			{
				Name:         "argya",
				Role:         "application",
				Files:        100,
				TestFiles:    5,
				CrossCutting: map[string]int{"security": 3},
				TestLevels:   map[string]int{"e2e": 4, "integration": 2},
			},
		},
	}
	issue := jira.Issue{
		Key:                "SDCICD-2000",
		Summary:            "Security E2E validation for tenant isolation",
		AcceptanceCriteria: "Cross-tenant access must be denied",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	tc := out[0]
	if len(tc.MatchedSignals) == 0 {
		t.Fatal("expected matched repository signals")
	}
	if tc.CoverageStatus != "COVERED" && tc.CoverageStatus != "PARTIAL" {
		t.Fatalf("expected covered/partial when repo tests exist, got %s", tc.CoverageStatus)
	}
}

func TestPerformanceCardIsDeprioritizedForArgya(t *testing.T) {
	res := &Result{Repos: []RepoResult{{Name: "argya", Files: 10}}}
	issue := jira.Issue{
		Key:                "SDCICD-3000",
		Summary:            "Load test progressive delivery throughput",
		Description:        "Measure performance and latency during rollout",
		AcceptanceCriteria: "Latency remains under threshold during load test",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	tc := out[0]
	if tc.PriorityNote == "" {
		t.Fatal("expected priority note for deprioritized performance card")
	}
}

func TestGitOpsCardClassifiedAsGitOpsNotIntegration(t *testing.T) {
	res := &Result{Repos: []RepoResult{{Name: "argya", Files: 10}}}
	issue := jira.Issue{
		Key:                "SDCICD-1911",
		Summary:            "Argo Progressive delivery pipeline feature testing",
		Description:        "Validate progressive delivery on live ArgoCD hub",
		AcceptanceCriteria: "Rollout pauses on failure\nRollback restores previous version",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	tc := out[0]
	if tc.PrimaryTestCategory != "gitops" {
		t.Fatalf("expected gitops primary category, got %s (categories=%v)", tc.PrimaryTestCategory, tc.TestCategories)
	}
}

func TestTenantIsolationClassifiedAsMultiTenancy(t *testing.T) {
	res := &Result{Repos: []RepoResult{{Name: "argya", Files: 10}}}
	issue := jira.Issue{
		Key:                "SDCICD-2001",
		Summary:            "Tenant namespace isolation validation",
		AcceptanceCriteria: "Each tenant project receives an isolated namespace",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	tc := out[0]
	if tc.PrimaryTestCategory != "multi_tenancy" {
		t.Fatalf("expected multi_tenancy primary category, got %s (categories=%v)", tc.PrimaryTestCategory, tc.TestCategories)
	}
}

func TestFeatureCardCanClassifyAsFunctionality(t *testing.T) {
	res := &Result{Repos: []RepoResult{{Name: "argya", Files: 10}}}
	issue := jira.Issue{
		Key:         "SDCICD-2100",
		Summary:     "Functional regression for addon behavior",
		Description: "Verify feature behavior matches expected capability",
	}
	out := AnalyzeInputCases([]jira.Issue{issue}, res, argyaConfig())
	tc := out[0]
	if tc.PrimaryTestCategory != "functionality" {
		t.Fatalf("expected functionality primary category, got %s (categories=%v)", tc.PrimaryTestCategory, tc.TestCategories)
	}
}

func TestRepositoryModeWhenNoJira(t *testing.T) {
	res := &Result{Repos: []RepoResult{{Name: "argya", Files: 10}}}
	out := AnalyzeInputCases(nil, res, argyaConfig())
	if out != nil {
		t.Fatal("expected nil input cases without jira")
	}
}
