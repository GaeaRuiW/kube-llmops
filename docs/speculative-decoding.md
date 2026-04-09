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

## How It Works

Speculative decoding uses a small "draft" model to generate candidate tokens, then verifies them in parallel with the main model. Correct tokens are accepted without re-computation, yielding 1.5-2.5x throughput improvement for latency-bound workloads.

## Performance Characteristics

| Scenario | Expected Speedup | Best For |
|----------|-----------------|----------|
| Code generation | 2-2.5x | Repetitive syntax patterns |
| Translation | 1.5-2x | Common phrases and grammar |
| Creative writing | 1.2-1.5x | Less predictable output |
| Mathematical reasoning | 1.1-1.3x | Diverse token distribution |

Speedup depends on the acceptance rate — how often the draft model's predictions match the main model.

## Configuration Tips

1. **Draft model size**: Use a model ~10x smaller than the main model (e.g., 0.5B draft for 7B main)
2. **Speculative tokens**: Start with 5, increase to 8-10 for highly predictable workloads
3. **Max model length**: Both models must support the same max sequence length
4. **Same tokenizer**: Draft and main models should share the same tokenizer for best results

## When NOT to Use Speculative Decoding

- **Batch-heavy workloads**: Throughput-optimized scenarios (high batch sizes) don't benefit
- **Embedding/reranking**: Only applicable to autoregressive text generation
- **Very small models**: The draft model overhead outweighs speedup gains

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| No speedup observed | Low acceptance rate | Try a more capable draft model |
| Higher latency than baseline | Draft model too large | Use a smaller draft model |
| OOM error | Combined model memory exceeds GPU | Reduce `--max-model-len` or add GPUs |
| Tokenizer mismatch warnings | Different tokenizers | Use models from the same family |
