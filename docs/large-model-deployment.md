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
