package analyzer

import (
	"strings"
	"testing"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

func TestManifestRepoSkipsRuntimeSecurityIAM(t *testing.T) {
	cfg := &config.Config{}
	rr := RepoResult{
		Name:         "hp-gitops-manifests",
		Role:         "gitops-manifests",
		Files:        204,
		CriticalHits: map[string]int{"sts": 72, "oidc": 51, "secret": 412},
		CrossCutting: map[string]int{},
		TestLevels:   map[string]int{},
	}
	findings := repoFindings(cfg, rr)
	for _, f := range findings {
		if f.Category == "Security / IAM" {
			t.Fatalf("manifest repo should not get runtime Security/IAM finding: %+v", f)
		}
		if strings.Contains(f.Recommendation, "Attempt forbidden API calls") {
			t.Fatalf("manifest repo should not recommend runtime API tests: %+v", f)
		}
	}
	if !hasCategory(findings, "Manifest security policy") {
		t.Fatalf("expected manifest security policy finding, got: %v", categoryNames(findings))
	}
}

func TestTenantRepoSkipsRuntimeSecurityIAM(t *testing.T) {
	cfg := &config.Config{}
	rr := RepoResult{
		Name:         "hp-gitops-tenants",
		Role:         "gitops-tenants",
		Files:        66,
		CriticalHits: map[string]int{"sts": 10, "oidc": 8, "tenant": 477, "secret": 319},
		CrossCutting: map[string]int{},
		TestLevels:   map[string]int{},
	}
	findings := repoFindings(cfg, rr)
	for _, f := range findings {
		if f.Category == "Security / IAM" {
			t.Fatalf("tenant repo should not get runtime Security/IAM finding: %+v", f)
		}
	}
	if !hasCategory(findings, "Tenant boundary configuration") {
		t.Fatalf("expected tenant boundary finding, got: %v", categoryNames(findings))
	}
}

func TestApplicationRepoKeepsRuntimeSecurityIAM(t *testing.T) {
	cfg := &config.Config{}
	rr := RepoResult{
		Name:         "argya",
		Role:         "application",
		Files:        126,
		CriticalHits: map[string]int{"sts": 482, "oidc": 40, "iam": 10},
		CrossCutting: map[string]int{},
		TestLevels:   map[string]int{},
	}
	findings := repoFindings(cfg, rr)
	if !hasCategory(findings, "Security / IAM") {
		t.Fatalf("application repo should keep runtime Security/IAM finding, got: %v", categoryNames(findings))
	}
}

func hasCategory(findings []Finding, name string) bool {
	for _, f := range findings {
		if f.Category == name {
			return true
		}
	}
	return false
}

func categoryNames(findings []Finding) []string {
	var names []string
	for _, f := range findings {
		names = append(names, f.Category)
	}
	return names
}
