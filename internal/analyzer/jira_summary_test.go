package analyzer

import (
	"testing"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

func TestBuildJiraSummaryGroupsByCategory(t *testing.T) {
	cfg := argyaConfig()
	cases := []InputTestCaseResult{
		{Key: "A-1", Summary: "e2e one", PrimaryTestCategory: "e2e", CoverageStatus: "JIRA_DEFINED", CoverageScore: 70},
		{Key: "A-2", Summary: "e2e two", PrimaryTestCategory: "e2e", CoverageStatus: "JIRA_DEFINED", CoverageScore: 75},
		{Key: "B-1", Summary: "security", PrimaryTestCategory: "security", CoverageStatus: "COVERED", CoverageScore: 90},
	}
	summary := BuildJiraSummary(cases, cfg)
	if summary.TotalCards != 3 {
		t.Fatalf("expected 3 cards, got %d", summary.TotalCards)
	}
	if len(summary.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(summary.Categories))
	}
}

func TestCardNeedsAttentionSkipsCovered(t *testing.T) {
	if CardNeedsAttention(InputTestCaseResult{CoverageStatus: "COVERED", CoverageScore: 90}) {
		t.Fatal("covered cards should not need attention")
	}
	if !CardNeedsAttention(InputTestCaseResult{CoverageStatus: "JIRA_DEFINED", CoverageScore: 70}) {
		t.Fatal("jira-only cards should need attention")
	}
}

func TestBuildJiraSummaryFlagsCount(t *testing.T) {
	cfg := &config.Config{}
	cases := []InputTestCaseResult{
		{Key: "A-1", PrimaryTestCategory: "e2e", CoverageStatus: "COVERED", CoverageScore: 90},
		{Key: "A-2", PrimaryTestCategory: "e2e", CoverageStatus: "JIRA_DEFINED", CoverageScore: 70},
	}
	summary := BuildJiraSummary(cases, cfg)
	if summary.FlaggedCount != 1 {
		t.Fatalf("expected 1 flagged card, got %d", summary.FlaggedCount)
	}
}
