package llm

import (
	"strings"
	"testing"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

func TestCompactEvidenceCapsFindings(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.MaxFindingsPerRepo = 2
	cfg.LLM.MaxEvidenceChars = 0
	cfg.LLM.Mode = "executive"

	result := &analyzer.Result{
		OverallRisk:  55,
		RiskLevel:    "medium",
		AnalysisMode: "repository",
		Repos: []analyzer.RepoResult{{
			Name: "app",
			Role: "application",
			Findings: []analyzer.Finding{
				{Severity: "LOW", Category: "perf", Score: 1, Evidence: "low", Recommendation: "ignore"},
				{Severity: "HIGH", Category: "sec", Score: 90, Evidence: "high", Recommendation: "fix"},
				{Severity: "CRITICAL", Category: "auth", Score: 95, Evidence: "crit", Recommendation: "fix now"},
			},
		}},
	}

	out := compactEvidence(cfg, result)
	if !strings.Contains(out, "[CRITICAL auth 95]") {
		t.Fatalf("expected critical finding in evidence: %s", out)
	}
	if !strings.Contains(out, "[HIGH sec 90]") {
		t.Fatalf("expected high finding in evidence: %s", out)
	}
	if strings.Contains(out, "[LOW perf 1]") {
		t.Fatalf("expected low finding to be omitted: %s", out)
	}
	if !strings.Contains(out, "1 more findings omitted") {
		t.Fatalf("expected omission note: %s", out)
	}
}

func TestCompactEvidenceTruncates(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.MaxFindingsPerRepo = 50
	cfg.LLM.MaxEvidenceChars = 60
	cfg.LLM.Mode = "full"

	result := &analyzer.Result{
		OverallRisk: 10,
		RiskLevel:   "low",
		TopRisks:    []string{"risk one", "risk two", "risk three"},
	}

	out := compactEvidence(cfg, result)
	if !strings.Contains(out, "[evidence truncated for token budget]") {
		t.Fatalf("expected truncation marker: %s", out)
	}
}

func TestLLMEnabledRespectsModeOff(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Enabled = true
	cfg.LLM.Mode = "off"
	if LLMEnabled(cfg) {
		t.Fatal("expected llm mode off to disable synthesis")
	}
}
