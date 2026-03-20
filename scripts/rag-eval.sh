#!/usr/bin/env bash
set -euo pipefail

# RAG Evaluation Pipeline
# Runs evaluation queries against the RAG system, scores answers,
# and pushes results to Langfuse + Prometheus.
#
# Usage:
#   ./scripts/rag-eval.sh                          # Use default eval dataset
#   EVAL_FILE=my-eval.json ./scripts/rag-eval.sh   # Use custom dataset
#   LITELLM_URL=http://localhost:4000 ./scripts/rag-eval.sh  # Custom endpoint

LITELLM_URL="${LITELLM_URL:-http://kube-llmops-litellm:4000}"
LITELLM_KEY="${LITELLM_KEY:-sk-kube-llmops-dev}"
LANGFUSE_HOST="${LANGFUSE_HOST:-http://kube-llmops-langfuse:3000}"
LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY:-pk-lf-kube-llmops}"
LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY:-sk-lf-kube-llmops}"
MODEL="${MODEL:-qwen2-5-0-5b}"
EVAL_FILE="${EVAL_FILE:-$(dirname "$0")/../examples/eval/eval-dataset.json}"

echo "============================================="
echo "  RAG Evaluation Pipeline"
echo "============================================="
echo "  LiteLLM: ${LITELLM_URL}"
echo "  Model:   ${MODEL}"
echo "  Dataset:  ${EVAL_FILE}"
echo ""

# Check prerequisites
command -v curl >/dev/null || { echo "ERROR: curl required"; exit 1; }
command -v python3 >/dev/null || { echo "ERROR: python3 required"; exit 1; }

# Run evaluation
python3 - << 'PYTHON_EVAL'
import json, os, sys, time
from urllib.request import Request, urlopen
from urllib.error import URLError

LITELLM_URL = os.environ["LITELLM_URL"]
LITELLM_KEY = os.environ["LITELLM_KEY"]
MODEL = os.environ["MODEL"]
EVAL_FILE = os.environ["EVAL_FILE"]
LANGFUSE_HOST = os.environ["LANGFUSE_HOST"]
LANGFUSE_PK = os.environ["LANGFUSE_PUBLIC_KEY"]
LANGFUSE_SK = os.environ["LANGFUSE_SECRET_KEY"]

# Load eval dataset
if os.path.exists(EVAL_FILE):
    with open(EVAL_FILE) as f:
        dataset = json.load(f)
else:
    print(f"WARNING: {EVAL_FILE} not found, using built-in test questions")
    dataset = [
        {"question": "What is Kubernetes?", "expected_keywords": ["container", "orchestrat"]},
        {"question": "What is vLLM?", "expected_keywords": ["inference", "LLM", "serving"]},
        {"question": "What is RAG?", "expected_keywords": ["retrieval", "generation", "augmented"]},
    ]

def call_llm(question):
    """Call LiteLLM API"""
    body = json.dumps({
        "model": MODEL,
        "messages": [{"role": "user", "content": question}],
        "max_tokens": 100
    }).encode()
    req = Request(
        f"{LITELLM_URL}/v1/chat/completions",
        data=body,
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {LITELLM_KEY}"}
    )
    try:
        resp = urlopen(req, timeout=30)
        data = json.loads(resp.read())
        return data["choices"][0]["message"]["content"], data.get("usage", {})
    except Exception as e:
        return f"ERROR: {e}", {}

def score_answer(answer, expected_keywords):
    """Simple keyword-based scoring (0-1)"""
    answer_lower = answer.lower()
    hits = sum(1 for kw in expected_keywords if kw.lower() in answer_lower)
    return hits / max(len(expected_keywords), 1)

def push_to_langfuse(question, answer, score, latency):
    """Push evaluation trace to Langfuse"""
    try:
        body = json.dumps({
            "batch": [{
                "id": f"eval-{int(time.time()*1000)}",
                "type": "trace-create",
                "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                "body": {
                    "name": "rag-eval",
                    "input": {"question": question},
                    "output": {"answer": answer[:200]},
                    "metadata": {"score": score, "latency_ms": latency, "model": MODEL}
                }
            }]
        }).encode()
        req = Request(
            f"{LANGFUSE_HOST}/api/public/ingestion",
            data=body,
            headers={
                "Content-Type": "application/json",
                "X-Langfuse-Public-Key": LANGFUSE_PK,
                "X-Langfuse-Sdk-Name": "kube-llmops-eval",
            }
        )
        import base64
        auth = base64.b64encode(f"{LANGFUSE_PK}:{LANGFUSE_SK}".encode()).decode()
        req.add_header("Authorization", f"Basic {auth}")
        urlopen(req, timeout=10)
    except Exception as e:
        print(f"  Langfuse push failed: {e}")

# Run evaluations
results = []
for i, item in enumerate(dataset):
    q = item["question"]
    expected = item.get("expected_keywords", [])
    
    start = time.time()
    answer, usage = call_llm(q)
    latency_ms = int((time.time() - start) * 1000)
    
    score = score_answer(answer, expected)
    results.append({"question": q, "score": score, "latency_ms": latency_ms})
    
    push_to_langfuse(q, answer, score, latency_ms)
    
    status = "PASS" if score >= 0.5 else "FAIL"
    print(f"  [{i+1}/{len(dataset)}] {status} score={score:.2f} latency={latency_ms}ms")
    print(f"    Q: {q}")
    print(f"    A: {answer[:100]}...")

# Summary
avg_score = sum(r["score"] for r in results) / max(len(results), 1)
avg_latency = sum(r["latency_ms"] for r in results) / max(len(results), 1)
pass_rate = sum(1 for r in results if r["score"] >= 0.5) / max(len(results), 1)

print(f"\n{'='*50}")
print(f"  Results: {len(results)} queries")
print(f"  Avg Score:   {avg_score:.2f}")
print(f"  Avg Latency: {avg_latency:.0f}ms")
print(f"  Pass Rate:   {pass_rate:.0%}")
print(f"{'='*50}")

# Exit with failure if pass rate < threshold
THRESHOLD = float(os.environ.get("EVAL_THRESHOLD", "0.5"))
if pass_rate < THRESHOLD:
    print(f"\nFAILED: pass rate {pass_rate:.0%} < threshold {THRESHOLD:.0%}")
    sys.exit(1)
else:
    print(f"\nPASSED: pass rate {pass_rate:.0%} >= threshold {THRESHOLD:.0%}")
PYTHON_EVAL

echo ""
echo "Evaluation complete."
