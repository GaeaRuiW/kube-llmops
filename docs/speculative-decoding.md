# Speculative Decoding

Speculative decoding uses a small "draft" model to predict multiple tokens, then verifies them with the main model in a single forward pass. This can reduce latency by 2-3x.

## Configuration

Use `engineArgs` to configure speculative decoding:

```yaml
global:
  models:
    - name: llama-70b
      source: meta-llama/Llama-3.3-70B-Instruct
      resources:
        gpu: 4
        memory: 128Gi
      engineArgs:
        --speculative-model: "meta-llama/Llama-3.2-1B"
        --num-speculative-tokens: "5"
        --speculative-max-model-len: "2048"
```

## Draft Model Selection

- Choose a model from the same family (e.g., Llama-1B for Llama-70B)
- Smaller is better -- the draft model should be fast
- Vocabulary must match between draft and main model

## Monitoring

Speculative decoding acceptance rate is visible in the vLLM Model Serving dashboard under token throughput metrics.
