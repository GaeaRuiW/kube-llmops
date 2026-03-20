"""
kube-llmops RAG Example
Demonstrates: pgvector storage → similarity search → LLM generation via LiteLLM
"""

import os
import json
import hashlib
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import psycopg2
import psycopg2.extras
from pgvector.psycopg2 import register_vector
from openai import OpenAI
from langfuse import Langfuse
from langfuse.decorators import observe, langfuse_context

# ---------------------------------------------------------------------------
# Config from environment
# ---------------------------------------------------------------------------
DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://litellm:llmops-pg-dev-pw@kube-llmops-litellm-pg:5432/litellm")
LITELLM_URL = os.getenv("LITELLM_URL", "http://kube-llmops-litellm:4000")
LITELLM_KEY = os.getenv("LITELLM_KEY", "sk-kube-llmops-dev")
LANGFUSE_HOST = os.getenv("LANGFUSE_HOST", "http://kube-llmops-langfuse:3000")
LANGFUSE_PUBLIC_KEY = os.getenv("LANGFUSE_PUBLIC_KEY", "pk-lf-kube-llmops")
LANGFUSE_SECRET_KEY = os.getenv("LANGFUSE_SECRET_KEY", "sk-lf-kube-llmops")
MODEL_NAME = os.getenv("MODEL_NAME", "qwen2-5-0-5b")
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIM", "384"))  # all-MiniLM-L6-v2 dimension

# ---------------------------------------------------------------------------
# Clients
# ---------------------------------------------------------------------------
llm = OpenAI(base_url=f"{LITELLM_URL}/v1", api_key=LITELLM_KEY)
langfuse = Langfuse(
    host=LANGFUSE_HOST,
    public_key=LANGFUSE_PUBLIC_KEY,
    secret_key=LANGFUSE_SECRET_KEY,
)

conn = None
embedder = None


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
def get_db():
    """Return a (re-usable) psycopg2 connection with pgvector registered."""
    global conn
    if conn is None or conn.closed:
        conn = psycopg2.connect(DATABASE_URL)
        conn.autocommit = True
        register_vector(conn)
    return conn


def get_embedder():
    """Lazy-load sentence transformer (downloads model on first use)."""
    global embedder
    if embedder is None:
        from sentence_transformers import SentenceTransformer
        embedder = SentenceTransformer("all-MiniLM-L6-v2")
    return embedder


def embed_text(text: str) -> list:
    """Embed a single text string and return a plain Python list of floats."""
    return get_embedder().encode(text).tolist()


# ---------------------------------------------------------------------------
# App lifespan – set up pgvector table on startup
# ---------------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    db = get_db()
    cur = db.cursor()
    cur.execute("CREATE EXTENSION IF NOT EXISTS vector;")
    cur.execute(f"""
        CREATE TABLE IF NOT EXISTS documents (
            id TEXT PRIMARY KEY,
            content TEXT NOT NULL,
            embedding vector({EMBEDDING_DIM}),
            metadata JSONB DEFAULT '{{}}'::jsonb,
            created_at TIMESTAMP DEFAULT NOW()
        );
    """)
    # IVFFlat index for fast approximate nearest-neighbour search
    cur.execute(f"""
        CREATE INDEX IF NOT EXISTS documents_embedding_idx
        ON documents USING ivfflat (embedding vector_cosine_ops)
        WITH (lists = 10);
    """)
    cur.close()
    print(f"RAG app ready. Model: {MODEL_NAME}, DB: connected, Embedding dim: {EMBEDDING_DIM}")
    yield


# ---------------------------------------------------------------------------
# FastAPI application
# ---------------------------------------------------------------------------
app = FastAPI(title="kube-llmops RAG Example", lifespan=lifespan)


# -- Request / Response models -----------------------------------------------
class IngestRequest(BaseModel):
    content: str
    metadata: dict = {}


class QueryRequest(BaseModel):
    question: str
    top_k: int = 3


class QueryResponse(BaseModel):
    answer: str
    sources: list
    trace_url: str = ""


# -- Endpoints ----------------------------------------------------------------
@app.post("/ingest")
@observe()
def ingest_document(req: IngestRequest):
    """Ingest a document: embed + store in pgvector."""
    doc_id = hashlib.md5(req.content.encode()).hexdigest()
    embedding = embed_text(req.content)

    db = get_db()
    cur = db.cursor()
    cur.execute(
        "INSERT INTO documents (id, content, embedding, metadata) "
        "VALUES (%s, %s, %s, %s) ON CONFLICT (id) DO NOTHING",
        (doc_id, req.content, embedding, json.dumps(req.metadata)),
    )
    cur.close()

    return {"id": doc_id, "status": "ingested", "embedding_dim": len(embedding)}


@app.post("/query", response_model=QueryResponse)
@observe()
def query_rag(req: QueryRequest):
    """RAG query: embed question → search pgvector → generate answer via LLM."""
    # 1. Embed the question
    q_embedding = embed_text(req.question)

    # 2. Search pgvector for similar documents
    db = get_db()
    cur = db.cursor()
    cur.execute(
        "SELECT content, 1 - (embedding <=> %s::vector) AS similarity "
        "FROM documents ORDER BY embedding <=> %s::vector LIMIT %s",
        (q_embedding, q_embedding, req.top_k),
    )
    results = cur.fetchall()
    cur.close()

    if not results:
        raise HTTPException(status_code=404, detail="No documents found. Ingest some first.")

    # 3. Build context from retrieved documents
    context = "\n---\n".join(
        [f"[Relevance: {sim:.2f}] {content}" for content, sim in results]
    )

    # 4. Generate answer via LiteLLM
    response = llm.chat.completions.create(
        model=MODEL_NAME,
        messages=[
            {
                "role": "system",
                "content": (
                    "Answer based on the following context. "
                    "If the context doesn't contain the answer, say so.\n\n"
                    f"Context:\n{context}"
                ),
            },
            {"role": "user", "content": req.question},
        ],
        max_tokens=200,
    )

    answer = response.choices[0].message.content

    # 5. Get Langfuse trace URL for observability
    langfuse_context.flush()
    trace_id = langfuse_context.get_current_trace_id()
    trace_url = f"{LANGFUSE_HOST}/trace/{trace_id}" if trace_id else ""

    return QueryResponse(
        answer=answer,
        sources=[{"content": c[:100], "similarity": round(s, 3)} for c, s in results],
        trace_url=trace_url,
    )


@app.get("/health")
def health():
    """Simple health-check endpoint."""
    return {"status": "ok", "model": MODEL_NAME}


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
