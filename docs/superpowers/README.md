# Superpowers — Design Specs & Implementation Plans

This directory contains design specifications and step-by-step implementation plans
used during development of kube-llmops features. Plans include checkbox-based task
tracking and are intended for agentic (AI-assisted) execution.

## Completed

| Feature | Version | Spec | Plan |
|---------|---------|------|------|
| LLaMA-Factory Fine-tuning | v0.4.0 | [spec](specs/2026-03-30-llamafactory-finetune-design.md) | [plan](plans/2026-03-30-llamafactory-finetune.md) |
| Module Switches | v0.5.0 | [spec](specs/2026-04-05-module-switches-design.md) | [plan](plans/2026-04-05-module-switches.md) |
| Phase 5: Advanced Inference | v0.5.0 | [spec](specs/2026-04-05-phase5-advanced-inference-design.md) | [plan](plans/2026-04-05-phase5-advanced-inference.md) |
| Headlamp Dashboard | v0.5.0 | [spec](specs/2026-04-05-headlamp-dashboard-design.md) | — (replaced Phase 6 dashboard plan) |

## Planned (v1.0.0)

| Feature | Spec | Plan | Status |
|---------|------|------|--------|
| Kubernetes Operator (3 CRDs) | [spec](specs/2026-04-05-phase6-operator-design.md) | [plan](plans/2026-04-05-phase6-operator.md) | Not started |
| kubectl-llmops CLI | [spec](specs/2026-04-06-phase6-cli-design.md) | [plan](plans/2026-04-06-phase6-cli.md) | Not started |
| Web Dashboard (React + Go) | [spec](specs/2026-04-05-phase6-dashboard-design.md) | [plan](plans/2026-04-05-phase6-dashboard.md) | Not started |

## Conventions

- **Specs** (`specs/`): Design documents — architecture, data model, API contracts. Written before implementation.
- **Plans** (`plans/`): Task-by-task execution checklists with exact file paths and code snippets. Consumed by AI agents.
- Status markers: `STATUS: COMPLETED` in plan header = all tasks done and verified.
