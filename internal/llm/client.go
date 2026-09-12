package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

type generateContentRequest struct {
	SystemInstruction *contentBlock    `json:"systemInstruction,omitempty"`
	Contents          []contentBlock   `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type generationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type contentBlock struct {
	Role  string     `json:"role,omitempty"`
	Parts []textPart `json:"parts"`
}

type textPart struct {
	Text string `json:"text"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []textPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func LLMEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.LLM.Mode))
	if mode == "off" {
		return false
	}
	return cfg.LLM.Enabled
}

func SynthesizeReport(cfg *config.Config, result *analyzer.Result, systemPrompt, userPrompt string) (string, error) {
	if !LLMEnabled(cfg) {
		return "", nil
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required when llm.enabled is true")
	}
	if cfg.LLM.BaseURL == "" {
		return "", fmt.Errorf("llm.base_url is required when llm.enabled is true")
	}
	if cfg.LLM.Model == "" {
		return "", fmt.Errorf("llm.model is required when llm.enabled is true")
	}

	evidence := compactEvidence(cfg, result)
	user := strings.TrimSpace(userPrompt) + "\n\n## Evidence\n\n" + evidence

	timeout := time.Duration(cfg.LLM.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	maxOut := cfg.LLM.MaxOutputTokens
	if maxOut <= 0 {
		if strings.EqualFold(cfg.LLM.Mode, "full") {
			maxOut = 8192
		} else {
			maxOut = 2048
		}
	}

	body, err := json.Marshal(generateContentRequest{
		SystemInstruction: &contentBlock{
			Parts: []textPart{{Text: systemPrompt}},
		},
		Contents: []contentBlock{
			{
				Role:  "user",
				Parts: []textPart{{Text: user}},
			},
		},
		GenerationConfig: generationConfig{MaxOutputTokens: maxOut},
	})
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := fmt.Sprintf("%s/models/%s:generateContent",
		strings.TrimRight(cfg.LLM.BaseURL, "/"),
		cfg.LLM.Model,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out generateContentResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("llm error: %s", out.Error.Message)
	}

	var text strings.Builder
	for _, candidate := range out.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				text.WriteString(part.Text)
			}
		}
	}
	if text.Len() == 0 {
		return "", fmt.Errorf("llm returned no content")
	}
	return strings.TrimSpace(text.String()), nil
}

func compactEvidence(cfg *config.Config, result *analyzer.Result) string {
	maxFindings := 8
	maxChars := 24000
	executive := true
	if cfg != nil {
		if cfg.LLM.MaxFindingsPerRepo > 0 {
			maxFindings = cfg.LLM.MaxFindingsPerRepo
		}
		if cfg.LLM.MaxEvidenceChars > 0 {
			maxChars = cfg.LLM.MaxEvidenceChars
		}
		executive = !strings.EqualFold(cfg.LLM.Mode, "full")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Overall risk: %d/100 (%s)\n", result.OverallRisk, result.RiskLevel)
	if result.AnalysisMode != "" {
		fmt.Fprintf(&b, "Analysis mode: %s\n", result.AnalysisMode)
	}
	if len(result.DeprioritizedCategories) > 0 {
		fmt.Fprintf(&b, "Deprioritized test categories: %s\n", strings.Join(result.DeprioritizedCategories, ", "))
	}
	fmt.Fprintf(&b, "Quality: %.1f/10 | Release readiness: %s\n\n",
		result.Scorecard.OverallQuality, result.Scorecard.ReleaseReadiness)

	for _, repo := range result.Repos {
		fmt.Fprintf(&b, "### Repository: %s (%s)\n", repo.Name, repo.Role)
		fmt.Fprintf(&b, "- Files: %d, Lines: %d, Test files: %d, Spec files: %d, Skips: %d\n",
			repo.Files, repo.Lines, repo.TestFiles, repo.SpecFiles, repo.SkipCount)
		writeNonZeroMap(&b, "Test level signals", repo.TestLevels)
		writeNonZeroMap(&b, "Cross-cutting signals", repo.CrossCutting)
		writeTopMapEntries(&b, "Critical term hits", repo.CriticalHits, 8)
		for _, f := range topFindings(repo.Findings, maxFindings) {
			fmt.Fprintf(&b, "- [%s %s %d] %s | %s\n", f.Severity, f.Category, f.Score, f.Evidence, f.Recommendation)
		}
		if len(repo.Findings) > maxFindings {
			fmt.Fprintf(&b, "- ... %d more findings omitted\n", len(repo.Findings)-maxFindings)
		}
		b.WriteByte('\n')
	}

	writeBulletSection(&b, "### Top risks", result.TopRisks, 10)
	writeBulletSection(&b, "### Cross-repository gaps", result.CrossRepoGaps, 10)
	if !executive && len(result.KeyIssues) > 0 {
		b.WriteString("### Key issues\n")
		for _, issue := range result.KeyIssues {
			fmt.Fprintf(&b, "- %s: %s\n", issue.Issue, issue.Impact)
		}
		b.WriteByte('\n')
	}
	if len(result.InputTestCases) > 0 {
		b.WriteString("### Jira test scope (category-level)\n")
		if result.JiraSummary.OverallAnalysis != "" {
			fmt.Fprintf(&b, "%s\n", result.JiraSummary.OverallAnalysis)
		}
		for _, cat := range result.JiraSummary.Categories {
			fmt.Fprintf(&b, "- category=%s cards=%d risk=%s repo_support=%s keys=%s analysis=%s\n",
				cat.Category, cat.CardCount, cat.RiskLevel, cat.RepoSupport, strings.Join(cat.Keys, ","), cat.RiskAnalysis)
		}
		for _, risk := range result.JiraSummary.TopRisks {
			fmt.Fprintf(&b, "- category_risk: %s\n", risk)
		}
		for _, tc := range result.InputTestCases {
			if !analyzer.CardNeedsAttention(tc) {
				continue
			}
			fmt.Fprintf(&b, "- flagged %s (%s): category=%s coverage=%s score=%d recommendation=%s\n",
				tc.Key, tc.Summary, tc.PrimaryTestCategory, tc.CoverageStatus, tc.CoverageScore, tc.Recommendation)
		}
		b.WriteByte('\n')
	}

	out := b.String()
	if maxChars > 0 && len(out) > maxChars {
		out = out[:maxChars] + "\n\n[evidence truncated for token budget]\n"
	}
	return out
}

func writeBulletSection(b *strings.Builder, title string, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	b.WriteString(title)
	b.WriteByte('\n')
	for i, item := range items {
		if limit > 0 && i >= limit {
			fmt.Fprintf(b, "- ... %d more omitted\n", len(items)-limit)
			break
		}
		fmt.Fprintf(b, "- %s\n", item)
	}
	b.WriteByte('\n')
}

func writeNonZeroMap(b *strings.Builder, label string, m map[string]int) {
	if len(m) == 0 {
		return
	}
	var parts []string
	for k, v := range m {
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", k, v))
		}
	}
	if len(parts) == 0 {
		return
	}
	sort.Strings(parts)
	b.WriteString("- ")
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(strings.Join(parts, " "))
	b.WriteByte('\n')
}

func writeTopMapEntries(b *strings.Builder, label string, m map[string]int, limit int) {
	if len(m) == 0 {
		return
	}
	type kv struct {
		key string
		val int
	}
	var entries []kv
	for k, v := range m {
		if v > 0 {
			entries = append(entries, kv{k, v})
		}
	}
	if len(entries) == 0 {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].val > entries[j].val })
	b.WriteString("- ")
	b.WriteString(label)
	b.WriteString(": ")
	for i, e := range entries {
		if limit > 0 && i >= limit {
			break
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(b, "%s=%d", e.key, e.val)
	}
	if limit > 0 && len(entries) > limit {
		fmt.Fprintf(b, " ...+%d", len(entries)-limit)
	}
	b.WriteByte('\n')
}

func topFindings(findings []analyzer.Finding, limit int) []analyzer.Finding {
	if len(findings) == 0 || limit <= 0 {
		return nil
	}
	out := append([]analyzer.Finding(nil), findings...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if si != sj {
			return si > sj
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func severityRank(sev string) int {
	switch strings.ToUpper(strings.TrimSpace(sev)) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}
