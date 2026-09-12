package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

// JiraCategorySummary aggregates Jira cards by test category for risk analysis.
type JiraCategorySummary struct {
	Category     string   `json:"category"`
	CardCount    int      `json:"card_count"`
	Keys         []string `json:"keys"`
	RiskLevel    string   `json:"risk_level"`
	RepoSupport  string   `json:"repo_support"`
	RiskAnalysis string   `json:"risk_analysis"`
	Deprioritized bool    `json:"deprioritized"`
}

// JiraSummary provides category-level Jira analysis without per-card verbosity.
type JiraSummary struct {
	TotalCards      int                   `json:"total_cards"`
	Categories      []JiraCategorySummary `json:"categories"`
	OverallAnalysis string                `json:"overall_analysis"`
	TopRisks        []string              `json:"top_risks,omitempty"`
	FlaggedCount    int                   `json:"flagged_count"`
}

// BuildJiraSummary groups analyzed Jira cards by category for risk assessment.
func BuildJiraSummary(cases []InputTestCaseResult, cfg *config.Config) JiraSummary {
	if len(cases) == 0 {
		return JiraSummary{}
	}

	byCategory := map[string][]InputTestCaseResult{}
	for _, tc := range cases {
		cat := tc.PrimaryTestCategory
		if cat == "" {
			cat = "e2e"
		}
		byCategory[cat] = append(byCategory[cat], tc)
	}

	order := defaultCategoryOrder(cfg)
	seen := map[string]bool{}
	var categories []JiraCategorySummary
	appendCategory := func(cat string) {
		if seen[cat] {
			return
		}
		group := byCategory[cat]
		if len(group) == 0 {
			return
		}
		seen[cat] = true
		categories = append(categories, summarizeCategory(cat, group, cfg))
	}
	for _, cat := range order {
		appendCategory(cat)
	}
	for cat, group := range byCategory {
		if !seen[cat] {
			categories = append(categories, summarizeCategory(cat, group, cfg))
		}
	}

	summary := JiraSummary{
		TotalCards: len(cases),
		Categories: categories,
		TopRisks:   topJiraRisks(categories, cfg),
	}
	for _, tc := range cases {
		if CardNeedsAttention(tc) {
			summary.FlaggedCount++
		}
	}
	summary.OverallAnalysis = buildOverallJiraAnalysis(summary, cfg)
	return summary
}

func summarizeCategory(cat string, group []InputTestCaseResult, cfg *config.Config) JiraCategorySummary {
	keys := make([]string, 0, len(group))
	covered, partial, jiraOnly, needsDetail := 0, 0, 0, 0
	for _, tc := range group {
		keys = append(keys, tc.Key)
		switch tc.CoverageStatus {
		case "COVERED":
			covered++
		case "PARTIAL":
			partial++
		case "JIRA_DEFINED":
			jiraOnly++
		case "NEEDS_DETAIL":
			needsDetail++
		}
	}
	sort.Strings(keys)

	repoSupport := "NONE"
	switch {
	case covered > 0 && jiraOnly == 0 && partial == 0:
		repoSupport = "SUPPORTED"
	case covered > 0 || partial > 0:
		repoSupport = "PARTIAL"
	}

	riskLevel := categoryRiskLevel(cat, covered, partial, jiraOnly, needsDetail, cfg)
	analysis := fmt.Sprintf(
		"%d Jira card(s) in category %s: %d covered, %d partial, %d Jira-only, %d need more detail.",
		len(group), cat, covered, partial, jiraOnly, needsDetail,
	)
	if repoSupport == "NONE" {
		analysis += " No matching repository test evidence; rely on Jira requirements and live E2E execution."
	} else if repoSupport == "PARTIAL" {
		analysis += " Some repository test signals exist but coverage is incomplete."
	} else {
		analysis += " Repository tests support this category."
	}

	return JiraCategorySummary{
		Category:      cat,
		CardCount:     len(group),
		Keys:          keys,
		RiskLevel:     riskLevel,
		RepoSupport:   repoSupport,
		RiskAnalysis:  analysis,
		Deprioritized: cfg != nil && cfg.IsDeprioritized(cat),
	}
}

func categoryRiskLevel(cat string, covered, partial, jiraOnly, needsDetail int, cfg *config.Config) string {
	if cfg != nil && cfg.IsDeprioritized(cat) {
		if covered > 0 {
			return "LOW"
		}
		return "MEDIUM"
	}
	if needsDetail > 0 {
		return "MEDIUM"
	}
	if jiraOnly > 0 && covered == 0 {
		switch cat {
		case "security", "negative", "gitops", "multi_tenancy", "customer_impact":
			return "HIGH"
		default:
			return "MEDIUM"
		}
	}
	if partial > 0 {
		return "MEDIUM"
	}
	if covered > 0 {
		return "LOW"
	}
	return "MEDIUM"
}

func topJiraRisks(categories []JiraCategorySummary, cfg *config.Config) []string {
	var risks []string
	for _, cat := range categories {
		if cat.Deprioritized {
			continue
		}
		switch cat.RiskLevel {
		case "HIGH":
			risks = append(risks, fmt.Sprintf("%s (%d cards: %s): %s", cat.Category, cat.CardCount, strings.Join(cat.Keys, ", "), cat.RiskAnalysis))
		case "MEDIUM":
			if cat.RepoSupport == "NONE" || cat.RepoSupport == "PARTIAL" {
				risks = append(risks, fmt.Sprintf("%s (%d cards): %s", cat.Category, cat.CardCount, cat.RiskAnalysis))
			}
		}
	}
	return risks
}

func buildOverallJiraAnalysis(summary JiraSummary, cfg *config.Config) string {
	if summary.TotalCards == 0 {
		return ""
	}
	parts := []string{
		fmt.Sprintf("Analyzed %d Jira card(s) across %d test categories from card descriptions.", summary.TotalCards, len(summary.Categories)),
	}
	if summary.FlaggedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d card(s) need individual review due to gaps or missing detail.", summary.FlaggedCount))
	} else {
		parts = append(parts, "No individual card review is required; category-level evidence is sufficient.")
	}
	if cfg != nil && len(cfg.Risk.Deprioritize) > 0 {
		parts = append(parts, "Deprioritized categories: "+strings.Join(cfg.Risk.Deprioritize, ", ")+".")
	}
	return strings.Join(parts, " ")
}

// CardNeedsAttention reports whether a Jira card warrants individual drill-down.
func CardNeedsAttention(tc InputTestCaseResult) bool {
	switch tc.CoverageStatus {
	case "COVERED":
		return false
	case "NEEDS_DETAIL":
		return true
	case "JIRA_DEFINED":
		return true
	case "PARTIAL":
		return true
	}
	if tc.PriorityNote != "" {
		return true
	}
	return tc.CoverageScore < 50
}
