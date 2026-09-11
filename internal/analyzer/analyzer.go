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
)

type RepoResult struct {
	Name string `json:"name"`
	URL string `json:"url"`
	Role string `json:"role"`
	Files int `json:"files"`
	Lines int `json:"lines"`
	TestFiles int `json:"test_files"`
	Findings []Finding `json:"findings"`
	TestTypes map[string]int `json:"test_types"`
	CriticalHits map[string]int `json:"critical_hits"`
}

type Finding struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	Score int `json:"score"`
	Repo string `json:"repo"`
	Evidence string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

type Result struct {
	OverallRisk int `json:"overall_risk"`
	RiskLevel string `json:"risk_level"`
	Repos []RepoResult `json:"repos"`
	CrossRepoGaps []string `json:"cross_repo_gaps"`
	Recommendations []Finding `json:"recommendations"`
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
		if err != nil { return nil, func(){}, err }
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", r.Branch, r.URL, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, func(){}, fmt.Errorf("clone %s: %w\n%s", r.Name, err, bytes.TrimSpace(out))
		}
		roots[r.Name] = dir
		temps = append(temps, dir)
	}
	return roots, func() {
		for _, d := range temps { _ = os.RemoveAll(d) }
	}, nil
}

func Analyze(cfg *config.Config, roots map[string]string) (*Result, error) {
	res := &Result{}
	for _, r := range cfg.Repositories {
		rr, err := analyzeRepo(cfg, r, roots[r.Name])
		if err != nil { return nil, err }
		res.Repos = append(res.Repos, rr)
	}

	// Cross-repository heuristics: application repos should have tests, and
	// GitOps repos should contain deployment/sync evidence.
	for _, rr := range res.Repos {
		if rr.Role == "application" && rr.TestFiles == 0 {
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

	res.OverallRisk = overallRisk(res)
	res.RiskLevel = level(res.OverallRisk)
	sort.Slice(res.Recommendations, func(i,j int) bool { return res.Recommendations[i].Score > res.Recommendations[j].Score })
	return res, nil
}

func analyzeRepo(cfg *config.Config, r config.Repository, root string) (RepoResult, error) {
	rr := RepoResult{Name:r.Name, URL:r.URL, Role:r.Role, TestTypes:map[string]int{}, CriticalHits:map[string]int{}}
	reTest := regexp.MustCompile(`(_test\.go$|_test\.py$|\.spec\.(ts|js)$|\.test\.(ts|js)$|\.feature$)`)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() {
			for _, x := range cfg.Analysis.Ignore {
				if info.Name() == x { return filepath.SkipDir }
			}
			return nil
		}
		if rr.Files >= cfg.Analysis.MaxFiles { return nil }
		if info.Size() > int64(cfg.Analysis.MaxFileBytes) { return nil }
		rr.Files++
		f, e := os.Open(path); if e != nil { return nil }
		defer f.Close()
		sc := bufio.NewScanner(f)
		lineCount := 0
		var sample strings.Builder
		for sc.Scan() {
			lineCount++
			line := strings.ToLower(sc.Text())
			if sample.Len() < 50000 { sample.WriteString(line); sample.WriteByte('\n') }
			for _, term := range cfg.Analysis.CriticalTerms {
				if strings.Contains(line, strings.ToLower(term)) { rr.CriticalHits[strings.ToLower(term)]++ }
			}
			for typ, names := range map[string][]string{
				"negative":{"negative","invalid","forbidden","denied","failure","error"},
				"security":{"security","rbac","authorization","unauthorized"},
				"resilience":{"chaos","fault","recovery","restart","failover"},
				"performance":{"benchmark","load","stress","performance","latency"},
				"upgrade":{"upgrade","migration","rollback","compatibility"},
				"e2e":{"e2e","end-to-end"},
				"integration":{"integration","envtest"},
			} {
				for _, n := range names {
					if strings.Contains(line,n) { rr.TestTypes[typ]++; break }
				}
			}
		}
		rr.Lines += lineCount
		if reTest.MatchString(path) { rr.TestFiles++ }
		return nil
	})
	if err != nil { return rr, err }

	// Convert evidence into risk findings.
	add := func(cat string, score int, sev, ev, rec string) {
		rr.Findings = append(rr.Findings, Finding{Category:cat, Score:score, Severity:sev, Repo:r.Name, Evidence:ev, Recommendation:rec})
	}
	if rr.CriticalHits["iam"]+rr.CriticalHits["sts"]+rr.CriticalHits["oidc"] > 0 &&
		rr.TestTypes["security"] == 0 && rr.TestTypes["negative"] == 0 {
		add("Security / IAM", 90, "P0", "IAM/STS/OIDC-related implementation evidence exists but no explicit security/negative test evidence was detected.",
			"Add denied-permission, expired/invalid credential, and authorization boundary tests at integration/E2E level.")
	}
	if rr.CriticalHits["tenant"]+rr.CriticalHits["namespace"] > 0 && rr.TestTypes["negative"] == 0 {
		add("Multi-tenancy", 85, "P0", "Tenant/namespace concepts are present without detected negative isolation tests.",
			"Add cross-tenant access-denied tests, namespace isolation tests, and credential/RBAC boundary tests.")
	}
	if rr.CriticalHits["argocd"]+rr.CriticalHits["application"]+rr.CriticalHits["applicationset"] > 0 && rr.TestTypes["negative"] == 0 {
		add("GitOps failure handling", 80, "P1", "Argo CD/Application evidence exists but no explicit negative test evidence was detected.",
			"Test invalid manifests, failed sync, missing dependencies, drift, rollback, and recovery from partial sync.")
	}
	if rr.CriticalHits["cleanup"]+rr.CriticalHits["finalizer"] > 0 && rr.TestTypes["resilience"] == 0 {
		add("Cleanup / recovery", 78, "P1", "Cleanup/finalizer behavior is present without detected recovery testing.",
			"Add partial-failure cleanup, deletion timeout, stuck-finalizer, retry, and idempotent reconciliation tests.")
	}
	if rr.TestFiles > 0 && rr.TestTypes["e2e"] == 0 {
		add("System/E2E coverage", 65, "P1", "Test files were detected but no explicit E2E evidence was found.",
			"Add at least one end-to-end customer workflow covering deployment, failure, recovery, and verification.")
	}
	return rr, nil
}

func overallRisk(res *Result) int {
	max := 0
	for _, r := range res.Repos {
		for _, f := range r.Findings { if f.Score > max { max = f.Score } }
	}
	if max == 0 { max = 35 }
	return max
}
func level(n int) string {
	if n >= 70 { return "HIGH" }
	if n >= 40 { return "MEDIUM" }
	return "LOW"
}
func sanitize(s string) string { return strings.Map(func(r rune) rune { if (r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9') { return r }; return '-' }, s) }
