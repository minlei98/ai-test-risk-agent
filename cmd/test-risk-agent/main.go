package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
	"github.com/minlei98/ai-test-risk-agent/internal/jira"
	"github.com/minlei98/ai-test-risk-agent/internal/llm"
	"github.com/minlei98/ai-test-risk-agent/internal/paths"
	"github.com/minlei98/ai-test-risk-agent/internal/report"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Println("usage: test-risk-agent analyze --config CONFIG --output report.md [--jira KEY] [--jira-file FILE] [--json-output report.json]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/argya-gitops.yaml", "configuration file")
	output := fs.String("output", "report.md", "markdown report")
	jsonOutput := fs.String("json-output", "", "optional JSON report")
	jiraKeys := fs.String("jira", "", "comma-separated Jira issue keys or browse URLs")
	jiraFile := fs.String("jira-file", "", "comma-separated JSON files with Jira issue fixtures")
	jiraNoChildren := fs.Bool("jira-no-children", false, "do not fetch child/subtask issues for parent cards")
	keep := fs.Bool("keep-workdir", false, "keep cloned repositories")
	noClone := fs.Bool("no-clone", false, "use repository paths as local paths when url is a local directory")
	_ = keep
	_ = noClone
	fs.Parse(os.Args[2:])

	cfg, err := config.Load(*cfgPath)
	if err != nil { fail(err) }

	roots, cleanup, err := analyzer.PrepareRepositories(cfg)
	if err != nil { fail(err) }
	defer cleanup()

	result, err := analyzer.Analyze(cfg, roots)
	if err != nil { fail(err) }

	jiraKeysList := mergeCSV(*jiraKeys, cfg.Jira.Keys)
	jiraFilesList := mergeCSV(*jiraFile, cfg.Jira.Files)
	if len(jiraKeysList) > 0 || len(jiraFilesList) > 0 {
		creds, err := jira.LoadCredentials()
		if err != nil { fail(err) }
		opts := jira.DefaultResolveOptions()
		opts.MaxIssues = cfg.Jira.MaxIssues
		opts.IncludeChildren = !*jiraNoChildren
		if cfg.Jira.IncludeChildren != nil {
			opts.IncludeChildren = *cfg.Jira.IncludeChildren
		}
		issues, err := jira.ResolveIssues(opts, cfg.Jira.BaseURL, creds, jiraKeysList, jiraFilesList)
		if err != nil { fail(err) }
		result.InputTestCases = analyzer.AnalyzeInputCases(issues, result, cfg)
	}

	var llmReport string
	if cfg.LLM.Enabled {
		promptsDir, err := paths.ResolveDir(cfg.SourcePath, "prompts")
		if err != nil { fail(err) }
		systemPrompt, userPrompt, err := llm.LoadPrompts(promptsDir)
		if err != nil { fail(err) }
		llmReport, err = llm.SynthesizeReport(cfg, result, systemPrompt, userPrompt)
		if err != nil { fail(err) }
	}

	md := report.Markdown(cfg, result, llmReport)
	if err := os.WriteFile(*output, []byte(md), 0644); err != nil { fail(err) }

	if *jsonOutput != "" {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil { fail(err) }
		if err := os.WriteFile(*jsonOutput, b, 0644); err != nil { fail(err) }
	}

	fmt.Printf("Report written to %s\n", filepath.Clean(*output))
	fmt.Printf("Overall risk: %d/100 (%s)\n", result.OverallRisk, result.RiskLevel)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func mergeCSV(flagValue string, configValues []string) []string {
	var out []string
	for _, part := range strings.Split(flagValue, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	out = append(out, configValues...)
	return out
}
