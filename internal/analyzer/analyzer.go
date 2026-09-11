package analyzer

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
	"github.com/minlei98/ai-test-risk-agent/internal/paths"
	"github.com/minlei98/ai-test-risk-agent/internal/rules"
)

type RepoResult struct {
	Name               string         `json:"name"`
	URL                string         `json:"url"`
	Role               string         `json:"role"`
	Files              int            `json:"files"`
	Lines              int            `json:"lines"`
	TestFiles          int            `json:"test_files"`
	SpecFiles          int            `json:"spec_files"`
	Findings           []Finding      `json:"findings"`
	TestLevels         map[string]int `json:"test_levels"`
	CrossCutting       map[string]int `json:"cross_cutting"`
	CriticalHits       map[string]int `json:"critical_hits"`
	RiskCategories     map[string]int `json:"risk_categories"`
	CIEvidence         []string       `json:"ci_evidence"`
	KeyAreas           []string       `json:"key_areas"`
	SkipCount          int            `json:"skip_count"`
	ShallowAssertCount int            `json:"shallow_assert_count"`
	HasGinkgo          bool           `json:"has_ginkgo"`
	HasJUnitOutput     bool           `json:"has_junit_output"`
	HasProw            bool           `json:"has_prow"`
	HasTekton          bool           `json:"has_tekton"`
	HasGitHubActions   bool           `json:"has_github_actions"`
	WorkflowSignals    map[string]int `json:"workflow_signals"`
}

type Finding struct {
	Category       string `json:"category"`
	Severity       string `json:"severity"`
	Score          int    `json:"score"`
	Repo           string `json:"repo"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
	MissingScenario string `json:"missing_scenario,omitempty"`
	TestLevel      string `json:"test_level,omitempty"`
	TestSteps      string `json:"test_steps,omitempty"`
}

type Result struct {
	OverallRisk      int              `json:"overall_risk"`
	RiskLevel        string           `json:"risk_level"`
	Repos            []RepoResult     `json:"repos"`
	CrossRepoGaps    []string         `json:"cross_repo_gaps"`
	Recommendations  []Finding        `json:"recommendations"`
	CoverageNotes    []string         `json:"coverage_notes"`
	Scorecard        Scorecard        `json:"scorecard"`
	TopRisks         []string         `json:"top_risks"`
	KeyIssues        []KeyIssue       `json:"key_issues"`
	Workflows        []WorkflowStatus `json:"workflows"`
	RiskCoverage     RiskCoverage     `json:"risk_coverage"`
	SecurityChecks   []SecurityCheck  `json:"security_checks"`
	ResilienceChecks []ResilienceCheck `json:"resilience_checks"`
	CILayers         []CILayer        `json:"ci_layers"`
	UncoveredRisks   []string              `json:"uncovered_risks"`
	AnalysisMode            string                `json:"analysis_mode"`
	DeprioritizedCategories []string              `json:"deprioritized_categories,omitempty"`
	InputTestCases          []InputTestCaseResult `json:"input_test_cases,omitempty"`
}

func PrepareRepositories(cfg *config.Config) (map[string]string, func(), error) {
	roots := map[string]string{}
	var temps []string
	for _, r := range cfg.Repositories {
		if r.Path != "" {
			roots[r.Name] = r.Path
			continue
		}
		dir, err := os.MkdirTemp("", "test-risk-"+sanitize(r.Name)+"-")
		if err != nil {
			return nil, func() {}, err
		}
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", r.Branch, r.URL, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, func() {}, fmt.Errorf("clone %s: %w\n%s", r.Name, err, bytes.TrimSpace(out))
		}
		roots[r.Name] = dir
		temps = append(temps, dir)
	}
	return roots, func() {
		for _, d := range temps {
			_ = os.RemoveAll(d)
		}
	}, nil
}

func Analyze(cfg *config.Config, roots map[string]string) (*Result, error) {
	rulesPath, err := paths.ResolveRulesFile(cfg.SourcePath, cfg.Analysis.RulesFile)
	if err != nil {
		return nil, err
	}
	ruleSet, err := rules.Load(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("load rules %s: %w", rulesPath, err)
	}

	res := &Result{
		DeprioritizedCategories: cfg.Risk.Deprioritize,
	}
	for _, r := range cfg.Repositories {
		rr, err := analyzeRepo(cfg, ruleSet, r, roots[r.Name])
		if err != nil {
			return nil, err
		}
		res.Repos = append(res.Repos, rr)
	}

	crossRepoGaps(cfg, res)
	res.CoverageNotes = coverageNotes(cfg, res)
	res.OverallRisk = overallRisk(res)
	res.RiskLevel = level(res.OverallRisk, cfg)

	for _, repo := range res.Repos {
		res.Recommendations = append(res.Recommendations, repo.Findings...)
	}
	sort.Slice(res.Recommendations, func(i, j int) bool {
		return res.Recommendations[i].Score > res.Recommendations[j].Score
	})
	buildAssessment(cfg, res)
	if res.AnalysisMode == "" {
		res.AnalysisMode = AnalysisModeRepository
	}

	return res, nil
}

func analyzeRepo(cfg *config.Config, ruleSet *rules.Rules, r config.Repository, root string) (RepoResult, error) {
	rr := RepoResult{
		Name:            r.Name,
		URL:             r.URL,
		Role:            r.Role,
		TestLevels:      map[string]int{},
		CrossCutting:    map[string]int{},
		CriticalHits:    map[string]int{},
		RiskCategories:  map[string]int{},
		WorkflowSignals: map[string]int{},
	}

	levelPatterns := ruleSet.TestTypePatterns(cfg.Analysis.TestLevels)
	crossPatterns := ruleSet.TestTypePatterns(cfg.Analysis.CrossCutting)
	categoryTerms := ruleSet.CategoryTerms()
	criticalTerms := cfg.Analysis.CriticalTerms
	if len(criticalTerms) == 0 {
		criticalTerms = defaultCriticalTerms(cfg.Project.Type)
	}

	reTest := regexp.MustCompile(`(_test\.go$|_test\.py$|\.spec\.(ts|js)$|\.test\.(ts|js)$|\.feature$|_suite_test\.go$)`)
	reGinkgo := regexp.MustCompile(`\b(describe|ginkgo\.describe|context)\s*\(`)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		if info.IsDir() {
			for _, x := range cfg.Analysis.Ignore {
				if info.Name() == x {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if rr.Files >= cfg.Analysis.MaxFiles {
			return nil
		}
		if info.Size() > int64(cfg.Analysis.MaxFileBytes) {
			return nil
		}

		rr.Files++
		noteCIEvidence(&rr, rel, path)

		f, e := os.Open(path)
		if e != nil {
			return nil
		}
		defer f.Close()

		isTestFile := reTest.MatchString(path)
		isSpecFile := strings.Contains(rel, "pkg/e2e/") && strings.HasSuffix(path, ".go") && !isTestFile
		if isTestFile {
			rr.TestFiles++
		}
		if isSpecFile {
			rr.SpecFiles++
		}

		sc := bufio.NewScanner(f)
		lineCount := 0
		hasGinkgo := false
		for sc.Scan() {
			lineCount++
			line := strings.ToLower(sc.Text())
			if isSpecFile && reGinkgo.MatchString(line) {
				hasGinkgo = true
			}

			for _, term := range criticalTerms {
				if strings.Contains(line, strings.ToLower(term)) {
					rr.CriticalHits[strings.ToLower(term)]++
				}
			}
			for cat, terms := range categoryTerms {
				for _, term := range terms {
					if strings.Contains(line, term) {
						rr.RiskCategories[cat]++
						break
					}
				}
			}

			if isTestFile || isSpecFile {
				noteTestSignals(&rr, line)
			}
			if isTestFile || isSpecFile {
				for typ, names := range levelPatterns {
					for _, n := range names {
						if strings.Contains(line, strings.ToLower(n)) {
							rr.TestLevels[typ]++
							break
						}
					}
				}
				for typ, names := range crossPatterns {
					for _, n := range names {
						if strings.Contains(line, strings.ToLower(n)) {
							rr.CrossCutting[typ]++
							break
						}
					}
				}
			}
		}
		if isSpecFile && hasGinkgo {
			rr.TestLevels["e2e"]++
		}
		rr.Lines += lineCount
		return nil
	})
	if err != nil {
		return rr, err
	}

	rr.KeyAreas = topTerms(rr.CriticalHits, 5)
	rr.Findings = repoFindings(cfg, rr)
	return rr, nil
}

func repoFindings(cfg *config.Config, rr RepoResult) []Finding {
	var out []Finding
	add := func(cat string, score int, sev, ev, missing, rec, level, steps string) {
		out = append(out, Finding{
			Category: cat, Score: score, Severity: sev, Repo: rr.Name,
			Evidence: ev, MissingScenario: missing, Recommendation: rec,
			TestLevel: level, TestSteps: steps,
		})
	}

	totalTests := rr.TestFiles + rr.SpecFiles
	switch cfg.Project.Type {
	case "managed-openshift":
		managedOpenShiftFindings(rr, totalTests, add)
	default:
		gitOpsFindings(rr, totalTests, add)
	}
	return out
}

func gitOpsFindings(rr RepoResult, totalTests int, add func(string, int, string, string, string, string, string, string)) {
	if rr.CriticalHits["iam"]+rr.CriticalHits["sts"]+rr.CriticalHits["oidc"] > 0 &&
		rr.CrossCutting["security"] == 0 && rr.CrossCutting["negative"] == 0 {
		add("Security / IAM", 90, "P0",
			"IAM/STS/OIDC-related implementation evidence exists but no explicit security/negative test evidence was detected in test files.",
			"Denied-permission, expired credential, and authorization boundary scenarios are not evidenced in tests.",
			"Add denied-permission, expired/invalid credential, and authorization boundary tests at integration/E2E level.",
			"Integration / E2E",
			"1. Provision identities with least privilege.\n2. Attempt forbidden API calls.\n3. Assert explicit 403/denied responses.")
	}
	if rr.CriticalHits["tenant"]+rr.CriticalHits["namespace"] > 0 && rr.CrossCutting["negative"] == 0 {
		add("Multi-tenancy", 85, "P0",
			"Tenant/namespace concepts are present without detected negative isolation tests.",
			"Cross-tenant access attempts are not evidenced in test files.",
			"Add cross-tenant access-denied tests, namespace isolation tests, and credential/RBAC boundary tests.",
			"E2E / System",
			"1. Deploy two tenants.\n2. Attempt cross-namespace API and network access.\n3. Assert isolation holds.")
	}
	if rr.CriticalHits["argocd"]+rr.CriticalHits["application"]+rr.CriticalHits["applicationset"] > 0 && rr.CrossCutting["negative"] == 0 {
		add("GitOps failure handling", 80, "P1",
			"Argo CD/Application evidence exists but no explicit negative test evidence was detected.",
			"Invalid manifests, failed sync, and rollback scenarios are not evidenced.",
			"Test invalid manifests, failed sync, missing dependencies, drift, rollback, and recovery from partial sync.",
			"Integration / E2E",
			"1. Introduce invalid manifest.\n2. Trigger sync.\n3. Assert failure handling and recovery path.")
	}
	if rr.CriticalHits["cleanup"]+rr.CriticalHits["finalizer"] > 0 && rr.CrossCutting["resilience"] == 0 {
		add("Cleanup / recovery", 78, "P1",
			"Cleanup/finalizer behavior is present without detected recovery testing.",
			"Partial-failure cleanup and stuck-finalizer scenarios are not evidenced.",
			"Add partial-failure cleanup, deletion timeout, stuck-finalizer, retry, and idempotent reconciliation tests.",
			"Integration / E2E",
			"1. Create resources with finalizers.\n2. Simulate partial deletion failure.\n3. Assert cleanup completes without leaks.")
	}
	if totalTests > 0 && rr.TestLevels["e2e"] == 0 && rr.TestLevels["system"] == 0 {
		add("System/E2E coverage", 65, "P1",
			"Test files were detected but no explicit E2E/system evidence was found.",
			"End-to-end customer workflows are not evidenced in test files.",
			"Add at least one end-to-end customer workflow covering deployment, failure, recovery, and verification.",
			"E2E / System",
			"1. Deploy full stack.\n2. Exercise primary workflow.\n3. Inject failure and verify recovery.")
	}
	if totalTests == 0 && rr.Files > 20 {
		add("Test existence", 88, "P0",
			fmt.Sprintf("%s contains %d files but no detected test or spec files.", rr.Name, rr.Files),
			"No automated validation is evidenced in this repository.",
			"Add CI validation and at least one executable test suite before treating changes as safe to promote.",
			"CI / Unit",
			"1. Add lint/schema validation in CI.\n2. Add one smoke test for the primary workflow.\n3. Gate merges on test success.")
	}
}

func managedOpenShiftFindings(rr RepoResult, totalTests int, add func(string, int, string, string, string, string, string, string)) {
	if rr.CriticalHits["upgrade"] > 0 && rr.CrossCutting["upgrade"] == 0 {
		add("Cluster upgrade coverage", 82, "P0",
			"Upgrade-related implementation evidence exists without matching upgrade test evidence in test/spec files.",
			"Managed cluster upgrade, rollback, and compatibility scenarios are not evidenced in executable tests.",
			"Add upgrade-path tests covering pre-checks, execution, health validation, and rollback for supported streams.",
			"E2E / System",
			"1. Provision supported cluster version.\n2. Trigger managed upgrade.\n3. Validate health and workloads.\n4. Exercise rollback if supported.")
	}
	if rr.CriticalHits["hypershift"]+rr.CriticalHits["hosted"] > 0 && rr.CrossCutting["negative"] < 3 {
		add("HyperShift / hosted control plane", 78, "P1",
			"HyperShift/hosted control plane signals exist with limited negative test evidence.",
			"Hosted control plane failure and isolation scenarios may be under-tested.",
			"Add HyperShift-specific negative and recovery tests for worker availability, routes, and infra constraints.",
			"E2E",
			"1. Deploy HyperShift cluster.\n2. Disrupt worker/control paths.\n3. Assert recovery and customer-visible behavior.")
	}
	if rr.CriticalHits["ocm"]+rr.CriticalHits["rosa"]+rr.CriticalHits["sts"] > 0 && rr.CrossCutting["security"] == 0 {
		add("OCM / ROSA security", 80, "P0",
			"OCM/ROSA/STS implementation evidence exists without explicit security test evidence in test files.",
			"Credential, STS, and OCM authorization failures are not evidenced in tests.",
			"Add security and negative tests for OCM auth, STS role assumption, and credential expiry.",
			"Integration / E2E",
			"1. Use invalid/expired credentials.\n2. Attempt privileged OCM operations.\n3. Assert denied responses.")
	}
	if rr.CriticalHits["azure"] > 0 && rr.CriticalHits["aws"] == 0 && rr.CriticalHits["gcp"] == 0 {
		add("Cloud provider parity", 70, "P2",
			"Azure signals detected without balanced AWS/GCP evidence in the same repository.",
			"Multi-cloud parity may be incomplete across providers.",
			"Review cloud-specific configs and add provider-parity tests for critical workflows.",
			"E2E",
			"1. Identify provider-specific config gaps.\n2. Add equivalent tests per supported cloud.\n3. Compare pass/fail signals across providers.")
	}
	if rr.CriticalHits["prometheus"]+rr.CriticalHits["alertmanager"]+rr.CriticalHits["must-gather"] > 0 && rr.CrossCutting["resilience"] == 0 {
		add("Observability and recovery", 72, "P1",
			"Observability/recovery tooling is referenced without resilience test evidence.",
			"Alerting, must-gather, and recovery workflows are not evidenced in tests.",
			"Add resilience tests that validate alert firing, must-gather collection, and post-incident recovery.",
			"E2E / System",
			"1. Trigger failure condition.\n2. Validate alert and must-gather behavior.\n3. Verify service recovery.")
	}
	if totalTests > 0 && rr.TestLevels["e2e"] == 0 && rr.SpecFiles == 0 {
		add("Executable E2E specs", 68, "P1",
			fmt.Sprintf("%s has %d test files but no in-repo Ginkgo E2E specs were detected.", rr.Name, rr.TestFiles),
			"Most coverage may be unit-level while managed OpenShift workflows need executable E2E specs.",
			"Add or expand pkg/e2e specs and wire them into default suite configs.",
			"E2E",
			"1. Add Ginkgo specs for primary managed OpenShift workflows.\n2. Register in default suite config.\n3. Run in CI periodically.")
	}
	if totalTests == 0 && rr.Role != "library" && rr.Files > 20 {
		add("Test existence", 88, "P0",
			fmt.Sprintf("%s contains %d files but no detected test or spec files.", rr.Name, rr.Files),
			"No automated validation is evidenced in this repository.",
			"Add CI validation and executable tests for managed OpenShift workflows.",
			"CI / E2E",
			"1. Add CI pipeline gates.\n2. Add smoke E2E suite.\n3. Block merges on failures.")
	}
	if rr.Role == "library" && totalTests < 3 && rr.Files > 10 {
		add("Shared library test gap", 75, "P1",
			fmt.Sprintf("%s is a shared library with only %d test files across %d source files.", rr.Name, totalTests, rr.Files),
			"Shared provisioning/client helpers may lack regression protection.",
			"Add unit and integration tests for OCM/OpenShift client helpers consumed by downstream E2E frameworks.",
			"Unit / Integration",
			"1. Identify exported client surfaces.\n2. Add unit tests with mocked providers.\n3. Add one integration test per critical client.")
	}
}

func crossRepoGaps(cfg *config.Config, res *Result) {
	names := map[string]RepoResult{}
	for _, rr := range res.Repos {
		names[rr.Name] = rr
	}

	for _, rr := range res.Repos {
		if rr.Role == "application" && rr.TestFiles+rr.SpecFiles == 0 {
			res.CrossRepoGaps = append(res.CrossRepoGaps,
				fmt.Sprintf("%s has no detected test files; runtime behavior may be covered in another repository.", rr.Name))
		}
		if rr.Role == "gitops-manifests" || rr.Role == "gitops-tenants" {
			if rr.CriticalHits["argocd"] == 0 && rr.CriticalHits["application"] == 0 {
				res.CrossRepoGaps = append(res.CrossRepoGaps,
					fmt.Sprintf("%s has no obvious Argo CD Application/ApplicationSet evidence.", rr.Name))
			}
		}
	}

	if cfg.Project.Type == "managed-openshift" {
		if main, ok := names["osde2e"]; ok {
			if lib, ok := names["osde2e-common"]; ok {
				if main.TestFiles+main.SpecFiles > 0 && lib.TestFiles < 5 {
					res.CrossRepoGaps = append(res.CrossRepoGaps,
						"osde2e has executable tests but osde2e-common has thin test coverage for shared client/provisioning helpers.")
				}
				if len(main.CIEvidence) > 0 && len(lib.CIEvidence) == 0 {
					res.CrossRepoGaps = append(res.CrossRepoGaps,
						"osde2e-common lacks obvious CI configuration while osde2e has pipeline definitions.")
				}
			}
		}
		if hasRepo(res, "osde2e") {
			main := names["osde2e"]
			if main.CriticalHits["addon"] > 0 && main.CrossCutting["regression"] == 0 {
				res.CrossRepoGaps = append(res.CrossRepoGaps,
					"Addon/operator workflows are referenced but regression test evidence is limited in detected test files.")
			}
		}
	}
}

func coverageNotes(cfg *config.Config, res *Result) []string {
	notes := []string{
		"Test-level and cross-cutting counts are derived from test/spec files only, not comments or documentation.",
		"Absence of a detected test is a gap signal, not proof that the test does not exist in another repository or external harness.",
	}
	if cfg.Project.Type == "managed-openshift" {
		notes = append(notes,
			"osde2e commonly delegates operator coverage to external ad-hoc harness images; in-repo pkg/e2e specs may under-represent true system coverage.",
			"Ginkgo specs under pkg/e2e/ are counted even when they do not use *_test.go naming.")
	}
	return notes
}

func noteCIEvidence(rr *RepoResult, rel, path string) {
	base := filepath.Base(rel)
	switch {
	case strings.HasPrefix(rel, ".github/workflows/"):
		rr.HasGitHubActions = true
		rr.CIEvidence = append(rr.CIEvidence, rel)
	case strings.HasPrefix(rel, ".tekton/"):
		rr.HasTekton = true
		rr.CIEvidence = append(rr.CIEvidence, rel)
	case base == ".gitlab-ci.yml":
		rr.CIEvidence = append(rr.CIEvidence, rel)
	case base == "Makefile" && !contains(rr.CIEvidence, "Makefile"):
		rr.CIEvidence = append(rr.CIEvidence, rel)
	}
	if strings.Contains(rel, "ci-operator") || strings.Contains(rel, "prow") || strings.Contains(rel, "openshift/release") {
		rr.HasProw = true
	}
	if strings.Contains(strings.ToLower(path), "ginkgo") {
		rr.HasGinkgo = true
	}
}

func noteTestSignals(rr *RepoResult, line string) {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "skip(") || strings.Contains(lower, "ginkgoskip") || strings.Contains(lower, "pending(") {
		rr.SkipCount++
	}
	if strings.Contains(lower, "benil()") || strings.Contains(lower, "beempty()") || strings.Contains(lower, "notto(beempty") ||
		strings.Contains(lower, "len(") && strings.Contains(lower, "> 0") {
		rr.ShallowAssertCount++
	}
	if strings.Contains(lower, "ginkgo") || strings.Contains(lower, "describe(") || strings.Contains(lower, "it(") {
		rr.HasGinkgo = true
	}
	if strings.Contains(lower, "junit") || strings.Contains(lower, "ginkgo.junitreport") {
		rr.HasJUnitOutput = true
	}
	for signal, terms := range workflowTerms() {
		for _, term := range terms {
			if strings.Contains(lower, term) {
				rr.WorkflowSignals[signal]++
				break
			}
		}
	}
}

func workflowTerms() map[string][]string {
	return map[string][]string{
		"install":     {"install", "create cluster", "provision"},
		"provision":   {"provision", "create cluster"},
		"onboarding":  {"onboarding", "onboard"},
		"oidc":        {"oidc", "openid"},
		"ingress":     {"ingress", "route"},
		"rbac":        {"rbac", "clusterrole", "rolebinding"},
		"upgrade":     {"upgrade", "migration"},
		"scaling":     {"scale", "autoscale", "resize"},
		"nodepool":    {"nodepool", "machinepool"},
		"maintenance": {"maintenance", "drain", "cordon"},
		"must-gather": {"must-gather", "mustgather"},
		"diagnostic":  {"diagnostic", "troubleshoot"},
		"backup":      {"backup"},
		"restore":     {"restore"},
		"disaster":    {"disaster", "dr "},
		"rollback":    {"rollback", "failed upgrade"},
		"chaos":       {"chaos", "fault", "disruption", "monkey"},
		"privatelink": {"privatelink", "private link"},
		"dns-failure": {"dns failure", "dns outage"},
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func hasRepo(res *Result, name string) bool {
	for _, r := range res.Repos {
		if r.Name == name {
			return true
		}
	}
	return false
}

func topTerms(hits map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range hits {
		if v > 0 {
			pairs = append(pairs, kv{k, v})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	var out []string
	for i, p := range pairs {
		if i >= n {
			break
		}
		out = append(out, fmt.Sprintf("%s (%d)", p.k, p.v))
	}
	return out
}

func defaultCriticalTerms(projectType string) []string {
	switch projectType {
	case "managed-openshift":
		return []string{
			"openshift", "cluster", "ocm", "rosa", "sre", "upgrade", "must-gather",
			"prometheus", "alertmanager", "oauth", "rbac", "secret", "route", "ingress",
			"operator", "hive", "addon", "managed", "hypershift", "hosted", "sts",
			"aws", "gcp", "azure", "fedramp", "fips", "proxy", "privatelink", "kms",
		}
	default:
		return []string{
			"iam", "sts", "oidc", "rbac", "secret", "tenant", "namespace",
			"argocd", "application", "applicationset", "sync", "upgrade",
			"delete", "cleanup", "finalizer", "networkpolicy", "ingress",
		}
	}
}

func overallRisk(res *Result) int {
	max := 0
	for _, r := range res.Repos {
		for _, f := range r.Findings {
			if f.Score > max {
				max = f.Score
			}
		}
	}
	if max == 0 {
		max = 35
	}
	return max
}

func level(n int, cfg *config.Config) string {
	high := int(cfg.Risk.HighThreshold)
	medium := int(cfg.Risk.MediumThreshold)
	if high == 0 {
		high = 70
	}
	if medium == 0 {
		medium = 40
	}
	if n >= high {
		return "HIGH"
	}
	if n >= medium {
		return "MEDIUM"
	}
	return "LOW"
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
}
