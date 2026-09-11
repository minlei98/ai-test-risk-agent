package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minlei98/ai-test-risk-agent/internal/analyzer"
	"github.com/minlei98/ai-test-risk-agent/internal/config"
	"github.com/minlei98/ai-test-risk-agent/internal/llm"
	"github.com/minlei98/ai-test-risk-agent/internal/report"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Println("usage: test-risk-agent analyze --config CONFIG --output report.md [--json-output report.json]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/argya-gitops.yaml", "configuration file")
	output := fs.String("output", "report.md", "markdown report")
	jsonOutput := fs.String("json-output", "", "optional JSON report")
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

	var llmReport string
	if cfg.LLM.Enabled {
		systemPrompt, userPrompt, err := llm.LoadPrompts("prompts")
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
