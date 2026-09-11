package report

import (
	"fmt"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

func Markdown(cfg *config.Config, r *analyzer.Result, llmReport string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AI Test Risk & Coverage Report — %s\n\n", cfg.Project.Name)
	fmt.Fprintf(&b, "**Overall risk:** %d/100 (**%s**)\n\n", r.OverallRisk, r.RiskLevel)

	if strings.TrimSpace(llmReport) != "" {
		b.WriteString(llmReport)
		b.WriteString("\n\n---\n\n## Deterministic Analysis\n\n")
	} else {
		b.WriteString("## Executive Summary\n\n")
		b.WriteString("This report is evidence-driven. It identifies implementation signals, detected tests, risk gaps, and concrete recommendations. Absence of a detected test is a gap signal, not proof that the test does not exist elsewhere.\n\n")
	}

	b.WriteString("## Repository Inventory\n\n| Repository | Role | Files | Lines | Test files |\n|---|---|---:|---:|---:|\n")
	for _, x := range r.Repos {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d |\n", x.Name, x.Role, x.Files, x.Lines, x.TestFiles)
	}
	b.WriteString("\n## Test Coverage Signals\n\n| Repository | E2E | Integration | Negative | Security | Resilience | Performance | Upgrade |\n|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, x := range r.Repos {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", x.Name, x.TestTypes["e2e"], x.TestTypes["integration"], x.TestTypes["negative"], x.TestTypes["security"], x.TestTypes["resilience"], x.TestTypes["performance"], x.TestTypes["upgrade"])
	}
	b.WriteString("\n## Risk Areas\n\n")
	for _, x := range r.Repos {
		for _, f := range x.Findings {
			fmt.Fprintf(&b, "### %s — %s (%d/100, %s)\n\n", f.Category, f.Repo, f.Score, f.Severity)
			fmt.Fprintf(&b, "**Evidence:** %s\n\n", f.Evidence)
			fmt.Fprintf(&b, "**Recommendation:** %s\n\n", f.Recommendation)
		}
	}
	b.WriteString("## Cross-Repository Gaps\n\n")
	if len(r.CrossRepoGaps)==0 { b.WriteString("No cross-repository heuristic gaps detected.\n\n") }
	for _, g := range r.CrossRepoGaps { fmt.Fprintf(&b, "- %s\n", g) }

	b.WriteString("\n## Recommendations\n\n")
	for _, x := range r.Recommendations {
		fmt.Fprintf(&b, "- **%s — %s (%d/100):** %s\n", x.Severity, x.Category, x.Score, x.Recommendation)
	}
	b.WriteString("\n## Proposed Test Backlog\n\n")
	for _, x := range r.Recommendations {
		fmt.Fprintf(&b, "### %s — %s\n\n- Priority: **%s**\n- Risk score: **%d/100**\n- Evidence: %s\n- Proposed test: %s\n\n", x.Repo, x.Category, x.Severity, x.Score, x.Evidence, x.Recommendation)
	}
	b.WriteString("## Scoring\n\nRisk is currently evidence-based and intentionally conservative. The MVP emphasizes security, multi-tenancy, GitOps failure handling, cleanup/recovery, and system-level coverage. Extend `rules/risk.yaml` and the analyzer as domain knowledge grows.\n")
	return b.String()
}
