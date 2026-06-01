# Engine Selection Guide

kube-llmops supports five inference engines: **vLLM**, **SGLang**, **llama.cpp**, **Chitu (赤兔)**, and **TEI**.
The engine for each model is automatically selected by a capability-based resolution algorithm,
or can be explicitly specified per model.

## Supported Engines

| Engine | Best For | Port | Image | API |
|--------|----------|------|-------|-----|
| vLLM | General LLM serving | 8000 | `vllm/vllm-openai` | OpenAI `/v1` |
| SGLang | MoE models, VLM, prefix caching | 30000 | `lmsysorg/sglang:latest-runtime` | OpenAI `/v1` |
| llama.cpp | GGUF quantized models, CPU/edge | 8080 | `ghcr.io/ggml-org/llama.cpp:server-cuda` | OpenAI `/v1` |
| Chitu (赤兔) | Domestic GPUs (Ascend/Hygon/Muxi), DeepSeek | 21002 | `chitu-nvidia_arch_90:latest` | OpenAI `/v1` |
| TEI | Embedding and reranking | 8080 | `text-embeddings-inference` | HuggingFace |

## Auto-Detection Algorithm

When no explicit `engine` is set on a model (or when `engine: auto`), kube-llmops walks
a priority chain to determine the best engine. The first match wins:

| Priority | Check | Result | Example Source |
|----------|-------|--------|----------------|
| 1 | Explicit `engine` field (not `""`, not `"auto"`) | Use as-is | `engine: vllm` |
| 2 | Source contains `gguf` or `guff` | `llamacpp` | `TheBloke/Mistral-7B-GGUF` |
| 3 | Source matches embedding/reranker pattern (`rerank`, `bge-`, `e5-`, `gte-`, `jina-embed`, `nomic-embed`, `embedding`, etc.) | `tei` | `BAAI/bge-small-en-v1.5` |
| 4 | Feature tag `domestic-gpu` | `chitu` | `features: [domestic-gpu]` |
| 5a | Feature tag `moe` | `sglang` | `features: [moe]` |
| 5b | Feature tag `vlm` | `sglang` | `features: [vlm]` |
| 6a | Source auto-detect MoE (DeepSeek V3/V4/R1 non-distill, Qwen3 `*b-a*b`, Mixtral, GLM 4.5+, Kimi K2+) | `sglang` | `deepseek-ai/DeepSeek-R1` |
| 6b | Source auto-detect VLM (`-vl-`, `-vlm`, `-vision`, GLM `*v`) | `sglang` | `Qwen/Qwen2.5-VL-7B-Instruct` |
| 7 | None of the above | `global.defaultLLMEngine` (default `"vllm"`) | `Qwen/Qwen3-8B` |

**Note:** DeepSeek distill variants (e.g. `DeepSeek-R1-Distill-Qwen-7B`) are dense models
and are _not_ detected as MoE — they fall through to the default engine.

## Feature Tags

Feature tags are user-specified hints on a model entry. They override source-based
auto-detection but are themselves overridden by an explicit `engine` field.

| Tag | Effect | Rationale |
|-----|--------|-----------|
| `domestic-gpu` | Route to Chitu | Chitu supports Ascend (A2/A3), Hygon, Muxi, MooreThreads |
| `moe` | Route to SGLang | Better Expert Parallelism, DeepEP support |
| `vlm` | Route to SGLang | RadixAttention benefits multimodal prefix caching |

```yaml
global:
  models:
    - name: qwen3-ascend
      source: Qwen/Qwen3-8B
      features: [domestic-gpu]    # → chitu
```

## Configuration Examples

### Auto-detection (no config needed)

In the common case, no `engine` or `features` fields are required — the source name
is enough:

```yaml
global:
  models:
    - name: deepseek-r1
      source: deepseek-ai/DeepSeek-R1      # Auto → sglang (MoE detected)
    - name: qwen3-8b
      source: Qwen/Qwen3-8B                # Auto → vllm (dense model)
    - name: qwen-vl
      source: Qwen/Qwen2.5-VL-7B-Instruct  # Auto → sglang (VLM detected)
    - name: bge-embed
      source: BAAI/bge-small-en-v1.5        # Auto → tei (embedding detected)
```

### Feature tags

Use feature tags when auto-detection cannot infer the correct engine from the
source name alone:

```yaml
global:
  models:
    - name: qwen3-ascend
      source: Qwen/Qwen3-8B
      features: [domestic-gpu]    # → chitu
    - name: custom-moe
      source: my-org/custom-moe-model
      features: [moe]            # → sglang
```

### Explicit engine override

Force a specific engine regardless of what auto-detection would pick:

```yaml
global:
  models:
    - name: deepseek-r1
      source: deepseek-ai/DeepSeek-R1
      engine: vllm    # Force vLLM even though auto-detect would pick sglang
```

### Change default engine globally

Set `global.defaultLLMEngine` to change the fallback for all standard LLMs
that don't match any auto-detection rule:

```yaml
global:
  defaultLLMEngine: sglang    # All standard LLMs use SGLang by default
```

## Chitu Multi-Hardware Support

Chitu (赤兔) supports multiple domestic and NVIDIA GPU architectures via
hardware-specific container images:

| Hardware | Image Tag |
|----------|-----------|
| NVIDIA Hopper (arch 90) | `chitu-nvidia_arch_90:latest` |
| NVIDIA Ampere/Ada (arch 80/89) | `chitu-nvidia_arch_80_89:latest` |
| Ascend A2 | `chitu-ascend_a2:latest` |
| Ascend A3 | `chitu-ascend_a3:latest` |
| Muxi | `chitu-muxi:latest` |

The default Chitu image targets NVIDIA arch 90 (Hopper). To deploy on different
hardware, override the image in your values:

```yaml
chitu:
  image:
    repository: qingcheng-ai-cn-beijing.cr.volces.com/public/chitu-ascend_a2
    tag: latest
```

## SGLang-Specific Features

SGLang is preferred for MoE and VLM workloads because of:

- **RadixAttention** — prefix-tree-based KV cache that benefits multimodal and
  repeated-system-prompt workloads. Auto-enabled when `prefixCaching: true` is
  set on the model (injects `--enable-radix-attention`).
- **Expert Parallelism** — efficient expert placement for Mixture-of-Experts models.
- **DeepEP support** — optimized all-to-all communication for DeepSeek-class MoE.

```yaml
global:
  models:
    - name: deepseek-r1
      source: deepseek-ai/DeepSeek-R1
      prefixCaching: true    # Adds --enable-radix-attention to SGLang args
```

## Engine Arguments

Pass engine-specific command-line arguments via `engineArgs`. Keys are flag names
and values are their arguments (use `""` for boolean flags):

### vLLM / SGLang / llama.cpp

```yaml
models:
  - name: deepseek-r1
    source: deepseek-ai/DeepSeek-R1
    engineArgs:
      --tp: "8"
      --enable-radix-attention: ""
```

### Chitu (Hydra syntax)

Chitu uses Hydra-style configuration. Set `chituModel` to the model config name
and pass Hydra overrides as `engineArgs`:

```yaml
models:
  - name: deepseek-r1
    source: deepseek-ai/DeepSeek-R1
    engine: chitu
    chituModel: DeepSeek-R1    # Hydra model config name
    engineArgs:
      infer.soft_fp8: "True"
      infer.use_cuda_graph: "True"
```

## How It Works Internally

The resolution algorithm is implemented in two places (kept in sync):

- **Helm templates:** `charts/kube-llmops-stack/templates/_helpers.tpl`
  — `resolveEngine`, `isMoESource`, `isVLMSource` named templates.
  Used at `helm install` / `helm upgrade` time.
- **Go operator:** `operator/internal/engine/resolver.go`
  — `ResolveEngineEx`, `IsMoESource`, `IsVLMSource` functions.
  Used by the operator when reconciling `ModelDeployment` and `LLMPlatform` CRs.

Both implementations share the same priority chain and pattern lists. When adding
a new model family, update both files and run the test suites:

```bash
# Helm template tests
python -m pytest tests/helm/ -v

# Go unit tests
cd operator && go test ./internal/engine/...
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| MoE model runs on vLLM (OOM / slow) | Source name not in known MoE list | Add `features: [moe]` or set `engine: sglang` |
| DeepSeek-R1-Distill on SGLang unexpectedly | Source contains `deepseek-r1` but not `distill` | Verify source name; distill variants are excluded from MoE detection |
| Wrong engine after changing `global.defaultLLMEngine` | Explicit `engine` field still set on model | Remove the per-model `engine` override |
| Chitu crash on Ascend hardware | Default image targets NVIDIA arch 90 | Override `chitu.image.repository` to the correct hardware variant |
| Embedding model deployed as vLLM | Source name missing known embedding pattern | Add `engine: tei` explicitly |
