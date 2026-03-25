# SLO Guide for kube-llmops

## SLO Definitions

| SLO | Target | Metric | Window |
|-----|--------|--------|--------|
| LLM Inference Availability | 99.9% | Success rate of /v1/chat/completions | 30 days |
| E2E Latency P95 | < 2s | vllm:e2e_request_latency_seconds (p95) | 5 min |
| RAG Quality (Faithfulness) | >= 0.85 | ragas_faithfulness | Per eval run |
| Embedding Availability | 99.9% | Success rate of /v1/embeddings | 30 days |

## Error Budget

With 99.9% availability over 30 days:
- Total allowed downtime: 43.2 minutes
- Error budget: 0.1% of total requests

## Alert Configuration

Alerts are configured in Prometheus `rules.yml`:
- `RAGFaithfulnessLow`: fires when faithfulness < 0.7
- `VllmHighLatency`: fires when P95 > 10s
- `RAGQualityRegression`: fires when below 0.85 production target

## Monitoring

View SLO status in Grafana: **SLO Overview** dashboard.
