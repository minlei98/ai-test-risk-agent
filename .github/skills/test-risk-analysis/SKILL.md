---
name: test-risk-analysis
description: Analyze repository and Jira evidence for software test coverage, release risk, and concrete P0-P3 test recommendations. Use when reviewing test readiness, coverage gaps, GitOps or Kubernetes changes, or Jira test requirements.
---

# Test Risk Analysis

Use this skill to produce an evidence-driven test risk assessment for either:

- one or more repositories, or
- one or more Jira tickets.

Use both sources only when the user explicitly requests a hybrid review. Do not
silently expand a repository-only review into Jira analysis or a Jira-only review
into repository analysis.

## Operating rules

- Inspect the repository and relevant Jira evidence before making claims.
- Treat test presence as a signal, not proof of meaningful coverage.
- Distinguish unit, component, integration, API, functional, E2E, and system coverage.
- Check negative, security, resilience, upgrade, rollback, multi-tenant, and recovery scenarios.
- Prioritize customer impact, authorization, data integrity, deployment or sync failure, and cleanup behavior.
- Never invent components, ownership, requirements, or test results.
- State uncertainty and missing evidence explicitly.

## Select the review mode

Choose exactly one primary mode from the user's request:

### Repository mode

Use repository mode for source, test, manifest, CI, or cross-repository review.
Run the analyzer from the repository root:

From the repository root, build and run the deterministic analyzer first:

```bash
make build
./bin/test-risk-agent analyze \
  --config configs/argya-gitops.yaml \
  --output report.md \
  --json-output report.json \
  --no-llm
```

Use another configuration when the repository is not the default Argya/GitOps
target. Use `--no-clone` when repository paths in the configuration are local.
Add `--no-llm` whenever `GEMINI_API_KEY` is unavailable or deterministic output
is preferred.

### Jira mode

Use Jira mode when the user provides ticket keys, browse URLs, or fixture files
and asks for requirement or test-readiness analysis. Do not scan repositories in
this mode. Use the repository's no-repository configuration:

```bash
make build
./bin/test-risk-agent analyze \
  --config configs/jira-only.yaml \
  --jira KEY-123,KEY-456 \
  --output report.md \
  --json-output report.json \
  --no-clone \
  --no-llm
```

For offline work, replace `--jira` with `--jira-file testdata/jira/tenant-isolation.json`.
Use `--jira-no-children` when only explicitly supplied tickets should be
analyzed. If Jira credentials or a reachable Jira endpoint are unavailable,
request a fixture or report the limitation rather than inventing ticket data.

### Hybrid mode

Use hybrid mode only when the user asks to connect Jira requirements to
repository evidence. Add `--jira` or `--jira-file` to the repository-mode
command and explain which conclusions come from each source.

## Interpret the evidence

Read the generated report and JSON before responding. Summarize:

1. overall risk and release readiness,
2. highest-impact findings with repository evidence,
3. test levels and risk categories that are missing or weak,
4. cross-repository gaps in repository mode, Jira traceability gaps in Jira mode,
   or both in hybrid mode,
5. concrete proposed tests with priority P0-P3 and acceptance signals.

For every recommendation include the risk, evidence, missing scenario, proposed
test level, concrete test steps, and priority. Keep findings separate from
assumptions and avoid treating static manifest checks as runtime validation.

## Output contract

Return the report path and overall risk score, then a concise review organized
by severity. Preserve the generated Markdown and JSON artifacts unless the user
asks for a different output location. Recommend human review before treating
AI-generated recommendations as requirements.