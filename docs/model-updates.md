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
