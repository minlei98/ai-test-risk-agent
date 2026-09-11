package analyzer

import (
	"fmt"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/jira"
)

// InputTestCaseResult captures coverage analysis for a Jira input test case.
type InputTestCaseResult struct {
	Key                string   `json:"key"`
	Summary            string   `json:"summary"`
	URL                string   `json:"url,omitempty"`
	ParentKey          string   `json:"parent_key,omitempty"`
	IssueType          string   `json:"issue_type,omitempty"`
	Status             string   `json:"status,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	CoverageStatus     string   `json:"coverage_status"`
	CoverageScore      int      `json:"coverage_score"`
	Evidence           string   `json:"evidence"`
	MissingScenarios   []string `json:"missing_scenarios,omitempty"`
	RelatedRepos       []string `json:"related_repos,omitempty"`
	MatchedSignals     []string `json:"matched_signals,omitempty"`
	ProposedTestLevel  string   `json:"proposed_test_level,omitempty"`
	ProposedTestSteps  string   `json:"proposed_test_steps,omitempty"`
	Recommendation     string   `json:"recommendation,omitempty"`
}

var testIntentTerms = map[string][]string{
	"security":   {"security", "auth", "authorization", "rbac", "permission", "denied", "forbidden", "oidc", "iam", "sts", "credential", "secret"},
	"negative":   {"negative", "invalid", "failure", "error", "denied", "timeout", "reject", "malformed", "unauthorized"},
	"resilience": {"resilience", "recovery", "retry", "failover", "outage", "deletion", "cleanup", "finalizer", "rollback"},
	"e2e":        {"e2e", "end-to-end", "workflow", "integration", "system", "tenant", "provision", "deploy", "sync"},
	"upgrade":    {"upgrade", "migration", "version", "rollback"},
	"performance":{"performance", "latency", "throughput", "load", "scale"},
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
	missing := missingScenarios(intents, res.Repos)

	score := coverageScore(len(intents), len(matched), len(missing), res)
	status := coverageStatus(score, len(missing))

	result := InputTestCaseResult{
		Key:            issue.Key,
		Summary:        issue.Summary,
		URL:            issue.URL,
		ParentKey:      issue.ParentKey,
		IssueType:      issue.IssueType,
		Status:         issue.Status,
		Priority:       issue.Priority,
		CoverageStatus: status,
		CoverageScore:  score,
		RelatedRepos:   related,
		MatchedSignals: matched,
		MissingScenarios: missing,
		Evidence:       buildInputEvidence(issue, intents, matched, missing, res),
		ProposedTestLevel: proposeTestLevel(intents),
		ProposedTestSteps: proposeTestSteps(issue, intents, missing),
		Recommendation: recommendInputCase(status, issue, missing),
	}
	return result
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
		total := 0
		for _, repo := range repos {
			if intent == "e2e" {
				total += repo.TestLevels["e2e"] + repo.TestLevels["system"] + repo.TestLevels["integration"]
			} else {
				total += repo.CrossCutting[intent]
			}
		}
		if total > 0 {
			matched = append(matched, fmt.Sprintf("%s (%d signals)", intent, total))
		}
	}
	return matched
}

func missingScenarios(intents []string, repos []RepoResult) []string {
	var missing []string
	for _, intent := range intents {
		total := 0
		for _, repo := range repos {
			if intent == "e2e" {
				total += repo.TestLevels["e2e"] + repo.TestLevels["system"] + repo.TestLevels["integration"]
			} else {
				total += repo.CrossCutting[intent]
			}
		}
		if total == 0 {
			switch intent {
			case "security":
				missing = append(missing, "No explicit security test evidence matches this Jira card.")
			case "negative":
				missing = append(missing, "No explicit negative/failure-path test evidence matches this Jira card.")
			case "resilience":
				missing = append(missing, "No explicit resilience/recovery test evidence matches this Jira card.")
			case "e2e":
				missing = append(missing, "No E2E/system/integration test evidence matches this Jira workflow.")
			case "upgrade":
				missing = append(missing, "No upgrade/rollback test evidence matches this Jira card.")
			case "performance":
				missing = append(missing, "No performance/load test evidence matches this Jira card.")
			}
		}
	}

	totalTests := 0
	for _, repo := range repos {
		totalTests += repo.TestFiles + repo.SpecFiles
	}
	if totalTests == 0 {
		missing = append(missing, "No executable test files were detected in analyzed repositories.")
	}
	return missing
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

func buildInputEvidence(issue jira.Issue, intents, matched, missing []string, res *Result) string {
	parts := []string{
		fmt.Sprintf("Jira %s: %s", issue.Key, issue.Summary),
	}
	if len(intents) > 0 {
		parts = append(parts, "Detected test intents: "+strings.Join(intents, ", "))
	}
	if len(matched) > 0 {
		parts = append(parts, "Matched repository signals: "+strings.Join(matched, "; "))
	}
	if len(missing) > 0 {
		parts = append(parts, "Gaps: "+strings.Join(missing, "; "))
	}
	totalTests := 0
	for _, repo := range res.Repos {
		totalTests += repo.TestFiles + repo.SpecFiles
	}
	parts = append(parts, fmt.Sprintf("Total executable test/spec files across repos: %d", totalTests))
	return strings.Join(parts, ". ")
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

func proposeTestSteps(issue jira.Issue, intents []string, missing []string) string {
	steps := []string{
		fmt.Sprintf("1. Use Jira %s (%s) as the source requirement.", issue.Key, issue.Summary),
	}
	if issue.AcceptanceCriteria != "" {
		steps = append(steps, "2. Translate each acceptance criterion into an executable assertion.")
	} else {
		steps = append(steps, "2. Derive assertions from the Jira description and expected behavior.")
	}
	step := 3
	for _, gap := range missing {
		steps = append(steps, fmt.Sprintf("%d. Close gap: %s", step, gap))
		step++
	}
	if containsIntent(intents, "negative") || containsIntent(intents, "security") {
		steps = append(steps, fmt.Sprintf("%d. Add denied-permission and invalid-input scenarios; assert explicit failure responses.", step))
		step++
	}
	steps = append(steps, fmt.Sprintf("%d. Automate in CI and link results back to %s.", step, issue.Key))
	return strings.Join(steps, "\n")
}

func recommendInputCase(status string, issue jira.Issue, missing []string) string {
	switch status {
	case "COVERED":
		return fmt.Sprintf("Repository evidence suggests existing tests may cover %s; validate mapped assertions against the Jira acceptance criteria before closing.", issue.Key)
	case "PARTIAL":
		return fmt.Sprintf("Add targeted tests for %s focusing on uncovered intents and acceptance criteria.", issue.Key)
	default:
		if len(missing) > 0 {
			return fmt.Sprintf("Create a new automated test case for %s addressing: %s", issue.Key, strings.Join(missing, "; "))
		}
		return fmt.Sprintf("Create a new automated test case mapped directly to Jira %s.", issue.Key)
	}
}

func containsIntent(intents []string, target string) bool {
	for _, intent := range intents {
		if intent == target {
			return true
		}
	}
	return false
}
