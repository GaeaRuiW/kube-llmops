# Model Updates (Canary Deployment)

## Overview

Update model versions with zero downtime using canary deployments.

## Configuration

```yaml
global:
  models:
    - name: qwen3-8b
      source: Qwen/Qwen3-8B
      replicas: 2
      canary:
        enabled: true
        source: Qwen/Qwen3-8B-v2
        weight: 10
        replicas: 1
```

## Promotion Flow

1. **Deploy canary at 10%**
   ```bash
   helm upgrade kube-llmops charts/kube-llmops-stack \
     --set 'global.models[0].canary.enabled=true' \
     --set 'global.models[0].canary.weight=10'
   ```

2. **Monitor** -- Check LiteLLM AI Gateway dashboard for canary vs primary latency

3. **Increase to 50%**
   ```bash
   helm upgrade ... --set 'global.models[0].canary.weight=50'
   ```

4. **Promote** -- Update primary source, disable canary
   ```bash
   helm upgrade ... \
     --set 'global.models[0].source=Qwen/Qwen3-8B-v2' \
     --set 'global.models[0].canary.enabled=false'
   ```

## Rollback

Set canary weight to 0 or disable canary:

```bash
helm upgrade ... --set 'global.models[0].canary.enabled=false'
```

## Monitoring Canary Deployments

### Key Metrics

Monitor these on the **LiteLLM AI Gateway** Grafana dashboard during canary rollout:

| Metric | Stable Baseline | Canary Alert Threshold |
|--------|----------------|----------------------|
| Error rate | < 1% | > 5% |
| P95 latency | Model-specific | > 2x baseline |
| Token throughput | Model-specific | < 50% of baseline |

### Automated Quality Check

Use the Ragas evaluation CronJob to compare canary vs stable quality:

```bash
# Trigger evaluation targeting the canary
kubectl create job ragas-canary --from=cronjob/kube-llmops-ragas-eval
kubectl logs job/ragas-canary --tail=50
```

## Rollback Strategies

### Immediate Rollback

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set 'global.models[0].canary.enabled=false' --no-hooks
```

### Gradual Rollback

Decrease canary weight step-by-step:
```bash
# 10% → 5% → 0%
helm upgrade kube-llmops ... --set 'global.models[0].canary.weight=5' --no-hooks
helm upgrade kube-llmops ... --set 'global.models[0].canary.enabled=false' --no-hooks
```

## Best Practices

1. **Start small**: Begin with 5-10% canary weight
2. **Bake time**: Run canary for at least 1 hour before promotion
3. **Compare metrics**: Check P95 latency and error rate on Grafana before promoting
4. **Use same prompts**: Test canary with production traffic, not synthetic
5. **Automate**: Integrate with CI/CD to auto-promote if metrics are healthy
