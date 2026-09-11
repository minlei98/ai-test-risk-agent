# AI Test Risk Agent

A Go CLI that analyzes one or more Git repositories and produces a software test
coverage, risk, gap, and recommendation report. It is designed for Kubernetes /
OpenShift / Argo CD / GitOps systems and supports cross-repository analysis.

## Target repositories

The default configuration is designed for:

- https://github.com/openshift-online/argya
- https://github.com/openshift-online/hp-gitops-manifests
- https://github.com/openshift-online/hp-gitops-tenants

If these repositories are private, the agent uses the user's existing Git
credentials (`git credential`, SSH agent, or `GH_TOKEN` for HTTPS).

## What it analyzes

- Unit/component/integration/API/E2E/system tests
- Negative, security, resilience, performance, upgrade, compatibility tests
- Kubernetes/OpenShift resources and CRDs
- Argo CD / GitOps manifests
- CI/Prow/GitHub Actions configuration
- IAM, RBAC, secrets, networking, storage, multi-tenancy and deployment risks
- Cross-repository implementation-to-test gaps
- Risk score from 0-100
- P0/P1/P2/P3 recommendations
- Optional LLM-assisted report synthesis

## Quick start

```bash
go mod tidy
go build -o bin/test-risk-agent ./cmd/test-risk-agent

./bin/test-risk-agent analyze \
  --config configs/argya-gitops.yaml \
  --output report.md
```

The analyzer works without an LLM. It performs deterministic repository scanning
and scoring. If `llm.enabled` is true, it sends the collected evidence to the
Google Gemini API for synthesis (same stack as argya/featurecheck).

### Optional LLM

```bash
export GEMINI_API_KEY=...
./bin/test-risk-agent analyze --config configs/argya-gitops.yaml --output report.md
```

Gemini API settings (aligned with argya/osde2e defaults):

```yaml
llm:
  enabled: true
  base_url: "https://generativelanguage.googleapis.com/v1beta"
  model: "gemini-3.1-pro-preview"
```

The tool never sends source files automatically. Only compact extracted evidence
is sent to the LLM.

## Example

```bash
./bin/test-risk-agent analyze \
  --config configs/argya-gitops.yaml \
  --output report.md
```

Useful options:

```text
--keep-workdir       Keep cloned repositories
--no-clone           Analyze existing local paths from config
--output report.md
--json-output report.json
```

## Report sections

1. Executive summary
2. Repository inventory
3. Test-type coverage
4. Risk areas
5. Missing negative/security/resilience coverage
6. Cross-repository gaps
7. Customer/system workflow coverage
8. CI coverage
9. Recommendations
10. Proposed tests

## Important limitation

This is an evidence-driven static analyzer, not a proof of complete test
coverage. AI recommendations should be reviewed by engineers before being
treated as requirements.
# ai-test-risk-agent
