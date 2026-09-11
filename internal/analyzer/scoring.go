package analyzer

import (
	"fmt"
	"math"
	"strings"

	"github.com/minlei98/ai-test-risk-agent/internal/config"
)

type Scorecard struct {
	OverallQuality      float64            `json:"overall_quality"`
	ReleaseReadiness    string             `json:"release_readiness"`
	E2EEffectiveness    float64            `json:"e2e_effectiveness"`
	FrameworkQuality    float64            `json:"framework_quality"`
	CIIntegration       float64            `json:"ci_integration"`
	RiskCoverage        float64            `json:"risk_coverage"`
	CustomerWorkflow    float64            `json:"customer_workflow"`
	SecurityValidation  float64            `json:"security_validation"`
	ResilienceValidation float64           `json:"resilience_validation"`
	ReleaseConfidence   float64            `json:"release_confidence"`
	CategoryScores      map[string]float64 `json:"category_scores"`
}

type KeyIssue struct {
	Issue          string `json:"issue"`
	Impact         string `json:"impact"`
	Recommendation string `json:"recommendation"`
}

type WorkflowStatus struct {
	Workflow string `json:"workflow"`
	Status   string `json:"status"`
	Notes    string `json:"notes"`
}

type CoverageRating struct {
	SubArea  string `json:"sub_area"`
	Rating   string `json:"rating"`
	Evidence string `json:"evidence"`
}

type RiskCoverage struct {
	Functional   []CoverageRating `json:"functional"`
	Operational  []CoverageRating `json:"operational"`
	Reliability  []CoverageRating `json:"reliability"`
	Integration  []CoverageRating `json:"integration"`
	OverallFunc  string           `json:"overall_functional"`
	OverallOps   string           `json:"overall_operational"`
	OverallRel   string           `json:"overall_reliability"`
	OverallInt   string           `json:"overall_integration"`
}

type SecurityCheck struct {
	Area    string `json:"area"`
	Covered string `json:"covered"`
	Detail  string `json:"detail"`
}

type ResilienceCheck struct {
	Scenario  string `json:"scenario"`
	Validated string `json:"validated"`
}

type CILayer struct {
	Layer    string `json:"layer"`
	Purpose  string `json:"purpose"`
	Examples string `json:"examples"`
}

func buildAssessment(cfg *config.Config, res *Result) {
	primary := primaryRepo(res)
	totalTests := totalTestFiles(res)
	totalSkips := totalSkips(res)

	res.Scorecard = computeScorecard(cfg, res, primary, totalTests, totalSkips)
	res.TopRisks = topUncoveredRisks(res)
	res.KeyIssues = keyIssues(res, primary, totalTests, totalSkips)
	res.Workflows = customerWorkflows(primary)
	res.RiskCoverage = riskCoverage(primary, res)
	res.SecurityChecks = securityChecks(primary)
	res.ResilienceChecks = resilienceChecks(primary)
	res.CILayers = ciLayers(res)
	res.UncoveredRisks = uncoveredRisks(primary, res)
}

func primaryRepo(res *Result) RepoResult {
	for _, r := range res.Repos {
		if r.Role == "application" || r.Name == "osde2e" {
			return r
		}
	}
	if len(res.Repos) > 0 {
		return res.Repos[0]
	}
	return RepoResult{}
}

func totalTestFiles(res *Result) int {
	n := 0
	for _, r := range res.Repos {
		n += r.TestFiles + r.SpecFiles
	}
	return n
}

func totalSkips(res *Result) int {
	n := 0
	for _, r := range res.Repos {
		n += r.SkipCount
	}
	return n
}

func computeScorecard(cfg *config.Config, res *Result, primary RepoResult, totalTests, totalSkips int) Scorecard {
	e2e := scoreE2E(primary, totalTests, totalSkips)
	framework := scoreFramework(primary)
	ci := scoreCI(res)
	security := scoreSecurity(primary)
	resilience := scoreResilience(primary)
	risk := scoreRiskCoverage(res, primary)
	workflow := scoreWorkflows(primary)
	release := scoreReleaseConfidence(res.OverallRisk, e2e, security, resilience)

	overall := round1((e2e*0.2 + framework*0.1 + ci*0.15 + risk*0.15 + workflow*0.15 + security*0.1 + resilience*0.1 + release*0.05))

	return Scorecard{
		OverallQuality:       overall,
		ReleaseReadiness:     releaseReadiness(res.OverallRisk, overall),
		E2EEffectiveness:     e2e,
		FrameworkQuality:     framework,
		CIIntegration:        ci,
		RiskCoverage:         risk,
		CustomerWorkflow:     workflow,
		SecurityValidation:   security,
		ResilienceValidation: resilience,
		ReleaseConfidence:    release,
		CategoryScores: map[string]float64{
			"E2E Test Effectiveness":     e2e,
			"E2E Automation Framework":   framework,
			"CI Pipeline":              ci,
			"Risk Coverage":              risk,
			"Customer Workflow Validation": workflow,
			"Security Validation (E2E)":  security,
			"Resilience Validation (E2E)": resilience,
			"Release Confidence":         release,
		},
	}
}

func scoreE2E(rr RepoResult, totalTests, totalSkips int) float64 {
	if totalTests == 0 {
		return 2.0
	}
	e2eSignals := rr.TestLevels["e2e"] + rr.TestLevels["functional"] + rr.TestLevels["system"]
	base := clamp10(4.0 + float64(e2eSignals)/float64(max(totalTests, 1))*4.0)
	if rr.ShallowAssertCount > e2eSignals {
		base -= 1.5
	}
	if totalSkips > 0 {
		skipRate := float64(totalSkips) / float64(max(totalTests, 1))
		base -= clamp10(skipRate * 3.0)
	}
	if rr.CrossCutting["negative"] < 5 {
		base -= 0.5
	}
	return round1(clamp10(base))
}

func scoreFramework(rr RepoResult) float64 {
	score := 4.0
	if rr.HasGinkgo {
		score += 2.5
	}
	if rr.HasJUnitOutput {
		score += 1.0
	}
	if rr.TestLevels["integration"] > 0 {
		score += 0.5
	}
	if rr.SpecFiles > 10 {
		score += 1.0
	}
	return round1(clamp10(score))
}

func scoreCI(res *Result) float64 {
	score := 3.0
	for _, r := range res.Repos {
		if len(r.CIEvidence) > 0 {
			score += 1.5
		}
		if r.HasProw {
			score += 2.0
		}
		if r.HasTekton {
			score += 1.0
		}
		if r.HasGitHubActions {
			score += 0.5
		}
	}
	return round1(clamp10(score))
}

func scoreSecurity(rr RepoResult) float64 {
	score := 2.0
	sec := rr.CrossCutting["security"]
	neg := rr.CrossCutting["negative"]
	if sec > 0 {
		score += clamp10(float64(sec) / 20.0)
	}
	if neg > 10 {
		score += 2.0
	} else if neg > 0 {
		score += 1.0
	}
	if rr.WorkflowSignals["rbac"] > 0 && neg < 3 {
		score += 0.5
	}
	if rr.WorkflowSignals["privatelink"] > 0 && sec < 3 {
		score -= 0.5
	}
	return round1(clamp10(score))
}

func scoreResilience(rr RepoResult) float64 {
	score := 1.5
	res := rr.CrossCutting["resilience"]
	if res > 0 {
		score += clamp10(float64(res) / 30.0)
	}
	if rr.WorkflowSignals["rollback"] > 0 {
		score += 1.5
	}
	if rr.WorkflowSignals["chaos"] > 0 {
		score += 2.5
	}
	if rr.CrossCutting["upgrade"] > 0 {
		score += 1.0
	}
	return round1(clamp10(score))
}

func scoreRiskCoverage(res *Result, primary RepoResult) float64 {
	score := 5.0
	if len(res.Recommendations) > 0 {
		score -= float64(len(res.Recommendations)) * 0.4
	}
	if primary.CrossCutting["upgrade"] > 0 {
		score += 1.0
	}
	if primary.TestLevels["e2e"]+primary.TestLevels["functional"] > 50 {
		score += 1.0
	}
	return round1(clamp10(score))
}

func scoreWorkflows(rr RepoResult) float64 {
	wf := customerWorkflows(rr)
	covered := 0.0
	for _, w := range wf {
		switch w.Status {
		case "Covered":
			covered += 1.0
		case "Partially Covered":
			covered += 0.5
		}
	}
	if len(wf) == 0 {
		return 5.0
	}
	return round1(clamp10(covered / float64(len(wf)) * 10.0))
}

func scoreReleaseConfidence(overallRisk int, e2e, security, resilience float64) float64 {
	base := 7.0 - float64(overallRisk)/20.0
	base += (e2e + security + resilience - 15.0) / 10.0
	return round1(clamp10(base))
}

func releaseReadiness(risk int, quality float64) string {
	if risk >= 85 || quality < 4.0 {
		return "Not Ready"
	}
	if risk >= 60 || quality < 7.0 {
		return "Ready with Risks"
	}
	return "Ready"
}

func topUncoveredRisks(res *Result) []string {
	var risks []string
	for _, f := range res.Recommendations {
		if len(risks) >= 5 {
			break
		}
		risks = append(risks, fmt.Sprintf("%s — %s", f.Category, f.Evidence))
	}
	for _, g := range res.CrossRepoGaps {
		if len(risks) >= 5 {
			break
		}
		risks = append(risks, g)
	}
	return risks
}

func keyIssues(res *Result, primary RepoResult, totalTests, totalSkips int) []KeyIssue {
	var issues []KeyIssue
	if primary.ShallowAssertCount > primary.TestLevels["e2e"] {
		issues = append(issues, KeyIssue{
			Issue:          "Shallow feature assertions",
			Impact:         "False confidence on customer features",
			Recommendation: "Add behavioral checks (enforcement, delivery, isolation) instead of presence-only list assertions",
		})
	}
	if totalSkips > totalTests/4 && totalSkips > 0 {
		issues = append(issues, KeyIssue{
			Issue:          "Conditional skips",
			Impact:         "Pass rate overstates real coverage",
			Recommendation: "Track and gate on skip rate for critical tests",
		})
	}
	if primary.CrossCutting["negative"] < 5 {
		issues = append(issues, KeyIssue{
			Issue:          "Happy-path only",
			Impact:         "Regressions in error handling undetected",
			Recommendation: "Add negative E2E cases for authz, invalid input, and failure paths",
		})
	}
	if primary.CrossCutting["security"] < 5 && primary.CriticalHits["rbac"]+primary.CriticalHits["sts"]+primary.CriticalHits["oauth"] > 0 {
		issues = append(issues, KeyIssue{
			Issue:          "Minimal security validation",
			Impact:         "Security-sensitive features lack negative authz tests",
			Recommendation: "Add denied-permission, credential expiry, and isolation tests",
		})
	}
	if primary.CrossCutting["resilience"] < 5 && primary.WorkflowSignals["chaos"] == 0 {
		issues = append(issues, KeyIssue{
			Issue:          "No resilience/chaos validation",
			Impact:         "Failure recovery and rollback scenarios untested",
			Recommendation: "Add fault injection, rollback, and recovery scenarios",
		})
	}
	if len(res.Repos) > 1 && len(res.CrossRepoGaps) > 0 {
		issues = append(issues, KeyIssue{
			Issue:          "Coverage split across repositories/CI layers",
			Impact:         "No single job represents the full customer journey",
			Recommendation: "Define a unified release signal combining primary E2E, API contract, and conformance layers",
		})
	}
	return issues
}

func customerWorkflows(rr RepoResult) []WorkflowStatus {
	rate := func(signals ...int) string {
		sum := 0
		for _, s := range signals {
			sum += s
		}
		switch {
		case sum >= 10:
			return "Covered"
		case sum > 0:
			return "Partially Covered"
		default:
			return "Not Covered"
		}
	}

	return []WorkflowStatus{
		{"Day-0: Installation", rate(rr.WorkflowSignals["install"], rr.WorkflowSignals["provision"]), workflowNote(rr, "install", "provision")},
		{"Day-0: Onboarding", rate(rr.WorkflowSignals["onboarding"], rr.WorkflowSignals["oidc"]), workflowNote(rr, "onboarding", "oidc")},
		{"Day-1: Initial configuration", rate(rr.WorkflowSignals["ingress"], rr.WorkflowSignals["rbac"]), workflowNote(rr, "ingress", "rbac")},
		{"Day-2: Upgrades", rate(rr.CrossCutting["upgrade"], rr.WorkflowSignals["upgrade"]), workflowNote(rr, "upgrade")},
		{"Day-2: Scaling", rate(rr.WorkflowSignals["scaling"], rr.WorkflowSignals["nodepool"]), workflowNote(rr, "scaling", "nodepool")},
		{"Day-2: Maintenance", rate(rr.WorkflowSignals["maintenance"]), workflowNote(rr, "maintenance")},
		{"Day-2: Troubleshooting", rate(rr.WorkflowSignals["must-gather"], rr.WorkflowSignals["diagnostic"]), workflowNote(rr, "must-gather", "diagnostic")},
		{"Day-2: Disaster recovery", rate(rr.WorkflowSignals["backup"], rr.WorkflowSignals["restore"], rr.WorkflowSignals["disaster"]), workflowNote(rr, "backup", "restore")},
	}
}

func workflowNote(rr RepoResult, keys ...string) string {
	var found []string
	for _, k := range keys {
		if rr.WorkflowSignals[k] > 0 {
			found = append(found, k)
		}
	}
	if len(found) == 0 {
		return "No workflow signals detected in test/spec files"
	}
	return fmt.Sprintf("Signals detected: %s", strings.Join(found, ", "))
}

func riskCoverage(primary RepoResult, res *Result) RiskCoverage {
	rc := RiskCoverage{
		Functional: []CoverageRating{
			{"Feature validation", rateFeature(primary), featureEvidence(primary)},
			{"Configuration validation", rateConfig(primary), configEvidence(primary)},
			{"Upgrade validation", rateUpgrade(primary), upgradeEvidence(primary)},
		},
		Operational: []CoverageRating{
			{"Day-2 operations", rateOps(primary), opsEvidence(primary)},
			{"Scaling", rateScaling(primary), scalingEvidence(primary)},
			{"Cluster maintenance", rateMaintenance(primary), maintenanceEvidence(primary)},
			{"Recovery operations", rateRecovery(primary), recoveryEvidence(primary)},
		},
		Reliability: []CoverageRating{
			{"Long-running behavior", rateLongRunning(primary), longRunningEvidence(primary)},
			{"Service degradation", rateDegradation(primary), degradationEvidence(primary)},
			{"Failure recovery", rateFailureRecovery(primary), failureRecoveryEvidence(primary)},
		},
		Integration: []CoverageRating{
			{"Cross-operator interactions", rateCrossOperator(primary), crossOperatorEvidence(primary)},
			{"External services", rateExternal(primary), externalEvidence(primary)},
			{"Cloud provider dependencies", rateCloud(primary), cloudEvidence(primary)},
		},
	}
	rc.OverallFunc = overallRating(rc.Functional)
	rc.OverallOps = overallRating(rc.Operational)
	rc.OverallRel = overallRating(rc.Reliability)
	rc.OverallInt = overallRating(rc.Integration)
	return rc
}

func securityChecks(rr RepoResult) []SecurityCheck {
	yn := func(signal int, threshold int) string {
		switch {
		case signal >= threshold:
			return "Partial"
		case signal > 0:
			return "Minimal"
		default:
			return "No"
		}
	}
	return []SecurityCheck{
		{"Authentication", yn(rr.WorkflowSignals["oidc"]+rr.CrossCutting["security"], 5), authDetail(rr)},
		{"Authorization", yn(rr.CrossCutting["negative"]+rr.WorkflowSignals["rbac"], 8), authzDetail(rr)},
		{"RBAC", yn(rr.WorkflowSignals["rbac"]+rr.CrossCutting["security"], 5), rbacDetail(rr)},
		{"Secret handling", yn(rr.CriticalHits["secret"], 3), "Secret references detected; leakage validation not evidenced"},
		{"Token handling", yn(rr.CriticalHits["sts"]+rr.CriticalHits["ocm"], 5), "OCM/STS token usage referenced; expiry/rotation tests not evidenced"},
		{"Privilege escalation", yn(rr.CrossCutting["negative"], 10), "Negative authz scenarios limited in detected tests"},
	}
}

func resilienceChecks(rr RepoResult) []ResilienceCheck {
	yesNo := func(signal int) string {
		if signal > 0 {
			return "Yes"
		}
		return "No"
	}
	return []ResilienceCheck{
		{"Pod failures", yesNo(rr.WorkflowSignals["chaos"] + rr.CrossCutting["resilience"])},
		{"Node failures", yesNo(rr.WorkflowSignals["chaos"])},
		{"Network interruptions", yesNo(rr.WorkflowSignals["chaos"])},
		{"DNS failures", yesNo(rr.WorkflowSignals["dns-failure"])},
		{"Cloud API failures", yesNo(rr.WorkflowSignals["chaos"])},
		{"Cluster upgrades", yesNo(rr.CrossCutting["upgrade"])},
		{"Rollbacks", yesNo(rr.WorkflowSignals["rollback"])},
		{"Service restarts", yesNo(rr.CrossCutting["resilience"])},
	}
}

func ciLayers(res *Result) []CILayer {
	layers := []CILayer{}
	for _, r := range res.Repos {
		if r.HasGitHubActions || r.HasTekton {
			layers = append(layers, CILayer{
				Layer:    "Pre-submit (PR)",
				Purpose:  "Code quality gates",
				Examples: strings.Join(filterCI(r.CIEvidence, ".github", ".tekton"), ", "),
			})
		}
		if r.HasProw || r.HasTekton {
			layers = append(layers, CILayer{
				Layer:    "Periodic E2E",
				Purpose:  "Managed-service validation",
				Examples: strings.Join(filterCI(r.CIEvidence, "e2e", "periodic", "main"), ", "),
			})
		}
		if r.CrossCutting["upgrade"] > 0 {
			layers = append(layers, CILayer{
				Layer:    "Periodic upgrades",
				Purpose:  "Cross-version upgrade paths",
				Examples: "Upgrade-related test/spec evidence detected",
			})
		}
	}
	return layers
}

func uncoveredRisks(primary RepoResult, res *Result) []string {
	var out []string
	if primary.WorkflowSignals["rollback"] == 0 {
		out = append(out, "Failed upgrade rollback and stuck-cluster recovery")
	}
	if primary.WorkflowSignals["scaling"]+primary.WorkflowSignals["nodepool"] < 3 {
		out = append(out, "NodePool / MachinePool scaling and autoscaling")
	}
	if primary.WorkflowSignals["chaos"] == 0 {
		out = append(out, "Fault injection and failure recovery scenarios")
	}
	if primary.CrossCutting["security"] < 10 {
		out = append(out, "Negative authorization and isolation tests for security-sensitive features")
	}
	if primary.WorkflowSignals["privatelink"] > 0 && primary.CrossCutting["security"] < 5 {
		out = append(out, "PrivateLink end-to-end connectivity and public endpoint isolation")
	}
	for _, g := range res.CrossRepoGaps {
		out = append(out, g)
	}
	return out
}

func filterCI(evidence []string, keys ...string) []string {
	var out []string
	for _, e := range evidence {
		lower := strings.ToLower(e)
		for _, k := range keys {
			if strings.Contains(lower, k) {
				out = append(out, e)
				break
			}
		}
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func overallRating(items []CoverageRating) string {
	if len(items) == 0 {
		return "Partial"
	}
	score := 0.0
	for _, it := range items {
		switch it.Rating {
		case "Good":
			score += 1.0
		case "Partial":
			score += 0.5
		}
	}
	avg := score / float64(len(items))
	switch {
	case avg >= 0.75:
		return "Good"
	case avg >= 0.4:
		return "Partial"
	default:
		return "Missing"
	}
}

func rateFeature(rr RepoResult) string {
	if rr.TestLevels["functional"]+rr.TestLevels["e2e"] > 30 {
		return "Partial"
	}
	if rr.TestLevels["e2e"] > 0 {
		return "Partial"
	}
	return "Missing"
}

func featureEvidence(rr RepoResult) string {
	return fmt.Sprintf("E2E/functional signals=%d; shallow assertions=%d", rr.TestLevels["e2e"]+rr.TestLevels["functional"], rr.ShallowAssertCount)
}

func rateConfig(rr RepoResult) string {
	if rr.CriticalHits["privatelink"]+rr.CriticalHits["kms"] > 0 {
		return "Partial"
	}
	return "Partial"
}

func configEvidence(rr RepoResult) string {
	return fmt.Sprintf("Provider/config terms detected; privatelink=%d kms=%d", rr.CriticalHits["privatelink"], rr.CriticalHits["kms"])
}

func rateUpgrade(rr RepoResult) string {
	if rr.CrossCutting["upgrade"] > 20 {
		return "Good"
	}
	if rr.CrossCutting["upgrade"] > 0 {
		return "Partial"
	}
	return "Missing"
}

func upgradeEvidence(rr RepoResult) string {
	return fmt.Sprintf("Upgrade cross-cutting signals=%d", rr.CrossCutting["upgrade"])
}

func rateOps(rr RepoResult) string {
	if rr.TestLevels["e2e"]+rr.SpecFiles > 20 {
		return "Good"
	}
	return "Partial"
}

func opsEvidence(rr RepoResult) string {
	return fmt.Sprintf("Periodic/E2E spec evidence: e2e=%d specs=%d", rr.TestLevels["e2e"], rr.SpecFiles)
}

func rateScaling(rr RepoResult) string {
	if rr.WorkflowSignals["scaling"]+rr.WorkflowSignals["nodepool"] > 3 {
		return "Partial"
	}
	return "Missing"
}

func scalingEvidence(rr RepoResult) string {
	return fmt.Sprintf("Scaling/nodepool signals=%d", rr.WorkflowSignals["scaling"]+rr.WorkflowSignals["nodepool"])
}

func rateMaintenance(rr RepoResult) string {
	if rr.WorkflowSignals["maintenance"] > 0 {
		return "Partial"
	}
	return "Partial"
}

func maintenanceEvidence(rr RepoResult) string {
	return fmt.Sprintf("Maintenance signals=%d", rr.WorkflowSignals["maintenance"])
}

func rateRecovery(rr RepoResult) string {
	if rr.CrossCutting["resilience"]+rr.WorkflowSignals["rollback"] > 5 {
		return "Partial"
	}
	return "Missing"
}

func recoveryEvidence(rr RepoResult) string {
	return fmt.Sprintf("Resilience=%d rollback=%d", rr.CrossCutting["resilience"], rr.WorkflowSignals["rollback"])
}

func rateLongRunning(rr RepoResult) string {
	if rr.CrossCutting["performance"] > 0 {
		return "Partial"
	}
	return "Missing"
}

func longRunningEvidence(rr RepoResult) string {
	return fmt.Sprintf("Performance/long-running signals=%d", rr.CrossCutting["performance"])
}

func rateDegradation(rr RepoResult) string {
	if rr.RiskCategories["reliability"] > 50 {
		return "Partial"
	}
	return "Missing"
}

func degradationEvidence(rr RepoResult) string {
	return fmt.Sprintf("Reliability category hits=%d", rr.RiskCategories["reliability"])
}

func rateFailureRecovery(rr RepoResult) string {
	if rr.CrossCutting["resilience"]+rr.WorkflowSignals["rollback"] > 0 {
		return "Partial"
	}
	return "Missing"
}

func failureRecoveryEvidence(rr RepoResult) string {
	return fmt.Sprintf("Resilience=%d rollback=%d chaos=%d", rr.CrossCutting["resilience"], rr.WorkflowSignals["rollback"], rr.WorkflowSignals["chaos"])
}

func rateCrossOperator(rr RepoResult) string {
	if rr.RiskCategories["reliability"] > 100 {
		return "Partial"
	}
	return "Partial"
}

func crossOperatorEvidence(rr RepoResult) string {
	return fmt.Sprintf("Operator/reliability signals=%d", rr.RiskCategories["reliability"])
}

func rateExternal(rr RepoResult) string {
	if rr.CriticalHits["ocm"]+rr.CriticalHits["prometheus"] > 100 {
		return "Good"
	}
	return "Partial"
}

func externalEvidence(rr RepoResult) string {
	return fmt.Sprintf("OCM=%d prometheus=%d alertmanager=%d", rr.CriticalHits["ocm"], rr.CriticalHits["prometheus"], rr.CriticalHits["alertmanager"])
}

func rateCloud(rr RepoResult) string {
	providers := 0
	if rr.CriticalHits["aws"] > 0 {
		providers++
	}
	if rr.CriticalHits["gcp"] > 0 {
		providers++
	}
	if rr.CriticalHits["azure"] > 0 {
		providers++
	}
	switch {
	case providers >= 2:
		return "Partial"
	case providers == 1:
		return "Partial"
	default:
		return "Missing"
	}
}

func cloudEvidence(rr RepoResult) string {
	return fmt.Sprintf("aws=%d gcp=%d azure=%d", rr.CriticalHits["aws"], rr.CriticalHits["gcp"], rr.CriticalHits["azure"])
}

func authDetail(rr RepoResult) string {
	if rr.WorkflowSignals["oidc"] > 0 {
		return "OIDC signals in tests; full external auth flows may be pending"
	}
	return "External OIDC/auth flows not evidenced in executable tests"
}

func authzDetail(rr RepoResult) string {
	return fmt.Sprintf("Negative signals=%d RBAC signals=%d", rr.CrossCutting["negative"], rr.WorkflowSignals["rbac"])
}

func rbacDetail(rr RepoResult) string {
	if rr.WorkflowSignals["rbac"] > 0 && rr.CrossCutting["negative"] < 5 {
		return "RBAC presence detected; least-privilege negative cases limited"
	}
	return "RBAC references detected in implementation/tests"
}

func clamp10(v float64) float64 {
	return math.Min(10.0, math.Max(0.0, v))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
