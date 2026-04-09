# Large Model Deployment Guide

## Tensor Parallelism (Multi-GPU)

For models too large for a single GPU, use tensor parallelism:

```yaml
global:
  models:
    - name: llama-70b
      source: meta-llama/Llama-3.3-70B-Instruct
      resources:
        gpu: 4          # 4-way tensor parallel
        memory: 128Gi
```

vLLM automatically sets `--tensor-parallel-size` based on `resources.gpu`.

## Memory Estimation

Rule of thumb for FP16/BF16: **model params x 2 bytes + 20% overhead**.

| Model Size | FP16 Weight | Min GPU Memory |
|-----------|-------------|----------------|
| 7B | ~14 GB | 1x 24GB GPU |
| 13B | ~26 GB | 2x 24GB GPU |
| 70B | ~140 GB | 4x 80GB GPU |

## Quantization

AWQ and GPTQ models use less memory:

```yaml
- name: model-awq
  source: org/model-AWQ
  engineArgs:
    --dtype: "half"
    --quantization: "awq"
```

## MoE Models (Mixture of Experts)

MoE models (e.g., Mixtral, DeepSeek) activate only a subset of parameters per token. Weight memory is full size but compute is lower.

```yaml
- name: deepseek-r1
  source: deepseek-ai/DeepSeek-R1
  resources:
    gpu: 8
    memory: 256Gi
```

## MIG GPU Sharing

For small models (embedding, reranker), use MIG to share a GPU:

```yaml
- name: bge-small
  source: BAAI/bge-small-en-v1.5
  resources:
    gpu: 0
    migDevice: "nvidia.com/mig-1g.5gb"
    memory: 2Gi
```

## Pipeline Parallelism

For models that exceed the memory of a single node, combine tensor and pipeline parallelism:

```yaml
- name: llama-405b
  source: meta-llama/Llama-3.1-405B-Instruct
  resources:
    gpu: 8
    memory: 512Gi
  engineArgs:
    --tensor-parallel-size: "4"
    --pipeline-parallel-size: "2"
```

This splits the model across 2 nodes with 4 GPUs each (8 total).

## Spot/Preemptible GPU Instances

Reduce cost by running on spot instances:

```yaml
- name: my-model
  source: org/model
  spotToleration: true
  resources:
    gpu: 1
    memory: 24Gi
```

This adds tolerations for AWS spot, GCP preemptible, and Karpenter spot capacity types. Combine with KEDA autoscaling for automatic replacement.

## Memory Optimization Tips

1. **KV Cache**: vLLM auto-computes `--max-model-len` based on available GPU memory. Override with `engineArgs: { "--max-model-len": "4096" }` to reduce memory usage.
2. **GPU Memory Utilization**: Default is 0.8 (80%). Increase to 0.9 for memory-constrained setups: `engineArgs: { "--gpu-memory-utilization": "0.9" }`.
3. **Unified Memory (GB10/Grace Blackwell)**: Enable `unifiedMemory.enabled: true` for integrated GPU systems.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| OOM on model load | Insufficient GPU memory | Reduce `--max-model-len` or add GPUs |
| Slow first token | Model loading from HuggingFace | Check MinIO cache hit; rebuild model-loader image |
| CUDA out of memory at runtime | KV cache too large | Lower `--max-model-len` or `--gpu-memory-utilization` |
| Pod pending | No GPU node available | Check `kubectl describe node` for allocatable GPUs |
