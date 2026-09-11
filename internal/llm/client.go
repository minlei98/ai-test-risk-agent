package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

type generateContentRequest struct {
	SystemInstruction *contentBlock   `json:"systemInstruction,omitempty"`
	Contents          []contentBlock  `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type generationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type contentBlock struct {
	Role  string       `json:"role,omitempty"`
	Parts []textPart   `json:"parts"`
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

func SynthesizeReport(cfg *config.Config, result *analyzer.Result, systemPrompt, userPrompt string) (string, error) {
	if !cfg.LLM.Enabled {
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

	evidence := compactEvidence(result)
	user := strings.TrimSpace(userPrompt) + "\n\n## Evidence\n\n" + evidence

	timeout := time.Duration(cfg.LLM.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
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
		GenerationConfig: generationConfig{MaxOutputTokens: 8192},
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

func compactEvidence(result *analyzer.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Overall risk: %d/100 (%s)\n", result.OverallRisk, result.RiskLevel)
	fmt.Fprintf(&b, "Quality: %.1f/10 | Release readiness: %s\n\n",
		result.Scorecard.OverallQuality, result.Scorecard.ReleaseReadiness)

	for _, repo := range result.Repos {
		fmt.Fprintf(&b, "### Repository: %s (%s)\n", repo.Name, repo.Role)
		fmt.Fprintf(&b, "- Files: %d, Lines: %d, Test files: %d, Spec files: %d, Skips: %d\n",
			repo.Files, repo.Lines, repo.TestFiles, repo.SpecFiles, repo.SkipCount)
		if len(repo.TestLevels) > 0 {
			b.WriteString("- Test level signals: ")
			for k, v := range repo.TestLevels {
				fmt.Fprintf(&b, "%s=%d ", k, v)
			}
			b.WriteByte('\n')
		}
		if len(repo.CrossCutting) > 0 {
			b.WriteString("- Cross-cutting signals: ")
			for k, v := range repo.CrossCutting {
				fmt.Fprintf(&b, "%s=%d ", k, v)
			}
			b.WriteByte('\n')
		}
		if len(repo.CriticalHits) > 0 {
			b.WriteString("- Critical term hits: ")
			for k, v := range repo.CriticalHits {
				fmt.Fprintf(&b, "%s=%d ", k, v)
			}
			b.WriteByte('\n')
		}
		for _, f := range repo.Findings {
			fmt.Fprintf(&b, "- [%s %s %d] %s | %s\n", f.Severity, f.Category, f.Score, f.Evidence, f.Recommendation)
		}
		b.WriteByte('\n')
	}

	if len(result.TopRisks) > 0 {
		b.WriteString("### Top risks\n")
		for _, g := range result.TopRisks {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		b.WriteByte('\n')
	}
	if len(result.CrossRepoGaps) > 0 {
		b.WriteString("### Cross-repository gaps\n")
		for _, g := range result.CrossRepoGaps {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		b.WriteByte('\n')
	}
	if len(result.KeyIssues) > 0 {
		b.WriteString("### Key issues\n")
		for _, issue := range result.KeyIssues {
			fmt.Fprintf(&b, "- %s: %s\n", issue.Issue, issue.Impact)
		}
		b.WriteByte('\n')
	}
	if len(result.InputTestCases) > 0 {
		b.WriteString("### Input test cases (Jira)\n")
		b.WriteString("Analyze Jira cards separately from repository inventory. Use requirement analysis for what the card asks to validate, and repository traceability only for whether repos contain matching tests.\n")
		for _, tc := range result.InputTestCases {
			fmt.Fprintf(&b, "- %s (%s): requirement_coverage=%s score=%d\n",
				tc.Key, tc.Summary, tc.CoverageStatus, tc.CoverageScore)
			if tc.Description != "" {
				fmt.Fprintf(&b, "  description: %s\n", tc.Description)
			}
			if tc.AcceptanceCriteria != "" {
				fmt.Fprintf(&b, "  acceptance_criteria: %s\n", tc.AcceptanceCriteria)
			}
			if tc.RequirementAnalysis != "" {
				fmt.Fprintf(&b, "  requirement_analysis: %s\n", tc.RequirementAnalysis)
			}
			if tc.RepoTraceability != "" {
				fmt.Fprintf(&b, "  repo_traceability: %s\n", tc.RepoTraceability)
			}
			for _, item := range tc.RequirementItems {
				fmt.Fprintf(&b, "  requirement: [%s] %s | repo_overlap=%s\n",
					item.CoverageStatus, item.Text, item.RepoEvidence)
			}
			if len(tc.MissingScenarios) > 0 {
				fmt.Fprintf(&b, "  gaps: %s\n", strings.Join(tc.MissingScenarios, "; "))
			}
		}
		b.WriteByte('\n')
	}

	return b.String()
}
