package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

func Markdown(cfg *config.Config, r *analyzer.Result, llmReport string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Test Quality Analysis Report — %s\n\n", cfg.Project.Name)
	if cfg.Project.Type != "" {
		fmt.Fprintf(&b, "**Project type:** %s\n\n", cfg.Project.Type)
	}
	if cfg.Project.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", cfg.Project.Description)
	}
	fmt.Fprintf(&b, "**Date:** %s\n\n", time.Now().Format("January 2, 2006"))

	if strings.TrimSpace(llmReport) != "" {
		b.WriteString(llmReport)
		b.WriteString("\n\n---\n\n## Deterministic Analysis\n\n")
	}

	writeExecutiveSummary(&b, cfg, r)
	writeCIContext(&b, r)
	writeTestEffectiveness(&b, cfg, r)
	writeRiskCoverage(&b, r)
	writeCustomerWorkflows(&b, r)
	writeSecurityAnalysis(&b, r)
	writeResilienceAnalysis(&b, r)
	writeUncoveredRisks(&b, r)
	writeRiskAreas(&b, r)
	writeCrossRepoGaps(&b, r)
	writeRecommendations(&b, r)
	writeProposedTests(&b, r)
	writeScorecard(&b, cfg, r)

	return b.String()
}

func writeExecutiveSummary(b *strings.Builder, cfg *config.Config, r *analyzer.Result) {
	b.WriteString("## 1. Executive Summary\n\n")
	b.WriteString("| Metric | Assessment |\n|---|---|\n")
	fmt.Fprintf(b, "| Overall quality score | %.1f / 10 |\n", r.Scorecard.OverallQuality)
	fmt.Fprintf(b, "| Overall risk level | %s |\n", r.RiskLevel)
	fmt.Fprintf(b, "| Release readiness | %s |\n", r.Scorecard.ReleaseReadiness)
	fmt.Fprintf(b, "| Overall risk score | %d / 100 |\n", r.OverallRisk)

	totalTests := 0
	totalSpecs := 0
	for _, repo := range r.Repos {
		totalTests += repo.TestFiles
		totalSpecs += repo.SpecFiles
	}
	fmt.Fprintf(b, "| Executable test/spec files | %d test files + %d Ginkgo specs |\n", totalTests, totalSpecs)
	b.WriteString("\n")

	if len(r.TopRisks) > 0 {
		b.WriteString("### Top Uncovered Risks\n\n")
		for i, risk := range r.TopRisks {
			fmt.Fprintf(b, "%d. %s\n", i+1, risk)
		}
		b.WriteString("\n")
	}

	b.WriteString("This report is evidence-driven. Absence of a detected test is a gap signal, not proof that coverage does not exist elsewhere.\n\n")
	if len(r.CoverageNotes) > 0 {
		for _, note := range r.CoverageNotes {
			fmt.Fprintf(b, "- %s\n", note)
		}
		b.WriteString("\n")
	}
}

func writeCIContext(b *strings.Builder, r *analyzer.Result) {
	if len(r.CILayers) == 0 {
		return
	}
	b.WriteString("## 2. CI Framework Context\n\n")
	b.WriteString("| Layer | Purpose | Examples |\n|---|---|---|\n")
	for _, layer := range r.CILayers {
		examples := layer.Examples
		if examples == "" {
			examples = "—"
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", layer.Layer, layer.Purpose, examples)
	}
	b.WriteString("\n**CI evidence:**\n")
	for _, repo := range r.Repos {
		if len(repo.CIEvidence) == 0 {
			continue
		}
		fmt.Fprintf(b, "- **%s:** %s\n", repo.Name, strings.Join(repo.CIEvidence, ", "))
	}
	b.WriteString("\n")
}

func writeTestEffectiveness(b *strings.Builder, cfg *config.Config, r *analyzer.Result) {
	b.WriteString("## 3. Test Effectiveness Analysis\n\n")
	b.WriteString("### Category Scores\n\n")
	b.WriteString("| Category | Score | Rationale |\n|---|---:|---|\n")
	writeCategoryScore(b, "E2E Tests", r.Scorecard.E2EEffectiveness, e2eRationale(r))
	writeCategoryScore(b, "E2E Automation Framework", r.Scorecard.FrameworkQuality, frameworkRationale(r))
	writeCategoryScore(b, "CI Pipeline", r.Scorecard.CIIntegration, ciRationale(r))
	b.WriteString("\n")

	writeTestCoverageTables(b, cfg, r)

	if len(r.KeyIssues) > 0 {
		b.WriteString("### Key Issues\n\n")
		b.WriteString("| Issue | Impact | Recommendation |\n|---|---|---|\n")
		for _, issue := range r.KeyIssues {
			fmt.Fprintf(b, "| %s | %s | %s |\n", issue.Issue, issue.Impact, issue.Recommendation)
		}
		b.WriteString("\n")
	}
}

func writeCategoryScore(b *strings.Builder, name string, score float64, rationale string) {
	fmt.Fprintf(b, "| %s | %.1f / 10 | %s |\n", name, score, rationale)
}

func e2eRationale(r *analyzer.Result) string {
	primary := primaryRepo(r)
	return fmt.Sprintf("E2E/functional signals=%d; skips=%d; shallow assertions=%d",
		primary.TestLevels["e2e"]+primary.TestLevels["functional"], totalSkips(r), primary.ShallowAssertCount)
}

func frameworkRationale(r *analyzer.Result) string {
	primary := primaryRepo(r)
	parts := []string{}
	if primary.HasGinkgo {
		parts = append(parts, "Ginkgo detected")
	}
	if primary.HasJUnitOutput {
		parts = append(parts, "JUnit output")
	}
	if primary.SpecFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d in-repo specs", primary.SpecFiles))
	}
	if len(parts) == 0 {
		return "Limited framework signals detected"
	}
	return strings.Join(parts, "; ")
}

func ciRationale(r *analyzer.Result) string {
	parts := []string{}
	for _, repo := range r.Repos {
		if repo.HasProw {
			parts = append(parts, "Prow/ci-operator references")
		}
		if repo.HasTekton {
			parts = append(parts, "Tekton pipelines")
		}
		if repo.HasGitHubActions {
			parts = append(parts, "GitHub Actions")
		}
	}
	if len(parts) == 0 {
		return "Limited CI integration evidence"
	}
	return strings.Join(parts, "; ")
}

func writeTestCoverageTables(b *strings.Builder, cfg *config.Config, r *analyzer.Result) {
	b.WriteString("### Repository Inventory\n\n")
	b.WriteString("| Repository | Role | Files | Lines | Test files | Spec files | Key focus areas |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---|\n")
	for _, x := range r.Repos {
		areas := strings.Join(x.KeyAreas, ", ")
		if areas == "" {
			areas = "—"
		}
		fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d | %s |\n",
			x.Name, x.Role, x.Files, x.Lines, x.TestFiles, x.SpecFiles, areas)
	}
	b.WriteString("\n")

	levels := cfg.Analysis.TestLevels
	if len(levels) == 0 {
		levels = []string{"unit", "component", "integration", "api", "functional", "e2e", "system"}
	}
	b.WriteString("### Test levels\n\n| Repository |")
	for _, level := range levels {
		fmt.Fprintf(b, " %s |", level)
	}
	b.WriteString("\n|---|")
	for range levels {
		b.WriteString("---:|")
	}
	b.WriteString("\n")
	for _, x := range r.Repos {
		fmt.Fprintf(b, "| %s |", x.Name)
		for _, level := range levels {
			fmt.Fprintf(b, " %d |", x.TestLevels[level])
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	cross := cfg.Analysis.CrossCutting
	if len(cross) == 0 {
		cross = []string{"negative", "security", "resilience", "performance", "upgrade", "regression"}
	}
	b.WriteString("### Cross-cutting concerns\n\n| Repository |")
	for _, c := range cross {
		fmt.Fprintf(b, " %s |", c)
	}
	b.WriteString("\n|---|")
	for range cross {
		b.WriteString("---:|")
	}
	b.WriteString("\n")
	for _, x := range r.Repos {
		fmt.Fprintf(b, "| %s |", x.Name)
		for _, c := range cross {
			fmt.Fprintf(b, " %d |", x.CrossCutting[c])
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeRiskCoverage(b *strings.Builder, r *analyzer.Result) {
	b.WriteString("## 4. Risk Coverage Analysis\n\n")

	writeCoverageGroup(b, "Functional Risk", r.RiskCoverage.Functional, r.RiskCoverage.OverallFunc)
	writeCoverageGroup(b, "Operational Risk", r.RiskCoverage.Operational, r.RiskCoverage.OverallOps)
	writeCoverageGroup(b, "Reliability Risk", r.RiskCoverage.Reliability, r.RiskCoverage.OverallRel)
	writeCoverageGroup(b, "Integration Risk", r.RiskCoverage.Integration, r.RiskCoverage.OverallInt)
}

func writeCoverageGroup(b *strings.Builder, title string, items []analyzer.CoverageRating, overall string) {
	b.WriteString("### " + title + "\n\n")
	b.WriteString("| Sub-area | Rating | Evidence |\n|---|---|---|\n")
	for _, item := range items {
		fmt.Fprintf(b, "| %s | %s | %s |\n", item.SubArea, item.Rating, item.Evidence)
	}
	fmt.Fprintf(b, "\n**Overall %s Rating:** %s\n\n", title, overall)
}

func writeCustomerWorkflows(b *strings.Builder, r *analyzer.Result) {
	b.WriteString("## 5. Customer Workflow Validation\n\n")
	b.WriteString("| Workflow | Status | Notes |\n|---|---|---|\n")
	for _, wf := range r.Workflows {
		fmt.Fprintf(b, "| %s | %s | %s |\n", wf.Workflow, wf.Status, wf.Notes)
	}
	b.WriteString("\n")
}

func writeSecurityAnalysis(b *strings.Builder, r *analyzer.Result) {
	b.WriteString("## 6. Security Validation Analysis\n\n")
	b.WriteString("| Area | Covered? | Detail |\n|---|---|---|\n")
	for _, check := range r.SecurityChecks {
		fmt.Fprintf(b, "| %s | %s | %s |\n", check.Area, check.Covered, check.Detail)
	}
	fmt.Fprintf(b, "\n**Security Coverage Score:** %.1f / 10\n\n", r.Scorecard.SecurityValidation)
}

func writeResilienceAnalysis(b *strings.Builder, r *analyzer.Result) {
	b.WriteString("## 7. Resilience and Chaos Validation\n\n")
	b.WriteString("| Scenario | Validated? |\n|---|---|\n")
	for _, check := range r.ResilienceChecks {
		fmt.Fprintf(b, "| %s | %s |\n", check.Scenario, check.Validated)
	}
	fmt.Fprintf(b, "\n**Resilience Score:** %.1f / 10\n\n", r.Scorecard.ResilienceValidation)
}

func writeUncoveredRisks(b *strings.Builder, r *analyzer.Result) {
	if len(r.UncoveredRisks) == 0 {
		return
	}
	b.WriteString("### Uncovered Risks\n\n")
	for _, risk := range r.UncoveredRisks {
		fmt.Fprintf(b, "- %s\n", risk)
	}
	b.WriteString("\n")
}

func writeRiskAreas(b *strings.Builder, r *analyzer.Result) {
	if len(r.Recommendations) == 0 {
		return
	}
	b.WriteString("## Risk Areas\n\n")
	for i, f := range r.Recommendations {
		fmt.Fprintf(b, "%d. **%s — %s (%d/100, %s):** %s\n", i+1, f.Category, f.Repo, f.Score, f.Severity, f.Evidence)
	}
	b.WriteString("\n")
}

func writeCrossRepoGaps(b *strings.Builder, r *analyzer.Result) {
	b.WriteString("## Cross-Repository Gaps\n\n")
	if len(r.CrossRepoGaps) == 0 {
		b.WriteString("No cross-repository heuristic gaps detected.\n\n")
		return
	}
	for _, g := range r.CrossRepoGaps {
		fmt.Fprintf(b, "- %s\n", g)
	}
	b.WriteString("\n")
}

func writeRecommendations(b *strings.Builder, r *analyzer.Result) {
	if len(r.Recommendations) == 0 {
		return
	}
	b.WriteString("## 9. Recommended Additional Tests\n\n")
	priorityGroups := map[string][]analyzer.Finding{"P0": {}, "P1": {}, "P2": {}, "P3": {}}
	for _, f := range r.Recommendations {
		priorityGroups[f.Severity] = append(priorityGroups[f.Severity], f)
	}
	for _, sev := range []string{"P0", "P1", "P2", "P3"} {
		items := priorityGroups[sev]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(b, "### Priority %s\n\n", priorityLabel(sev))
		b.WriteString("| Test | Risk Addressed | Implementation |\n|---|---|---|\n")
		for _, f := range items {
			risk := f.MissingScenario
			if risk == "" {
				risk = f.Evidence
			}
			fmt.Fprintf(b, "| %s | %s | %s |\n", f.Category, risk, f.Recommendation)
		}
		b.WriteString("\n")
	}
}

func priorityLabel(sev string) string {
	switch sev {
	case "P0":
		return "1 (Critical)"
	case "P1":
		return "2 (High)"
	case "P2":
		return "3 (Medium)"
	default:
		return "4 (Low)"
	}
}

func writeProposedTests(b *strings.Builder, r *analyzer.Result) {
	if len(r.Recommendations) == 0 {
		return
	}
	b.WriteString("## Proposed Tests\n\n")
	for i, f := range r.Recommendations {
		fmt.Fprintf(b, "### Test %d: %s — %s\n\n", i+1, f.Category, f.Repo)
		fmt.Fprintf(b, "- **Priority:** %s\n", f.Severity)
		fmt.Fprintf(b, "- **Risk score:** %d/100\n", f.Score)
		fmt.Fprintf(b, "- **Evidence:** %s\n", f.Evidence)
		if f.MissingScenario != "" {
			fmt.Fprintf(b, "- **Missing scenario:** %s\n", f.MissingScenario)
		}
		if f.TestLevel != "" {
			fmt.Fprintf(b, "- **Proposed test level:** %s\n", f.TestLevel)
		}
		if f.TestSteps != "" {
			fmt.Fprintf(b, "- **Concrete test steps:**\n")
			for _, step := range strings.Split(f.TestSteps, "\n") {
				step = strings.TrimSpace(step)
				if step != "" {
					fmt.Fprintf(b, "  %s\n", step)
				}
			}
		}
		fmt.Fprintf(b, "- **Proposed test:** %s\n\n", f.Recommendation)
	}
}

func writeScorecard(b *strings.Builder, cfg *config.Config, r *analyzer.Result) {
	b.WriteString("## 10. Final Quality Scorecard\n\n")
	b.WriteString("| Category | Score |\n|---|---:|\n")
	for name, score := range r.Scorecard.CategoryScores {
		fmt.Fprintf(b, "| %s | %.1f / 10 |\n", name, score)
	}
	fmt.Fprintf(b, "\n**Final Recommendation:** %s\n\n", r.Scorecard.ReleaseReadiness)
	fmt.Fprintf(b, "**Rationale:** Overall quality %.1f/10 with risk %d/100 (%s). ", r.Scorecard.OverallQuality, r.OverallRisk, r.RiskLevel)
	if len(cfg.Risk.Prioritize) > 0 {
		fmt.Fprintf(b, "Prioritization dimensions: %s. ", strings.Join(cfg.Risk.Prioritize, ", "))
	}
	b.WriteString("Scores are evidence-based from repository scanning and intentionally conservative.\n")
}

func primaryRepo(r *analyzer.Result) analyzer.RepoResult {
	for _, repo := range r.Repos {
		if repo.Role == "application" || repo.Name == "osde2e" {
			return repo
		}
	}
	if len(r.Repos) > 0 {
		return r.Repos[0]
	}
	return analyzer.RepoResult{}
}

func totalSkips(r *analyzer.Result) int {
	n := 0
	for _, repo := range r.Repos {
		n += repo.SkipCount
	}
	return n
}
