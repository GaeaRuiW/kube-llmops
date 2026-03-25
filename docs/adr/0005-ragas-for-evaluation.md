# ADR-0005: Ragas for RAG Evaluation

## Status

Accepted

## Context

Measuring RAG quality with keyword-matching metrics (BLEU, ROUGE) fails to
capture semantic correctness, hallucination, and faithfulness to source
documents. Teams need automated, LLM-graded metrics that run in CI without
human annotation.

## Decision

We use the Ragas framework to compute faithfulness, answer relevancy, and
context precision scores. Evaluation scripts live in `tests/eval/` and can be
triggered as a post-deploy job or manual GitHub Action.

## Consequences

- Quantitative quality gates prevent regressions when prompts or models change.
- Ragas itself calls an LLM for scoring, adding inference cost per eval run.
- Baseline thresholds must be calibrated per use-case to avoid false failures.
