# Python Client Examples

Usage examples for kube-llmops (v0.5.0) with the OpenAI Python SDK.
The default LLM is `gemma-4-26b-a4b` (llama.cpp GGUF).

## Install

```bash
pip install openai langfuse
```

## Base URL Options

Pick one of the following for the `base_url` argument of the OpenAI client:

```python
# 1) NodePort (recommended for quick starts — no Ingress / /etc/hosts needed)
#    Deploy with: --set global.nodePort.enabled=true --set global.nodePort.host=$NODE_IP
base_url = "http://<NODE_IP>:30400/v1"

# 2) Ingress (requires *.llmops.local in /etc/hosts)
base_url = "http://litellm.llmops.local/v1"

# 3) port-forward (no Ingress, no NodePort)
#    Run: kubectl port-forward svc/kube-llmops-litellm 4000:4000 &
base_url = "http://localhost:4000/v1"
```

All snippets below use option (2); swap in the URL that matches your setup.

## Chat Completion

```python
# /// script
# requires-python = ">=3.10"
# dependencies = ["openai>=1.0"]
# ///
"""Chat completion via LiteLLM gateway."""

from openai import OpenAI

client = OpenAI(
    base_url="http://litellm.llmops.local/v1",
    api_key="sk-kube-llmops-dev",
)

# Basic chat
response = client.chat.completions.create(
    model="gemma-4-26b-a4b",
    messages=[{"role": "user", "content": "What is Kubernetes?"}],
    temperature=0.7,
    max_tokens=256,
)
print(response.choices[0].message.content)

# Streaming
stream = client.chat.completions.create(
    model="gemma-4-26b-a4b",
    messages=[{"role": "user", "content": "Explain PagedAttention briefly."}],
    stream=True,
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
print()
```

## Embedding

```python
# /// script
# requires-python = ">=3.10"
# dependencies = ["openai>=1.0"]
# ///
"""Generate embeddings via TEI through LiteLLM."""

from openai import OpenAI

client = OpenAI(
    base_url="http://litellm.llmops.local/v1",
    api_key="sk-kube-llmops-dev",
)

response = client.embeddings.create(
    model="bge-small-en",
    input=["kube-llmops deploys LLM infrastructure on Kubernetes"],
)

vector = response.data[0].embedding
print(f"Model: {response.model}")
print(f"Dimensions: {len(vector)}")
print(f"First 5 values: {vector[:5]}")
```

## Chat with Langfuse Tracing

```python
# /// script
# requires-python = ">=3.10"
# dependencies = ["openai>=1.0", "langfuse>=2.0"]
# ///
"""Chat with automatic Langfuse tracing."""

import os
from openai import OpenAI
from langfuse import Langfuse

os.environ.update({
    "LANGFUSE_PUBLIC_KEY": "pk-lf-kube-llmops",
    "LANGFUSE_SECRET_KEY": "sk-lf-kube-llmops",
    "LANGFUSE_HOST": "http://langfuse.llmops.local",
})

langfuse = Langfuse()
client = OpenAI(
    base_url="http://litellm.llmops.local/v1",
    api_key="sk-kube-llmops-dev",
)

# Create a traced generation
trace = langfuse.trace(name="python-example")
generation = trace.generation(
    name="chat",
    model="gemma-4-26b-a4b",
    input=[{"role": "user", "content": "What is vLLM?"}],
)

response = client.chat.completions.create(
    model="gemma-4-26b-a4b",
    messages=[{"role": "user", "content": "What is vLLM?"}],
)

answer = response.choices[0].message.content
generation.end(output=answer, usage={
    "input": response.usage.prompt_tokens,
    "output": response.usage.completion_tokens,
})
langfuse.flush()

print(f"Answer: {answer}")
print(f"Trace: http://langfuse.llmops.local/trace/{trace.id}")
```

## Batch Embedding with Cosine Similarity

```python
# /// script
# requires-python = ">=3.10"
# dependencies = ["openai>=1.0"]
# ///
"""Compute cosine similarity between texts using embeddings."""

import math
from openai import OpenAI

client = OpenAI(
    base_url="http://litellm.llmops.local/v1",
    api_key="sk-kube-llmops-dev",
)

texts = [
    "vLLM uses PagedAttention for efficient memory management",
    "LiteLLM provides a unified API gateway for LLM providers",
    "PagedAttention reduces GPU memory waste in LLM serving",
]

response = client.embeddings.create(model="bge-small-en", input=texts)
vectors = [d.embedding for d in response.data]


def cosine_sim(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    return dot / (na * nb)


print("Cosine similarity matrix:")
for i, t1 in enumerate(texts):
    for j, t2 in enumerate(texts):
        sim = cosine_sim(vectors[i], vectors[j])
        print(f"  [{i}][{j}] = {sim:.4f}", end="")
    print()
```

## Using with port-forward (no Ingress)

```python
# Forward first: kubectl port-forward svc/kube-llmops-litellm 4000:4000 &
client = OpenAI(base_url="http://localhost:4000/v1", api_key="sk-kube-llmops-dev")
```
