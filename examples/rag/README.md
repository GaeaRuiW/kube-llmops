# kube-llmops RAG Example

A complete Retrieval-Augmented Generation (RAG) pipeline demonstrating the full kube-llmops stack:

**Document Ingestion → pgvector Storage → Similarity Search → LLM Generation via LiteLLM**

## What This Demonstrates

| Component | Role |
|---|---|
| **pgvector** | Stores document embeddings for fast similarity search |
| **sentence-transformers** | Generates embeddings locally (all-MiniLM-L6-v2, 384-dim) |
| **LiteLLM** | Proxies LLM requests to the configured model (default: qwen2-5-0-5b) |
| **Langfuse** | Traces every ingest and query operation for observability |
| **FastAPI** | Serves the `/ingest`, `/query`, and `/health` endpoints |

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/ingest` | Embed and store a document in pgvector |
| `POST` | `/query` | Retrieve relevant docs and generate an answer via LLM |
| `GET` | `/health` | Health check |

## Quick Start (Local)

```bash
# 1. Install dependencies
pip install -r requirements.txt

# 2. Make sure kube-llmops services are running (LiteLLM, PostgreSQL, Langfuse)

# 3. Export environment variables (adjust to your setup)
export DATABASE_URL="postgresql://litellm:llmops-pg-dev-pw@localhost:5432/litellm"
export LITELLM_URL="http://localhost:4000"
export LITELLM_KEY="sk-kube-llmops-dev"
export LANGFUSE_HOST="http://localhost:3000"
export LANGFUSE_PUBLIC_KEY="pk-lf-kube-llmops"
export LANGFUSE_SECRET_KEY="sk-lf-kube-llmops"

# 4. Run the app
python app.py
```

## Quick Start (Kubernetes)

```bash
# 1. Create a ConfigMap from the application code
kubectl create configmap rag-example-code --from-file=app.py

# 2. Deploy
kubectl apply -f k8s-deployment.yaml

# 3. Port-forward to access locally
kubectl port-forward svc/rag-example 8000:8000
```

## Usage

```bash
# Ingest a document
curl -X POST http://localhost:8000/ingest \
  -H "Content-Type: application/json" \
  -d '{"content": "kube-llmops is a Kubernetes-native MLOps platform for managing LLM deployments, observability, and orchestration.", "metadata": {"source": "docs"}}'

# Query with RAG
curl -X POST http://localhost:8000/query \
  -H "Content-Type: application/json" \
  -d '{"question": "What is kube-llmops?", "top_k": 3}'
```

## Architecture

```
User ─── POST /ingest ──→ embed(text) ──→ pgvector INSERT
User ─── POST /query  ──→ embed(question) ──→ pgvector similarity search
                                            ──→ build context
                                            ──→ LiteLLM chat completion
                                            ──→ Langfuse trace
                                            ──→ return answer + sources
```

## Configuration

All settings are controlled via environment variables (see `app.py` header for defaults).
