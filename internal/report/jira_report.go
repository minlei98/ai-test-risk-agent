package report

import (
	"fmt"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
)

func writeInputTestCases(b *strings.Builder, r *analyzer.Result) {
	if len(r.InputTestCases) == 0 {
		return
	}

	b.WriteString("## Jira Test Scope Analysis\n\n")
	b.WriteString("Jira cards are grouped by test category derived from descriptions. Individual cards are only expanded when a gap or issue is detected.\n\n")

	if r.JiraSummary.OverallAnalysis != "" {
		fmt.Fprintf(b, "%s\n\n", r.JiraSummary.OverallAnalysis)
	}

	writeJiraCategorySummary(b, r.JiraSummary)

	if len(r.JiraSummary.TopRisks) > 0 {
		b.WriteString("### Category risks from Jira scope\n\n")
		for _, risk := range r.JiraSummary.TopRisks {
			fmt.Fprintf(b, "- %s\n", risk)
		}
		b.WriteString("\n")
	}

	writeJiraCardIndex(b, r.InputTestCases)

	flagged := flaggedJiraCards(r.InputTestCases)
	if len(flagged) > 0 {
		b.WriteString("### Flagged Jira cards\n\n")
		for i, tc := range flagged {
			writeJiraCardDetail(b, i+1, tc)
		}
	} else {
		b.WriteString("No individual Jira cards require drill-down; category-level analysis is sufficient.\n\n")
	}
}

func writeJiraCategorySummary(b *strings.Builder, summary analyzer.JiraSummary) {
	b.WriteString("### Category summary\n\n")
	b.WriteString("| Category | Cards | Risk | Repo support | Jira keys |\n")
	b.WriteString("|---|---:|---|---|---|\n")
	for _, cat := range summary.Categories {
		keys := strings.Join(cat.Keys, ", ")
		if len(keys) > 80 {
			keys = keys[:77] + "..."
		}
		label := cat.Category
		if cat.Deprioritized {
			label += " (deprioritized)"
		}
		fmt.Fprintf(b, "| %s | %d | %s | %s | %s |\n",
			label, cat.CardCount, cat.RiskLevel, cat.RepoSupport, keys)
	}
	b.WriteString("\n")

	for _, cat := range summary.Categories {
		fmt.Fprintf(b, "- **%s:** %s\n", cat.Category, cat.RiskAnalysis)
	}
	b.WriteString("\n")
}

func writeJiraCardIndex(b *strings.Builder, cases []analyzer.InputTestCaseResult) {
	b.WriteString("### Jira card index\n\n")
	b.WriteString("| Jira | Category | Coverage | Summary |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, tc := range cases {
		link := tc.Key
		if tc.URL != "" {
			link = fmt.Sprintf("[%s](%s)", tc.Key, tc.URL)
		}
		category := tc.PrimaryTestCategory
		if category == "" {
			category = "—"
		}
		summary := tc.Summary
		if len(summary) > 72 {
			summary = summary[:69] + "..."
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", link, category, tc.CoverageStatus, summary)
	}
	b.WriteString("\n")
}

func flaggedJiraCards(cases []analyzer.InputTestCaseResult) []analyzer.InputTestCaseResult {
	var out []analyzer.InputTestCaseResult
	for _, tc := range cases {
		if analyzer.CardNeedsAttention(tc) {
			out = append(out, tc)
		}
	}
	return out
}

func writeJiraCardDetail(b *strings.Builder, index int, tc analyzer.InputTestCaseResult) {
	fmt.Fprintf(b, "#### %d. %s — %s\n\n", index, tc.Key, tc.Summary)
	if tc.URL != "" {
		fmt.Fprintf(b, "- **Jira:** [%s](%s)\n", tc.Key, tc.URL)
	}
	fmt.Fprintf(b, "- **Category:** %s\n", tc.PrimaryTestCategory)
	fmt.Fprintf(b, "- **Coverage:** %s (%d/100)\n", tc.CoverageStatus, tc.CoverageScore)
	if tc.PriorityNote != "" {
		fmt.Fprintf(b, "- **Priority note:** %s\n", tc.PriorityNote)
	}
	if tc.Recommendation != "" {
		fmt.Fprintf(b, "- **Recommendation:** %s\n", tc.Recommendation)
	}
	if len(tc.E2EScenarios) > 0 {
		b.WriteString("- **Suggested validations:**\n")
		for _, scenario := range tc.E2EScenarios {
			fmt.Fprintf(b, "  - %s\n", scenario)
		}
	}
	b.WriteString("\n")
}
