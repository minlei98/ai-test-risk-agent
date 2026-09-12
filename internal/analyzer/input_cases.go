package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
	"github.com/minlei98/ai-test-risk-agent/internal/jira"
)

const (
	AnalysisModeRepository = "repository"
	AnalysisModeHybrid     = "hybrid"
)

// RequirementItem is one testable statement extracted from a Jira card.
type RequirementItem struct {
	Text           string `json:"text"`
	Status         string `json:"status"`
	JiraEvidence   string `json:"jira_evidence,omitempty"`
	RepoEvidence   string `json:"repo_evidence,omitempty"`
}

// InputTestCaseResult combines Jira requirement evidence with optional repository test evidence.
type InputTestCaseResult struct {
	Key                 string            `json:"key"`
	Summary             string            `json:"summary"`
	URL                 string            `json:"url,omitempty"`
	ParentKey           string            `json:"parent_key,omitempty"`
	IssueType           string            `json:"issue_type,omitempty"`
	Status              string            `json:"status,omitempty"`
	Priority            string            `json:"priority,omitempty"`
	Description         string            `json:"description,omitempty"`
	AcceptanceCriteria  string            `json:"acceptance_criteria,omitempty"`
	PrimaryTestCategory string            `json:"primary_test_category"`
	TestCategories      []string          `json:"test_categories,omitempty"`
	RiskDomains         []string          `json:"risk_domains,omitempty"`
	TestLevel           string            `json:"test_level"`
	RequirementAnalysis string            `json:"requirement_analysis"`
	RequirementItems    []RequirementItem `json:"requirement_items,omitempty"`
	JiraEvidence        string            `json:"jira_evidence"`
	RepoTestEvidence    string            `json:"repo_test_evidence"`
	RepoTraceability    string            `json:"repo_traceability"`
	CoverageStatus      string            `json:"coverage_status"`
	CoverageScore       int               `json:"coverage_score"`
	Evidence            string            `json:"evidence"`
	E2EScenarios        []string          `json:"e2e_scenarios,omitempty"`
	RelatedRepos        []string          `json:"related_repos,omitempty"`
	MatchedSignals      []string          `json:"matched_signals,omitempty"`
	ProposedTestLevel   string            `json:"proposed_test_level,omitempty"`
	ProposedTestSteps   string            `json:"proposed_test_steps,omitempty"`
	Recommendation      string            `json:"recommendation,omitempty"`
	PriorityNote        string            `json:"priority_note,omitempty"`
}

var testCategoryTerms = map[string][]string{
	"security":        {"security", "auth", "authorization", "rbac", "permission", "denied", "forbidden", "oidc", "iam", "sts", "credential", "secret"},
	"gitops":          {"gitops", "argocd", "application", "applicationset", "sync", "kustomize", "helm", "rollout", "progressive", "delivery", "drift"},
	"multi_tenancy":   {"multi-tenant", "multitenant", "tenant", "namespace", "project", "isolation", "cross-tenant"},
	"customer_impact": {"customer", "production", "outage", "sla", "downtime", "critical path", "user-facing", "customer-facing"},
	"resilience":      {"resilience", "recovery", "retry", "failover", "outage", "deletion", "cleanup", "finalizer", "rollback"},
	"upgrade":         {"upgrade", "migration", "version", "compatibility"},
	"negative":        {"negative", "invalid", "failure", "error", "denied", "timeout", "reject", "malformed", "unauthorized"},
	"functionality":   {"functional", "functionality", "feature", "behavior", "capability", "regression", "parity"},
	"integration":     {"integration", "deploy", "provision", "envtest", "contract"},
	"e2e":             {"e2e", "end-to-end", "workflow", "system", "live", "hub", "saas", "smoke", "sanity"},
	"performance":     {"performance", "latency", "throughput", "load", "scale", "benchmark", "stress"},
}

var riskDomainTerms = map[string][]string{
	"security":      {"iam", "sts", "oidc", "rbac", "secret", "credential", "token", "certificate", "networkpolicy"},
	"gitops":          {"argocd", "application", "applicationset", "sync", "kustomize", "helm", "gitops", "rollout"},
	"reliability":     {"retry", "timeout", "reconcile", "controller", "webhook", "health", "readiness", "cleanup", "finalizer"},
	"multi_tenancy":   {"tenant", "namespace", "project", "isolation"},
	"data":            {"database", "postgres", "pvc", "storage", "backup", "restore"},
	"networking":      {"service", "ingress", "route", "loadbalancer", "networkpolicy", "dns"},
}

// AnalyzeInputCases classifies Jira cards and combines Jira + repository evidence.
func AnalyzeInputCases(issues []jira.Issue, res *Result, cfg *config.Config) []InputTestCaseResult {
	if len(issues) == 0 {
		return nil
	}
	res.AnalysisMode = AnalysisModeHybrid
	if cfg != nil {
		res.DeprioritizedCategories = cfg.Risk.Deprioritize
	}

	var out []InputTestCaseResult
	for _, issue := range issues {
		out = append(out, analyzeInputCase(issue, res, cfg))
	}
	res.JiraSummary = BuildJiraSummary(out, cfg)
	return out
}

func analyzeInputCase(issue jira.Issue, res *Result, cfg *config.Config) InputTestCaseResult {
	text := strings.ToLower(issue.Text())
	categories := categorizeTestCategories(text, cfg)
	riskDomains := categorizeRiskDomains(text)
	primary := primaryTestCategory(categories, cfg)
	related := relatedRepos(text, res.Repos)
	matched := matchedSignals(categories, res.Repos)
	requirementItems := buildRequirementItems(issue, categories, res)
	scenarios := scenariosFromJira(issue, requirementItems, categories, cfg)
	jiraEvidence := buildJiraEvidence(issue, categories, requirementItems)
	repoTestEvidence := buildRepoTestEvidence(categories, matched, res)
	repoTraceability := buildRepoTraceability(related, matched, categories, res)
	requirementAnalysis := buildRequirementAnalysis(issue, primary, categories, riskDomains, requirementItems)

	score := combinedCoverageScore(issue, requirementItems, matched, cfg)
	status := combinedCoverageStatus(issue, requirementItems, matched)

	return InputTestCaseResult{
		Key:                 issue.Key,
		Summary:             issue.Summary,
		URL:                 issue.URL,
		ParentKey:           issue.ParentKey,
		IssueType:           issue.IssueType,
		Status:              issue.Status,
		Priority:            issue.Priority,
		Description:         trimForReport(issue.Description, 1200),
		AcceptanceCriteria:  issue.AcceptanceCriteria,
		PrimaryTestCategory: primary,
		TestCategories:      categories,
		RiskDomains:         riskDomains,
		TestLevel:           proposeTestLevel(categories, cfg),
		RequirementAnalysis: requirementAnalysis,
		RequirementItems:    requirementItems,
		JiraEvidence:        jiraEvidence,
		RepoTestEvidence:    repoTestEvidence,
		RepoTraceability:    repoTraceability,
		CoverageStatus:      status,
		CoverageScore:       score,
		RelatedRepos:        related,
		MatchedSignals:      matched,
		E2EScenarios:        scenarios,
		Evidence:            buildCombinedEvidence(jiraEvidence, repoTestEvidence),
		ProposedTestLevel:   proposeTestLevel(categories, cfg),
		ProposedTestSteps:   proposeTestSteps(issue, primary, requirementItems, scenarios, matched),
		Recommendation:      recommendInputCase(status, issue, primary, matched, requirementItems, cfg),
		PriorityNote:        priorityNote(primary, categories, cfg),
	}
}

func categorizeTestCategories(text string, cfg *config.Config) []string {
	var cats []string
	for cat, terms := range testCategoryTerms {
		for _, term := range terms {
			if strings.Contains(text, term) {
				cats = append(cats, cat)
				break
			}
		}
	}
	if len(cats) == 0 {
		cats = []string{"e2e"}
	}
	sort.Strings(cats)
	return cats
}

func categorizeRiskDomains(text string) []string {
	var domains []string
	for domain, terms := range riskDomainTerms {
		for _, term := range terms {
			if strings.Contains(text, term) {
				domains = append(domains, domain)
				break
			}
		}
	}
	sort.Strings(domains)
	return domains
}

func primaryTestCategory(categories []string, cfg *config.Config) string {
	order := defaultCategoryOrder(cfg)
	for _, preferred := range order {
		for _, cat := range categories {
			if cat == preferred {
				return cat
			}
		}
	}
	if len(categories) > 0 {
		return categories[0]
	}
	return "e2e"
}

func defaultCategoryOrder(cfg *config.Config) []string {
	if cfg != nil {
		return cfg.CategoryPriority()
	}
	return []string{"security", "gitops", "multi_tenancy", "customer_impact", "resilience", "upgrade", "negative", "functionality", "integration", "e2e", "performance"}
}

func priorityNote(primary string, categories []string, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	var deprioritized []string
	for _, cat := range categories {
		if cfg.IsDeprioritized(cat) {
			deprioritized = append(deprioritized, cat)
		}
	}
	if len(deprioritized) == 0 {
		return ""
	}
	focus := preferredFocusCategories(categories, cfg)
	if len(focus) == 0 && len(cfg.Risk.Prioritize) > 0 {
		focus = cfg.Risk.Prioritize
	}
	if len(focus) == 0 {
		return fmt.Sprintf("%s is deprioritized for this project.", strings.Join(deprioritized, ", "))
	}
	return fmt.Sprintf("%s is deprioritized for this project; prioritize %s first.", strings.Join(deprioritized, ", "), strings.Join(focus, ", "))
}

func preferredFocusCategories(categories []string, cfg *config.Config) []string {
	var focus []string
	for _, cat := range categories {
		if cfg.IsDeprioritized(cat) {
			continue
		}
		focus = append(focus, cat)
	}
	return uniqueStrings(focus)
}

func buildRequirementItems(issue jira.Issue, categories []string, res *Result) []RequirementItem {
	lines := parseRequirementLines(issue)
	if len(lines) == 0 {
		lines = []string{issue.Summary}
	}

	var items []RequirementItem
	for _, line := range lines {
		lineCats := categorizeTestCategories(strings.ToLower(line), nil)
		if len(lineCats) == 0 {
			lineCats = categories
		}
		matched := matchedSignals(lineCats, res.Repos)
		items = append(items, RequirementItem{
			Text:         line,
			Status:       itemCoverageStatus(matched),
			JiraEvidence: "Defined in Jira card text",
			RepoEvidence: repoEvidenceForLine(line, lineCats, matched, res),
		})
	}
	return items
}

func itemCoverageStatus(matched []string) string {
	if len(matched) > 0 {
		return "REPO_SUPPORTED"
	}
	return "JIRA_ONLY"
}

func repoEvidenceForLine(line string, categories []string, matched []string, res *Result) string {
	if len(matched) > 0 {
		return "Repository tests detected for categories: " + strings.Join(matched, "; ")
	}
	lineLower := strings.ToLower(line)
	var hits []string
	for _, repo := range res.Repos {
		for term, count := range repo.CriticalHits {
			if count > 0 && strings.Contains(lineLower, term) {
				hits = append(hits, fmt.Sprintf("%s implements %s", repo.Name, term))
			}
		}
	}
	if len(hits) > 0 {
		return "Implementation context in repos: " + strings.Join(hits, "; ")
	}
	return "No matching repository test evidence; Jira remains the primary evidence source."
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

func buildJiraEvidence(issue jira.Issue, categories []string, items []RequirementItem) string {
	parts := []string{
		fmt.Sprintf("Jira %s defines the requirement: %s", issue.Key, issue.Summary),
	}
	if len(categories) > 0 {
		parts = append(parts, "Test categories from card: "+strings.Join(categories, ", "))
	}
	if len(items) > 0 {
		parts = append(parts, fmt.Sprintf("%d requirement statement(s) extracted from Jira.", len(items)))
	}
	return strings.Join(parts, " ")
}

func buildRepoTestEvidence(categories []string, matched []string, res *Result) string {
	if len(matched) == 0 {
		totalTests := 0
		for _, repo := range res.Repos {
			totalTests += repo.TestFiles + repo.SpecFiles
		}
		if totalTests == 0 {
			return "No repository test evidence found for categories " + strings.Join(categories, ", ") + "."
		}
		return "Repositories contain tests, but none clearly match the Jira card categories (" + strings.Join(categories, ", ") + ")."
	}
	return "Repository test evidence supports categories: " + strings.Join(matched, "; ") + "."
}

func buildRequirementAnalysis(issue jira.Issue, primary string, categories, riskDomains []string, items []RequirementItem) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Jira %s is classified under test category **%s**.", issue.Key, primary))
	if len(categories) > 1 {
		parts = append(parts, "Additional categories: "+strings.Join(categories, ", ")+".")
	}
	if len(riskDomains) > 0 {
		parts = append(parts, "Risk domains: "+strings.Join(riskDomains, ", ")+".")
	}
	parts = append(parts, fmt.Sprintf("%d requirement statement(s) were extracted from the Jira card.", len(items)))
	return strings.Join(parts, " ")
}

func buildRepoTraceability(related, matched []string, categories []string, res *Result) string {
	var parts []string
	parts = append(parts, "Repository evidence is used when matching tests exist for the Jira card's test categories.")
	if len(related) > 0 {
		parts = append(parts, "Related repositories: "+strings.Join(related, ", ")+".")
	}
	if len(matched) > 0 {
		parts = append(parts, "Matched repository test signals: "+strings.Join(matched, "; ")+".")
	} else {
		parts = append(parts, "No matching repository tests for categories "+strings.Join(categories, ", ")+"; rely on Jira as primary evidence.")
	}
	return strings.Join(parts, " ")
}

func scenariosFromJira(issue jira.Issue, items []RequirementItem, categories []string, cfg *config.Config) []string {
	var scenarios []string
	for _, item := range items {
		scenarios = append(scenarios, fmt.Sprintf("[%s] %s", primaryTestCategory(categories, cfg), item.Text))
	}
	for _, cat := range categories {
		if cfg != nil && cfg.IsDeprioritized(cat) {
			continue
		}
		scenarios = append(scenarios, fmt.Sprintf("%s validation for %s", cat, issue.Key))
	}
	return uniqueStrings(scenarios)
}

func combinedCoverageScore(issue jira.Issue, items []RequirementItem, matched []string, cfg *config.Config) int {
	score := jiraDefinitionScore(issue, items)
	weightedMatches := 0
	for _, signal := range matched {
		cat := strings.Split(signal, " ")[0]
		if cfg != nil && cfg.IsDeprioritized(cat) {
			continue
		}
		weightedMatches++
	}
	if weightedMatches > 0 {
		score += min(weightedMatches*15, 45)
	}
	repoSupported := 0
	for _, item := range items {
		if item.Status == "REPO_SUPPORTED" {
			repoSupported++
		}
	}
	if len(items) > 0 {
		score += (repoSupported * 20) / len(items)
	}
	if score > 100 {
		score = 100
	}
	return score
}

func jiraDefinitionScore(issue jira.Issue, items []RequirementItem) int {
	score := 25
	if strings.TrimSpace(issue.Summary) != "" {
		score += 10
	}
	if strings.TrimSpace(issue.Description) != "" {
		score += 10
	}
	if strings.TrimSpace(issue.AcceptanceCriteria) != "" {
		score += 15
	}
	score += min(len(items)*5, 20)
	return score
}

func combinedCoverageStatus(issue jira.Issue, items []RequirementItem, matched []string) string {
	repoSupported := 0
	for _, item := range items {
		if item.Status == "REPO_SUPPORTED" {
			repoSupported++
		}
	}
	hasJiraDetail := strings.TrimSpace(issue.AcceptanceCriteria) != "" || strings.TrimSpace(issue.Description) != "" || len(items) > 0

	switch {
	case len(matched) > 0 && repoSupported > 0:
		return "COVERED"
	case len(matched) > 0 || repoSupported > 0:
		return "PARTIAL"
	case hasJiraDetail:
		return "JIRA_DEFINED"
	default:
		return "NEEDS_DETAIL"
	}
}

func relatedRepos(text string, repos []RepoResult) []string {
	var names []string
	for _, repo := range repos {
		if strings.Contains(strings.ToLower(text), strings.ToLower(repo.Name)) {
			names = append(names, repo.Name)
		}
	}
	return names
}

func matchedSignals(categories []string, repos []RepoResult) []string {
	var matched []string
	for _, cat := range categories {
		total := categorySignalTotal(cat, repos)
		if total > 0 {
			matched = append(matched, fmt.Sprintf("%s (%d signals)", cat, total))
		}
	}
	return matched
}

func categorySignalTotal(category string, repos []RepoResult) int {
	total := 0
	for _, repo := range repos {
		switch category {
		case "e2e", "integration", "functionality", "customer_impact":
			total += repo.TestLevels["e2e"] + repo.TestLevels["system"] + repo.TestLevels["integration"] + repo.TestLevels["functional"]
		case "gitops":
			total += repo.TestLevels["integration"] + repo.TestLevels["e2e"]
			total += repo.CriticalHits["argocd"] + repo.CriticalHits["application"] + repo.CriticalHits["applicationset"] + repo.CriticalHits["sync"]
		case "multi_tenancy":
			total += repo.CriticalHits["tenant"] + repo.CriticalHits["namespace"]
			total += repo.CrossCutting["negative"]
		default:
			total += repo.CrossCutting[category]
			total += repo.TestLevels[category]
		}
	}
	return total
}

func buildCombinedEvidence(jiraEvidence, repoTestEvidence string) string {
	return fmt.Sprintf("Jira evidence: %s Repository evidence: %s", jiraEvidence, repoTestEvidence)
}

func proposeTestLevel(categories []string, cfg *config.Config) string {
	primary := primaryTestCategory(categories, cfg)
	switch primary {
	case "performance":
		return "E2E / Performance"
	case "security", "negative":
		return "Security / E2E"
	case "gitops", "integration":
		return "Integration / E2E"
	case "multi_tenancy", "customer_impact":
		return "E2E / System"
	case "functionality":
		return "Functional / E2E"
	case "upgrade":
		return "E2E / System"
	default:
		return "E2E / System"
	}
}

func proposeTestSteps(issue jira.Issue, primary string, items []RequirementItem, scenarios []string, matched []string) string {
	steps := []string{
		fmt.Sprintf("1. Classify %s under test category %s.", issue.Key, primary),
		fmt.Sprintf("2. Use Jira %s (%s) as the requirement evidence.", issue.Key, issue.Summary),
	}
	if len(matched) > 0 {
		steps = append(steps, "3. Reuse or extend existing repository tests where signals were detected.")
	} else {
		steps = append(steps, "3. Implement validation in the appropriate E2E/system harness when no repository tests exist.")
	}
	step := 4
	for _, item := range items {
		steps = append(steps, fmt.Sprintf("%d. Validate: %s", step, item.Text))
		step++
	}
	for _, scenario := range scenarios {
		steps = append(steps, fmt.Sprintf("%d. %s", step, scenario))
		step++
	}
	steps = append(steps, fmt.Sprintf("%d. Record results and link evidence back to %s.", step, issue.Key))
	return strings.Join(steps, "\n")
}

func recommendInputCase(status string, issue jira.Issue, primary string, matched []string, items []RequirementItem, cfg *config.Config) string {
	if cfg != nil && cfg.IsDeprioritized(primary) {
		return fmt.Sprintf("%s is classified under deprioritized category %s; address higher-priority categories first unless this card is explicitly in scope.", issue.Key, primary)
	}
	switch status {
	case "COVERED":
		return fmt.Sprintf("%s appears supported by both Jira requirements and repository tests in category %s; confirm acceptance criteria before closing.", issue.Key, primary)
	case "PARTIAL":
		return fmt.Sprintf("Extend repository tests or E2E harness for %s to fully cover the %s requirements in the Jira card.", issue.Key, primary)
	case "JIRA_DEFINED":
		return fmt.Sprintf("%s is well-defined in Jira under %s; implement or run the E2E validation even if repository tests are absent.", issue.Key, primary)
	default:
		return fmt.Sprintf("Add clearer acceptance criteria to %s and classify the validation under %s.", issue.Key, primary)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
