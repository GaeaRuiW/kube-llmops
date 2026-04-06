# Phase 6 Sub-project 2: kubectl-llmops CLI — Design Spec

> **Status:** Approved (v1.0.0)
> **Date:** 2026-04-06
> **Checkpoint:** `kubectl llmops deploy qwen3.5-122b` works end-to-end

## 1. Overview

A kubectl plugin that provides imperative shortcuts for the kube-llmops operator's three CRDs (ModelDeployment, LLMPlatform, FineTuneRun) plus developer-experience commands (logs, test, port-forward) and RAG operations (via Dify API).

**Delivery:** Single Go binary `kubectl-llmops` in the `operator/` module, sharing `go.mod` and reusing existing packages (`api/v1alpha1`, `internal/engine`, `internal/gateway`).

**Interaction model:** Pure CLI flags (no interactive prompts). Output follows kubectl conventions: `-o table|json|yaml|wide`.

## 2. Command Hierarchy

Mixed mode: high-frequency model operations are top-level; finetune, platform, and rag are two-level subcommands.

### 2.1 Global Flags

All commands inherit:

```
-n, --namespace string     Kubernetes namespace (default: current context)
-o, --output string        Output format: table|json|yaml|wide (default: table)
    --kubeconfig string    Path to kubeconfig (default: ~/.kube/config)
    --context string       Kubernetes context to use
```

### 2.2 Top-Level Commands (Model Operations + DX)

#### deploy

```
kubectl llmops deploy <source> [flags]
```

Creates a ModelDeployment CR from a HuggingFace model ID.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | derived from source | Deployment name |
| `--engine` | string | `auto` | Engine: auto\|vllm\|tei\|llamacpp |
| `--replicas` | int | 1 | Replica count |
| `--gpu` | int | 1 | GPU count |
| `--memory` | string | `16Gi` | Memory limit |
| `--cpu` | string | `4` | CPU limit |
| `--accelerator` | string | `nvidia` | GPU vendor: nvidia\|amd\|gaudi |
| `--engine-arg` | KEY=VALUE | — | Extra engine args (repeatable) |
| `--prefix-caching` | bool | false | Enable vLLM prefix caching |
| `--dry-run` | bool | false | Print YAML without applying |

**Name derivation:** Takes the part after `/`, lowercases, truncates to 63 chars. E.g. `Qwen/Qwen2.5-7B-Instruct` → `qwen2.5-7b-instruct`.

#### list

```
kubectl llmops list [flags]
  -A, --all-namespaces     List across all namespaces
  -w, --watch              Watch for changes
```

#### status

```
kubectl llmops status <name> [flags]
  -w, --watch              Watch for changes
```

Shows phase, engine, replicas, endpoint, conditions, and canary status if present.

#### scale

```
kubectl llmops scale <name> --replicas=N
```

Patches `spec.replicas` on the ModelDeployment.

#### delete

```
kubectl llmops delete <name> [flags]
  --force                  Skip confirmation prompt
```

Prompts `Are you sure? [y/N]` unless `--force`.

#### canary

```
kubectl llmops canary <name> [flags]
  --target string          Canary model source (required)
  --weight int             Traffic weight 0-100 (required)
```

Patches `spec.canary` on the existing ModelDeployment.

#### promote

```
kubectl llmops promote <name>
```

Replaces `spec.source` with `spec.canary.source`, removes `spec.canary`.

#### rollback

```
kubectl llmops rollback <name>
```

Removes `spec.canary`, keeps original `spec.source`.

#### logs

```
kubectl llmops logs <name> [flags]
  -f, --follow             Stream logs
  --tail int               Lines to show (default: 100)
  --container string       Container name (default: model-server)
```

Finds pods by label `kube-llmops/model=<name>`, streams logs.

#### test

```
kubectl llmops test <name> [flags]
  --prompt string          Test prompt (default: "Hello")
  --stream                 Stream response
```

Discovers the LiteLLM gateway, sends a `/v1/chat/completions` request with the model name. Only works for LLM models (engine=vllm or llamacpp). For embedding/reranker models, prints an error: `Error: model "xxx" is an embedding/reranker model and does not support chat`.

#### endpoint

```
kubectl llmops endpoint <name>
```

Prints the model's API endpoint URL (from ModelDeployment status or gateway).

#### port-forward

```
kubectl llmops port-forward [flags]
  --service string         Service: gateway|grafana|langfuse|dify|minio (default: gateway)
```

Service mapping:

| Service | K8s Service | Local Port |
|---------|-------------|------------|
| gateway | kube-llmops-litellm | 4000 |
| grafana | kube-llmops-grafana | 3000 |
| langfuse | kube-llmops-langfuse | 3001 |
| dify | kube-llmops-dify-web | 5001 |
| minio | kube-llmops-minio | 9001 |

#### dashboard

```
kubectl llmops dashboard
```

Prints the Grafana URL (NodePort or port-forward). Opens browser if possible (`xdg-open` / `open`).

#### migrate

```
kubectl llmops migrate <helm-release> [flags]
  --output-dir string      Dir for generated CRs (default: ./generated)
```

Reads Helm release values, generates LLMPlatform + ModelDeployment CRs. Replaces `cmd/migrate/main.go` logic.

### 2.3 `finetune` Subcommand Group

#### finetune create

```
kubectl llmops finetune create [flags]
  --base-model string      Base model source (required)
  --output-name string     Output model name (required)
  --method string          Training method: lora|qlora|full (default: lora)
  --data-source string     Data path, e.g. s3://datasets/train.json (required)
  --data-format string     Data format: alpaca|sharegpt (default: alpaca)
  --epochs int             Training epochs (default: 3)
  --batch-size int         Batch size (default: 4)
  --learning-rate string   Learning rate (default: 2e-4)
  --lora-rank int          LoRA rank (default: 16)
  --lora-alpha int         LoRA alpha (default: 32)
  --gpu int                GPU count (default: 1)
  --eval                   Enable evaluation
  --quality-gate           Enable quality gate (requires --eval)
  --auto-deploy            Auto-deploy on success
  --canary-weight int      Canary weight for auto-deploy (default: 0)
  --dry-run                Print YAML without applying
```

#### finetune list

```
kubectl llmops finetune list [flags]
  -A, --all-namespaces
```

#### finetune status

```
kubectl llmops finetune status <name> [flags]
  -w, --watch
```

Shows phase, metrics (train/eval loss), duration, quality gate result, MLflow link.

#### finetune logs

```
kubectl llmops finetune logs <name> [flags]
  -f, --follow
  --step string            DAG step: prepare-data|finetune|merge-upload|evaluate|quality-gate|deploy
```

Finds Argo Workflow pods by label, streams step logs.

#### finetune delete

```
kubectl llmops finetune delete <name> [flags]
  --force
```

### 2.4 `platform` Subcommand Group

#### platform init

```
kubectl llmops platform init [flags]
  --gateway                Enable gateway (default: true)
  --observability          Enable observability (default: true)
  --logging                Enable logging (default: false)
  --rag                    Enable RAG module (default: false)
  --finetune               Enable finetune module (default: false)
  --security               Enable security module (default: false)
  --nodeport-host string   NodePort host IP
  --dry-run                Print YAML without applying
```

Creates a LLMPlatform CR with sensible defaults (gateway + observability + model store + postgresql enabled). ModelStore defaults: endpoint=`kube-llmops-minio:9000`, bucket=`models`, accessKey/secretKey=`minioadmin`.

#### platform status

```
kubectl llmops platform status [flags]
  -w, --watch
```

Shows overall phase + per-component health (gateway, grafana, prometheus, langfuse, minio, postgresql, dify).

#### platform update

```
kubectl llmops platform update [flags]
  --enable string          Enable module (repeatable): rag|finetune|security|logging
  --disable string         Disable module (repeatable)
```

Patches the LLMPlatform CR.

### 2.5 `rag` Subcommand Group

RAG commands wrap Dify REST API. The CLI auto-discovers the Dify endpoint (from K8s Service/NodePort) and API key (from K8s Secret).

#### rag list-kb

```
kubectl llmops rag list-kb
```

#### rag create-kb

```
kubectl llmops rag create-kb <name> [flags]
  --description string         KB description
  --embedding-model string     Embedding model (default: auto-discover from LiteLLM)
```

#### rag upload

```
kubectl llmops rag upload <kb-name> <file...> [flags]
  --chunk-size int             Chunk size
  --chunk-overlap int          Chunk overlap
```

#### rag delete-kb

```
kubectl llmops rag delete-kb <name> [flags]
  --force
```

#### rag query

```
kubectl llmops rag query <kb-name> --prompt "..." [flags]
  --top-k int                  Retrieval count (default: 3)
  --stream                     Stream output
```

#### rag eval

```
kubectl llmops rag eval [flags]
  --wait                       Wait for completion and print results
```

Creates a K8s Job from CronJob `kube-llmops-ragas-eval`.

## 3. Architecture

### 3.1 Code Layout

```
operator/
├── cmd/
│   ├── main.go                          # operator binary (unchanged)
│   ├── migrate/main.go                  # kept for backward compat, delegates to CLI
│   └── kubectl-llmops/
│       └── main.go                      # CLI entrypoint: cobra root command
├── internal/
│   ├── cli/
│   │   ├── cmd/                         # cobra command definitions
│   │   │   ├── root.go                  # root command + global flags
│   │   │   ├── deploy.go
│   │   │   ├── list.go
│   │   │   ├── status.go
│   │   │   ├── scale.go
│   │   │   ├── delete.go
│   │   │   ├── canary.go               # canary + promote + rollback
│   │   │   ├── logs.go
│   │   │   ├── test_cmd.go
│   │   │   ├── endpoint.go
│   │   │   ├── portforward.go
│   │   │   ├── dashboard.go
│   │   │   ├── migrate.go
│   │   │   ├── finetune.go             # finetune subcommand group
│   │   │   ├── platform.go             # platform subcommand group
│   │   │   └── rag.go                  # rag subcommand group
│   │   ├── printer/
│   │   │   └── printer.go              # table/json/yaml/wide output
│   │   └── util/
│   │       ├── kube.go                  # kubeconfig + client init
│   │       └── discovery.go             # auto-discover endpoints/secrets
│   ├── engine/                          # existing, reused by CLI
│   ├── gateway/                         # existing, reused by CLI
│   ├── builder/                         # existing, reused by CLI
│   └── controller/                      # existing, CLI does not touch
├── api/v1alpha1/                        # existing, reused by CLI
```

### 3.2 Dependency Rules

- CLI uses `client-go` for K8s API, NOT `controller-runtime`.
- CLI reuses: `api/v1alpha1` (CRD types), `internal/engine` (auto-detection), `internal/gateway` (LiteLLM client).
- CLI does NOT import `internal/controller` or `internal/helmbridge`.
- New packages: `internal/cli/cmd`, `internal/cli/printer`, `internal/cli/util`.

### 3.3 Build Integration

```makefile
# Added to operator/Makefile
.PHONY: build-cli
build-cli: fmt vet
	go build -o bin/kubectl-llmops ./cmd/kubectl-llmops/

.PHONY: install-cli
install-cli: build-cli
	cp bin/kubectl-llmops $(shell go env GOPATH)/bin/
```

kubectl plugin discovery: any binary named `kubectl-<plugin>` on `$PATH` is auto-registered. `kubectl llmops <args>` invokes `kubectl-llmops <args>`.

## 4. Data Flow

### 4.1 Model Deploy Flow

```
kubectl llmops deploy Qwen/Qwen2.5-7B --gpu=1 --memory=24Gi
  │
  ├─ 1. Parse flags → ModelDeploymentSpec
  ├─ 2. engine.ResolveEngine(source) → "vllm"
  ├─ 3. Derive name: slug("Qwen/Qwen2.5-7B") → "qwen2.5-7b"
  ├─ 4. Construct ModelDeployment CR
  ├─ 5. --dry-run? → printer.Print(cr, "yaml") → exit 0
  ├─ 6. client-go Create(cr) → API Server → Operator reconciles
  └─ 7. Print created resource status
```

### 4.2 Test Command Flow

```
kubectl llmops test gemma-4-26b --prompt="What is 2+2?"
  │
  ├─ 1. client-go Get(ModelDeployment) → status.endpoint
  ├─ 2. discovery.FindGatewayEndpoint() → NodePort or ClusterIP
  ├─ 3. gateway.HealthCheck()
  ├─ 4. POST /v1/chat/completions {model: name, messages: [...]}
  └─ 5. printer.Print(response, format)
```

### 4.3 RAG Upload Flow

```
kubectl llmops rag upload my-kb ./guide.pdf
  │
  ├─ 1. discovery.FindDifyEndpoint() → from Service/NodePort
  ├─ 2. discovery.FindDifyAPIKey() → from Secret
  ├─ 3. POST /v1/datasets/{kb-id}/document/create_by_file
  └─ 4. printer.Print(result, format)
```

### 4.4 Port-Forward Flow

```
kubectl llmops port-forward --service=gateway
  │
  ├─ 1. Resolve Service name → "kube-llmops-litellm"
  ├─ 2. client-go portforward.New() → tunnel
  ├─ 3. Print "Forwarding kube-llmops-litellm:4000 → localhost:4000"
  └─ 4. Block until Ctrl+C
```

### 4.5 Name Derivation

When `--name` is omitted, derive from source:

| Source | Derived Name |
|--------|-------------|
| `Qwen/Qwen2.5-7B-Instruct` | `qwen2.5-7b-instruct` |
| `cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit` | `gemma-4-26b-a4b-it-awq-4bit` |
| `BAAI/bge-small-en-v1.5` | `bge-small-en-v1.5` |
| `TheBloke/Llama-2-7B-GGUF` | `llama-2-7b-gguf` |

Rule: take part after `/`, lowercase, truncate to 63 chars (K8s name limit). If source has no `/`, use the full string lowercased.

## 5. Error Handling

### 5.1 Error Categories

| Category | Example | Behavior |
|----------|---------|----------|
| **Argument validation** | `--replicas=-1`, missing required flag | Print usage + error, exit 1 |
| **K8s connection** | bad kubeconfig, cluster unreachable | `Error: unable to connect to cluster: <reason>` |
| **CRD not installed** | operator not deployed | `Error: ModelDeployment CRD not found. Install the operator first: helm install kube-llmops-operator ...` |
| **Resource not found** | `status <name>` on nonexistent | `Error: ModelDeployment "xxx" not found in namespace "default"` |
| **Resource already exists** | `deploy` duplicate name | `Error: ModelDeployment "xxx" already exists. Use 'kubectl llmops scale' to update replicas or 'kubectl llmops canary' for model updates` |
| **Webhook rejection** | invalid engine, canary weight > 100 | Pass through API Server admission error message |
| **Service unreachable** | `test` when gateway down | `Error: LiteLLM gateway is not reachable. Check 'kubectl llmops platform status'` |
| **Dify not available** | RAG command, Dify not deployed | `Error: Dify is not available. Enable RAG module: kubectl llmops platform update --enable rag` |

### 5.2 Confirmation

Only destructive operations require confirmation:
- `kubectl llmops delete <name>` → `Are you sure you want to delete ModelDeployment "xxx"? [y/N]:`
- `kubectl llmops finetune delete <name>` → same pattern
- `kubectl llmops rag delete-kb <name>` → same pattern

All support `--force` to skip.

### 5.3 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (bad args, API error) |
| 2 | Resource not found |
| 3 | Timeout (waiting for phase=Ready) |

### 5.4 `--dry-run`

All write operations support `--dry-run`: constructs the full CR, prints as YAML (or requested format), does not call API Server, exits 0.

Supported by: `deploy`, `scale`, `canary`, `promote`, `rollback`, `finetune create`, `platform init`, `platform update`.

## 6. Testing

### 6.1 Test Layers

| Layer | Tool | Scope | Count |
|-------|------|-------|-------|
| Unit | Go `testing` | Flag parsing, name derivation, printer formatting, discovery logic | ~30 |
| Integration | envtest | CRD CRUD, --dry-run, error handling, output format | ~25 |
| E2E | kind + operator | Full deploy→status→scale→test→delete flow | ~10 |

### 6.2 Unit Tests

```
// Name derivation
TestSlugFromSource_Simple
TestSlugFromSource_LongName              // >63 char truncation
TestSlugFromSource_SpecialChars

// Printer
TestPrintTable_ModelDeployment
TestPrintJSON_ModelDeployment
TestPrintYAML_ModelDeployment
TestPrintWide_ModelDeployment

// Discovery
TestFindGatewayEndpoint_NodePort
TestFindGatewayEndpoint_ClusterIP
TestFindDifyEndpoint_NotDeployed
```

### 6.3 Integration Tests

envtest provides a fake K8s API Server with CRDs pre-registered:

```
// deploy
TestDeploy_CreatesModelDeployment
TestDeploy_WithAllFlags
TestDeploy_DryRun
TestDeploy_AlreadyExists
TestDeploy_EngineAutoDetect

// list / status
TestList_EmptyNamespace
TestList_MultipleModels
TestStatus_ShowsConditions

// canary
TestCanary_AddsCanaryToExisting
TestPromote_ReplacesSource
TestRollback_RemovesCanary

// finetune
TestFinetuneCreate_GeneratesCorrectCR
TestFinetuneCreate_QualityGateRequiresEval

// platform
TestPlatformInit_MinimalDefaults
TestPlatformUpdate_EnableModule
```

### 6.4 E2E Tests

Shared infrastructure with operator E2E (kind cluster):

```
TestE2E_DeployAndDelete
TestE2E_ScaleUpDown
TestE2E_ListAndStatus
TestE2E_DryRunNoSideEffects
```

### 6.5 Test File Locations

```
operator/internal/cli/
├── cmd/
│   ├── deploy_test.go
│   ├── list_test.go
│   ├── canary_test.go
│   ├── finetune_test.go
│   └── platform_test.go
├── printer/
│   └── printer_test.go
└── util/
    ├── kube_test.go
    └── discovery_test.go
```

## 7. Scope Summary

### In Scope (v1.0.0)

- kubectl plugin binary (`kubectl-llmops`)
- Model CRUD: deploy, list, status, scale, delete
- Canary: canary, promote, rollback
- Fine-tune: create, list, status, logs, delete
- Platform: init, status, update
- RAG: list-kb, create-kb, upload, delete-kb, query, eval
- DX: logs, test, endpoint, port-forward, dashboard
- Migration: migrate from Helm release
- Output: table, json, yaml, wide
- Dry-run for all write operations
- Unit + integration + E2E tests

### Out of Scope (future)

- Interactive prompts / wizard mode
- Shell completion generation (can add later with `cobra.GenBashCompletion`)
- Plugin distribution via krew
- `kubectl llmops version` with operator version check
- Telemetry / usage analytics
