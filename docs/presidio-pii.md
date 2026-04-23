# Presidio PII Detection & Anonymization

> **Note:** Presidio integration is part of the security module. Enable via `global.modules.security.enabled: true`.

## Overview

kube-llmops integrates [Microsoft Presidio](https://microsoft.github.io/presidio/) for detecting and anonymizing Personally Identifiable Information (PII) in LLM requests and responses. PII masking runs as a LiteLLM guardrail — no application code changes needed.

## Prerequisites

Enable the Presidio analyzer and anonymizer services:

```yaml
security:
  presidio:
    enabled: true
```

This deploys two containers:
- **Presidio Analyzer** (port 3000): NLP-based PII entity detection
- **Presidio Anonymizer** (port 3000): Replaces detected entities with placeholders

## Configuration

Enable the LiteLLM guardrail to route requests through Presidio:

```yaml
litellm:
  guardrails:
    presidio:
      enabled: true
      mode: "pre_call"        # pre_call | post_call | during_call | logging_only
      defaultOn: false         # true = scan every request; false = opt-in per request
      language: "en"
      entities:
        CREDIT_CARD: "MASK"    # Replace with <CREDIT_CARD>
        EMAIL_ADDRESS: "MASK"
        PHONE_NUMBER: "MASK"
        US_SSN: "MASK"
        IP_ADDRESS: "MASK"
      scoreThresholds:
        CREDIT_CARD: 0.8
        EMAIL_ADDRESS: 0.6
```

### Modes

| Mode | When | Use Case |
|------|------|----------|
| `pre_call` | Before sending to LLM | Prevent PII from reaching the model |
| `post_call` | After LLM response | Mask PII in generated output |
| `during_call` | Both pre and post | Maximum protection |
| `logging_only` | Log only, don't mask | Audit without blocking |

### Entity Actions

| Action | Behavior | Example |
|--------|----------|---------|
| `MASK` | Replace entity with type tag | `john@example.com` → `<EMAIL_ADDRESS>` |
| `BLOCK` | Reject the entire request | Returns 400 error |

## Per-Request Opt-In

When `defaultOn: false`, clients opt in by adding the guardrail to their request:

```bash
curl -X POST http://litellm:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-default" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-model",
    "messages": [{"role": "user", "content": "My email is john@example.com"}],
    "metadata": {"guardrails": ["presidio-pii"]}
  }'
```

## Supported Entities

Presidio supports 20+ entity types out of the box:

| Entity | Description |
|--------|-------------|
| `CREDIT_CARD` | Credit card numbers |
| `EMAIL_ADDRESS` | Email addresses |
| `PHONE_NUMBER` | Phone numbers (international) |
| `US_SSN` | US Social Security Numbers |
| `IP_ADDRESS` | IPv4 and IPv6 addresses |
| `PERSON` | Person names |
| `LOCATION` | Physical addresses, cities |
| `DATE_TIME` | Dates and timestamps |
| `NRP` | Nationality, religion, political group |
| `MEDICAL_LICENSE` | Medical license numbers |

For the full list, see [Presidio supported entities](https://microsoft.github.io/presidio/supported_entities/).

## Multi-Language Support

Change `language` to support other languages:

```yaml
presidio:
  enabled: true
  language: "de"   # German
```

Supported: `en`, `de`, `es`, `fr`, `it`, `pt`, `nl`, `he`, `ar`, `ja`, `zh`, `ko`.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Guardrail not appearing in LiteLLM | `presidio.enabled` not set in both security and litellm | Enable in both `security.presidio` and `litellm.guardrails.presidio` |
| False positives (over-masking) | Score threshold too low | Increase `scoreThresholds` for the entity |
| False negatives (missed PII) | Score threshold too high | Decrease `scoreThresholds` |
| Analyzer pod OOM | Large request payloads | Increase analyzer memory limits |
| Slow response times | PII scanning adds latency | Use `logging_only` mode for monitoring first |
