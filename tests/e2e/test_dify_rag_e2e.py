"""
Dify RAG E2E test: Create Knowledge Base → Upload Doc → Chat with context → Verify RAG answer
Uses Playwright headless browser + Dify 1.x console API.
"""
# /// script
# requires-python = ">=3.12"
# dependencies = ["playwright"]
# ///

import json
import os
import sys
import time
from playwright.sync_api import sync_playwright

DIFY_URL = os.environ.get("DIFY_URL", "http://dify.llmops.local")
DIFY_API_URL = os.environ.get("DIFY_API_URL", DIFY_URL)
ADMIN_EMAIL = "admin@kube-llmops.local"
ADMIN_PASSWORD = "Admin123!"
PROVIDER = "langgenius/openai_api_compatible/openai_api_compatible"
LLM_MODEL = os.environ.get("LLM_MODEL", "gemma-4-26b-a4b")

TEST_DOC = """kube-llmops is a Kubernetes-native LLMOps platform created in 2024.
It uses Umbrella Helm Charts and runs on k3s with NVIDIA GPU support.
Key components: LiteLLM (AI Gateway, port 4000), Langfuse v3 (observability),
vLLM (model serving with PagedAttention), TEI (embedding/reranking),
Dify (RAG platform), pgvector (vector storage), MinIO (S3 storage).
The default embedding model is bge-small-en-v1.5 with 384 dimensions.
The default LLM is Qwen2.5-0.5B-Instruct served by vLLM.
GPU memory utilization is set to 80 percent."""


def api(page, csrf, method, path, payload=None, timeout=60000):
    url = f"{DIFY_API_URL}{path}"
    body_js = f", body: JSON.stringify({json.dumps(payload)})" if payload else ""
    ct = "'Content-Type': 'application/json'," if payload else ""
    return page.evaluate(f"""async () => {{
        try {{
            const r = await fetch('{url}', {{
                method: '{method}',
                headers: {{ {ct} 'X-CSRF-Token': '{csrf}' }},
                credentials: 'include'{body_js},
                signal: AbortSignal.timeout({timeout})
            }});
            return {{ status: r.status, body: await r.text() }};
        }} catch(e) {{ return {{ status: 0, body: e.message }}; }}
    }}""")


def upload_text_file(page, csrf, filename, content):
    """Upload a text file via /console/api/files/upload."""
    url = f"{DIFY_API_URL}/console/api/files/upload"
    return page.evaluate(f"""async () => {{
        const blob = new Blob([{json.dumps(content)}], {{ type: 'text/plain' }});
        const file = new File([blob], '{filename}', {{ type: 'text/plain' }});
        const fd = new FormData();
        fd.append('file', file);
        fd.append('source', 'datasets');
        const r = await fetch('{url}', {{
            method: 'POST',
            headers: {{ 'X-CSRF-Token': '{csrf}' }},
            credentials: 'include',
            body: fd
        }});
        return {{ status: r.status, body: await r.text() }};
    }}""")


def test_rag():
    p_count = [0, 0]  # [passed, failed]

    def ok(name, cond, detail=""):
        p_count[0 if cond else 1] += 1
        print(f"  {'PASS' if cond else 'FAIL'}: {name}" + (f" | {detail}" if not cond and detail else ""))
        return cond

    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True)
        page = browser.new_context(ignore_https_errors=True).new_page()
        page.set_default_timeout(60000)

        # 1. Login
        print("[1/7] Login")
        page.goto(f"{DIFY_URL}/signin")
        page.wait_for_load_state("networkidle")
        page.fill('input[type="email"]', ADMIN_EMAIL)
        page.fill('input[type="password"]', ADMIN_PASSWORD)
        page.get_by_role("button", name="Sign in").click()
        page.wait_for_url("**/apps**", timeout=15000)
        ctx = page.context
        csrf = next(c["value"] for c in ctx.cookies() if c["name"] == "csrf_token")
        ok("Login", bool(csrf))

        # 2. Set default models
        print("[2/7] Set default models")
        r = api(page, csrf, "POST", "/console/api/workspaces/current/default-model", {
            "model_settings": [
                {"model_type": "text-embedding", "provider": PROVIDER, "model": "bge-small-en"},
                {"model_type": "llm", "provider": PROVIDER, "model": LLM_MODEL},
            ]
        })
        ok("Default models", r["status"] == 200, r["body"][:100])

        # 3. Create knowledge base
        print("[3/7] Create knowledge base")
        ds_name = f"rag-e2e-{int(time.time())}"
        r = api(page, csrf, "POST", "/console/api/datasets", {
            "name": ds_name, "indexing_technique": "high_quality", "permission": "only_me",
        })
        ds = json.loads(r["body"]) if r["status"] in (200, 201) else {}
        ds_id = ds.get("id", "")
        ok("Create KB", bool(ds_id), r["body"][:120])
        if not ds_id:
            browser.close()
            return False

        # 4. Upload file → create document
        print("[4/7] Upload document")
        # 4a. Upload file
        fu = upload_text_file(page, csrf, "kube-llmops-facts.txt", TEST_DOC)
        fu_data = json.loads(fu["body"]) if fu["status"] in (200, 201) else {}
        file_id = fu_data.get("id", "")
        ok("File upload", bool(file_id), fu["body"][:120])

        if file_id:
            # 4b. Create document from uploaded file
            doc_r = api(page, csrf, "POST", f"/console/api/datasets/{ds_id}/documents", {
                "data_source": {
                    "info_list": {
                        "data_source_type": "upload_file",
                        "file_info_list": {"file_ids": [file_id]},
                    }
                },
                "indexing_technique": "high_quality",
                "process_rule": {"mode": "automatic"},
                "doc_form": "text_model",
                "doc_language": "English",
            })
            doc_data = json.loads(doc_r["body"]) if doc_r["status"] in (200, 201) else {}
            batch = doc_data.get("batch", "")
            ok("Create document", bool(batch) or doc_r["status"] in (200, 201), doc_r["body"][:150])
        else:
            ok("Create document", False, "no file_id")

        # 5. Wait for indexing
        print("[5/7] Wait for indexing")
        indexed = False
        for i in range(60):
            time.sleep(5)
            dr = api(page, csrf, "GET", f"/console/api/datasets/{ds_id}/documents?page=1&limit=10")
            if dr["status"] == 200:
                docs = json.loads(dr["body"]).get("data", [])
                if docs:
                    st = docs[0].get("indexing_status", "")
                    wc = docs[0].get("word_count", 0)
                    if i % 3 == 0:
                        print(f"    [{i*5}s] status={st} words={wc}")
                    if st == "completed":
                        indexed = True
                        break
                    if st in ("error", "paused"):
                        print(f"    Indexing error: {docs[0].get('error','')}")
                        break
        ok("Indexing complete", indexed)

        # 6. Create chatbot app
        print("[6/7] Create chatbot app")
        app_r = api(page, csrf, "POST", "/console/api/apps", {
            "name": f"rag-test-{int(time.time())}",
            "mode": "chat",
            "icon_type": "emoji", "icon": "🤖", "icon_background": "#FFEAD5",
        })
        app = json.loads(app_r["body"]) if app_r["status"] in (200, 201) else {}
        app_id = app.get("id", "")
        ok("Create app", bool(app_id), app_r["body"][:100])

        # 7. Chat with RAG context
        if app_id and indexed:
            print("[7/7] Chat with RAG")
            # Build model_config with dataset context
            model_config = {
                "pre_prompt": "Answer based on the knowledge base context. Be concise.",
                "model": {
                    "provider": PROVIDER,
                    "name": LLM_MODEL,
                    "mode": "chat",
                    "completion_params": {"temperature": 0.2, "max_tokens": 256},
                },
                "dataset_configs": {
                    "retrieval_model": "single",
                    "datasets": {
                        "datasets": [{"dataset": {"enabled": True, "id": ds_id}}]
                    },
                },
                "user_input_form": [],
                "agent_mode": {"enabled": False},
                "opening_statement": "",
                "suggested_questions": [],
                "more_like_this": {"enabled": False},
                "sensitive_word_avoidance": {"enabled": False},
                "speech_to_text": {"enabled": False},
                "text_to_speech": {"enabled": False},
                "retriever_resource": {"enabled": True},
                "file_upload": {"enabled": False},
                "annotation_reply": {"enabled": False},
            }

            chat_r = api(page, csrf, "POST", f"/console/api/apps/{app_id}/chat-messages", {
                "inputs": {},
                "query": "What is the default embedding model in kube-llmops and how many dimensions does it have?",
                "model_config": model_config,
                "response_mode": "blocking",
            }, timeout=120000)

            if chat_r["status"] == 200:
                ans = json.loads(chat_r["body"]).get("answer", "")
                print(f"    Answer: {ans[:200]}")
                ok("RAG answer has model name", "bge" in ans.lower() or "small" in ans.lower(), ans[:80])
                ok("RAG answer has dimensions", "384" in ans, ans[:80])
            else:
                print(f"    Error: {chat_r['status']} {chat_r['body'][:200]}")
                ok("Chat API", False, f"status={chat_r['status']}")
                ok("RAG quality", False)
        else:
            print("[7/7] Skipped (missing app or indexing failed)")
            ok("Chat", False, "skipped")
            ok("RAG", False, "skipped")

        browser.close()

    total = p_count[0] + p_count[1]
    print(f"\n{'='*50}")
    print(f"Results: {p_count[0]} passed, {p_count[1]} failed / {total}")
    print("=== ALL RAG E2E TESTS PASSED ===" if p_count[1] == 0 else "=== SOME TESTS FAILED ===")
    return p_count[1] == 0


if __name__ == "__main__":
    sys.exit(0 if test_rag() else 1)
