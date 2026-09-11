package analyzer

import (
	"fmt"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/jira"
)

// RequirementItem is one testable statement from a Jira card.
type RequirementItem struct {
	Text           string `json:"text"`
	CoverageStatus string `json:"coverage_status"`
	RepoEvidence   string `json:"repo_evidence,omitempty"`
}

// InputTestCaseResult captures requirement analysis and repository traceability for a Jira card.
type InputTestCaseResult struct {
	Key                  string            `json:"key"`
	Summary              string            `json:"summary"`
	URL                  string            `json:"url,omitempty"`
	ParentKey            string            `json:"parent_key,omitempty"`
	IssueType            string            `json:"issue_type,omitempty"`
	Status               string            `json:"status,omitempty"`
	Priority             string            `json:"priority,omitempty"`
	Description          string            `json:"description,omitempty"`
	AcceptanceCriteria   string            `json:"acceptance_criteria,omitempty"`
	RequirementAnalysis  string            `json:"requirement_analysis"`
	RequirementItems     []RequirementItem `json:"requirement_items,omitempty"`
	RepoTraceability     string            `json:"repo_traceability"`
	CoverageStatus       string            `json:"coverage_status"`
	CoverageScore        int               `json:"coverage_score"`
	Evidence             string            `json:"evidence"`
	MissingScenarios     []string          `json:"missing_scenarios,omitempty"`
	RelatedRepos         []string          `json:"related_repos,omitempty"`
	MatchedSignals       []string          `json:"matched_signals,omitempty"`
	ProposedTestLevel    string            `json:"proposed_test_level,omitempty"`
	ProposedTestSteps    string            `json:"proposed_test_steps,omitempty"`
	Recommendation       string            `json:"recommendation,omitempty"`
}

var testIntentTerms = map[string][]string{
	"security":    {"security", "auth", "authorization", "rbac", "permission", "denied", "forbidden", "oidc", "iam", "sts", "credential", "secret"},
	"negative":    {"negative", "invalid", "failure", "error", "denied", "timeout", "reject", "malformed", "unauthorized"},
	"resilience":  {"resilience", "recovery", "retry", "failover", "outage", "deletion", "cleanup", "finalizer", "rollback"},
	"e2e":         {"e2e", "end-to-end", "workflow", "integration", "system", "tenant", "provision", "deploy", "sync"},
	"upgrade":     {"upgrade", "migration", "version", "rollback"},
	"performance": {"performance", "latency", "throughput", "load", "scale"},
}

func AnalyzeInputCases(issues []jira.Issue, res *Result) []InputTestCaseResult {
	if len(issues) == 0 {
		return nil
	}

	var out []InputTestCaseResult
	for _, issue := range issues {
		out = append(out, analyzeInputCase(issue, res))
	}
	return out
}

func analyzeInputCase(issue jira.Issue, res *Result) InputTestCaseResult {
	text := strings.ToLower(issue.Text())
	intents := detectIntents(text)
	related := relatedRepos(text, res.Repos)
	matched := matchedSignals(intents, res.Repos)
	requirementItems := buildRequirementItems(issue, intents, res)
	missing := missingFromRequirements(requirementItems, intents, res)
	requirementAnalysis := buildRequirementAnalysis(issue, intents, requirementItems)
	repoTraceability := buildRepoTraceability(issue, related, matched, missing, res)

	score := coverageScoreFromRequirements(requirementItems, len(matched), len(missing), res)
	status := coverageStatus(score, len(missing))

	result := InputTestCaseResult{
		Key:                issue.Key,
		Summary:            issue.Summary,
		URL:                issue.URL,
		ParentKey:          issue.ParentKey,
		IssueType:          issue.IssueType,
		Status:             issue.Status,
		Priority:           issue.Priority,
		Description:        trimForReport(issue.Description, 1200),
		AcceptanceCriteria: issue.AcceptanceCriteria,
		RequirementAnalysis: requirementAnalysis,
		RequirementItems:   requirementItems,
		RepoTraceability:   repoTraceability,
		CoverageStatus:     status,
		CoverageScore:      score,
		RelatedRepos:       related,
		MatchedSignals:     matched,
		MissingScenarios:   missing,
		Evidence:           buildInputEvidence(issue, requirementAnalysis, repoTraceability),
		ProposedTestLevel:  proposeTestLevel(intents),
		ProposedTestSteps:  proposeTestSteps(issue, requirementItems, missing),
		Recommendation:     recommendInputCase(status, issue, missing),
	}
	return result
}

func buildRequirementItems(issue jira.Issue, intents []string, res *Result) []RequirementItem {
	lines := parseRequirementLines(issue)
	if len(lines) == 0 {
		lines = []string{issue.Summary}
	}

	var items []RequirementItem
	for _, line := range lines {
		items = append(items, RequirementItem{
			Text:           line,
			CoverageStatus: requirementCoverage(line, intents, res),
			RepoEvidence:   requirementRepoEvidence(line, res),
		})
	}
	return items
}

func parseRequirementLines(issue jira.Issue) []string {
	var lines []string
	for _, block := range []string{issue.AcceptanceCriteria, issue.Description} {
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimLeft(line, "*-•")
			line = strings.TrimSpace(line)
			if len(line) < 8 {
				continue
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "acceptance criteria") || strings.HasPrefix(lower, "description") {
				continue
			}
			lines = append(lines, line)
		}
	}
	return uniqueStrings(lines)
}

func requirementCoverage(line string, intents []string, res *Result) string {
	lineLower := strings.ToLower(line)
	itemIntents := detectIntents(lineLower)
	if len(itemIntents) == 0 {
		itemIntents = intents
	}
	matched := matchedSignals(itemIntents, res.Repos)
	if len(matched) > 0 {
		return "PARTIAL"
	}
	totalTests := 0
	for _, repo := range res.Repos {
		totalTests += repo.TestFiles + repo.SpecFiles
	}
	if totalTests == 0 {
		return "NOT COVERED"
	}
	return "UNKNOWN"
}

func requirementRepoEvidence(line string, res *Result) string {
	lineLower := strings.ToLower(line)
	var hits []string
	for _, repo := range res.Repos {
		for term, count := range repo.CriticalHits {
			if count > 0 && strings.Contains(lineLower, term) {
				hits = append(hits, fmt.Sprintf("%s references %s (%d)", repo.Name, term, count))
			}
		}
	}
	if len(hits) == 0 {
		return "No repository implementation keyword overlap detected for this requirement line."
	}
	return strings.Join(hits, "; ")
}

func buildRequirementAnalysis(issue jira.Issue, intents []string, items []RequirementItem) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Jira %s defines a requirement-driven test scope: %s.", issue.Key, issue.Summary))
	if len(intents) > 0 {
		parts = append(parts, "Required test themes from the card: "+strings.Join(intents, ", ")+".")
	}
	if len(items) > 0 {
		parts = append(parts, fmt.Sprintf("The card contains %d testable requirement statement(s) derived from its description and acceptance criteria.", len(items)))
	}
	notCovered := 0
	for _, item := range items {
		if item.CoverageStatus == "NOT COVERED" || item.CoverageStatus == "UNKNOWN" {
			notCovered++
		}
	}
	if notCovered > 0 {
		parts = append(parts, fmt.Sprintf("%d requirement statement(s) have no confirmed automated validation.", notCovered))
	}
	return strings.Join(parts, " ")
}

func buildRepoTraceability(issue jira.Issue, related, matched, missing []string, res *Result) string {
	var parts []string
	parts = append(parts, "Repository traceability maps the Jira requirement to static evidence in cloned repositories (test files, keywords, and CI signals).")
	if len(related) > 0 {
		parts = append(parts, "Related repositories: "+strings.Join(related, ", ")+".")
	}
	if len(matched) > 0 {
		parts = append(parts, "Matched repository test signals: "+strings.Join(matched, "; ")+".")
	} else {
		parts = append(parts, "No matching executable test signals were found for themes in this card.")
	}
	totalTests := 0
	for _, repo := range res.Repos {
		totalTests += repo.TestFiles + repo.SpecFiles
	}
	parts = append(parts, fmt.Sprintf("Total executable test/spec files across analyzed repos: %d.", totalTests))
	if len(missing) > 0 {
		parts = append(parts, "Traceability gaps: "+strings.Join(missing, "; ")+".")
	}
	return strings.Join(parts, " ")
}

func missingFromRequirements(items []RequirementItem, intents []string, res *Result) []string {
	var missing []string
	for _, item := range items {
		if item.CoverageStatus == "NOT COVERED" || item.CoverageStatus == "UNKNOWN" {
			missing = append(missing, fmt.Sprintf("Requirement not evidenced in repos: %s", item.Text))
		}
	}
	for _, intent := range intents {
		total := intentSignalTotal(intent, res.Repos)
		if total == 0 {
			switch intent {
			case "security":
				missing = append(missing, "Jira card implies security validation, but no security test signals exist in repositories.")
			case "negative":
				missing = append(missing, "Jira card implies negative/failure-path validation, but no negative test signals exist in repositories.")
			case "resilience":
				missing = append(missing, "Jira card implies resilience/recovery validation, but no resilience test signals exist in repositories.")
			case "e2e":
				missing = append(missing, "Jira card implies an end-to-end workflow, but no E2E/system test signals exist in repositories.")
			case "upgrade":
				missing = append(missing, "Jira card implies upgrade/rollback validation, but no upgrade test signals exist in repositories.")
			case "performance":
				missing = append(missing, "Jira card implies performance/load validation, but no performance test signals exist in repositories.")
			}
		}
	}
	return uniqueStrings(missing)
}

func detectIntents(text string) []string {
	var intents []string
	for intent, terms := range testIntentTerms {
		for _, term := range terms {
			if strings.Contains(text, term) {
				intents = append(intents, intent)
				break
			}
		}
	}
	return intents
}

func relatedRepos(text string, repos []RepoResult) []string {
	var names []string
	for _, repo := range repos {
		if strings.Contains(strings.ToLower(text), strings.ToLower(repo.Name)) {
			names = append(names, repo.Name)
		}
	}
	if len(names) == 0 && len(repos) > 0 {
		for _, repo := range repos {
			if repo.Role == "application" {
				names = append(names, repo.Name)
				break
			}
		}
	}
	return names
}

func matchedSignals(intents []string, repos []RepoResult) []string {
	var matched []string
	for _, intent := range intents {
		total := intentSignalTotal(intent, repos)
		if total > 0 {
			matched = append(matched, fmt.Sprintf("%s (%d signals)", intent, total))
		}
	}
	return matched
}

func intentSignalTotal(intent string, repos []RepoResult) int {
	total := 0
	for _, repo := range repos {
		if intent == "e2e" {
			total += repo.TestLevels["e2e"] + repo.TestLevels["system"] + repo.TestLevels["integration"]
		} else {
			total += repo.CrossCutting[intent]
		}
	}
	return total
}

func coverageScoreFromRequirements(items []RequirementItem, matchedCount, missingCount int, res *Result) int {
	if len(items) == 0 {
		return coverageScore(1, matchedCount, missingCount, res)
	}
	covered := 0
	for _, item := range items {
		if item.CoverageStatus == "PARTIAL" || item.CoverageStatus == "COVERED" {
			covered++
		}
	}
	base := (covered * 70) / len(items)
	if matchedCount > 0 {
		base += 15
	}
	base -= missingCount * 8
	for _, repo := range res.Repos {
		if repo.TestFiles+repo.SpecFiles == 0 && repo.Files > 20 {
			base -= 10
		}
	}
	if base < 0 {
		base = 0
	}
	if base > 100 {
		base = 100
	}
	return base
}

func coverageScore(intentCount, matchedCount, missingCount int, res *Result) int {
	if intentCount == 0 {
		intentCount = 1
	}
	base := 20
	if matchedCount > 0 {
		base += (matchedCount * 60) / intentCount
	}
	base -= missingCount * 12
	for _, repo := range res.Repos {
		if repo.TestFiles+repo.SpecFiles == 0 && repo.Files > 20 {
			base -= 15
		}
	}
	if base < 0 {
		base = 0
	}
	if base > 100 {
		base = 100
	}
	return base
}

func coverageStatus(score int, missingCount int) string {
	switch {
	case missingCount == 0 && score >= 70:
		return "COVERED"
	case score >= 40:
		return "PARTIAL"
	case score > 0:
		return "GAP"
	default:
		return "NOT COVERED"
	}
}

func buildInputEvidence(issue jira.Issue, requirementAnalysis, repoTraceability string) string {
	return fmt.Sprintf("Requirement analysis: %s Repository traceability: %s", requirementAnalysis, repoTraceability)
}

func proposeTestLevel(intents []string) string {
	for _, intent := range intents {
		switch intent {
		case "security", "negative":
			return "Integration / E2E"
		case "resilience":
			return "Integration / E2E"
		case "e2e":
			return "E2E / System"
		case "upgrade":
			return "E2E / System"
		case "performance":
			return "Performance / Load"
		}
	}
	return "Integration / E2E"
}

func proposeTestSteps(issue jira.Issue, items []RequirementItem, missing []string) string {
	steps := []string{
		fmt.Sprintf("1. Treat Jira %s as the source requirement: %s.", issue.Key, issue.Summary),
	}
	if len(items) > 0 {
		steps = append(steps, "2. Convert each requirement statement below into executable assertions.")
	} else if issue.AcceptanceCriteria != "" {
		steps = append(steps, "2. Translate each acceptance criterion into an executable assertion.")
	} else {
		steps = append(steps, "2. Derive assertions from the Jira description and expected behavior.")
	}
	step := 3
	for _, item := range items {
		if item.CoverageStatus == "NOT COVERED" || item.CoverageStatus == "UNKNOWN" {
			steps = append(steps, fmt.Sprintf("%d. Implement validation for: %s", step, item.Text))
			step++
		}
	}
	for _, gap := range missing {
		steps = append(steps, fmt.Sprintf("%d. Close repository traceability gap: %s", step, gap))
		step++
	}
	steps = append(steps, fmt.Sprintf("%d. Automate in CI and link results back to %s.", step, issue.Key))
	return strings.Join(steps, "\n")
}

func recommendInputCase(status string, issue jira.Issue, missing []string) string {
	switch status {
	case "COVERED":
		return fmt.Sprintf("Requirement statements in %s appear partially reflected in repository tests; confirm each acceptance criterion before closing the card.", issue.Key)
	case "PARTIAL":
		return fmt.Sprintf("Implement missing validations for %s based on the Jira requirement statements, then confirm repository traceability.", issue.Key)
	default:
		if len(missing) > 0 {
			return fmt.Sprintf("Create automated tests for %s from the Jira requirement first, then add repository coverage for: %s", issue.Key, strings.Join(missing, "; "))
		}
		return fmt.Sprintf("Create automated tests mapped directly to the Jira requirement in %s.", issue.Key)
	}
}

func trimForReport(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
