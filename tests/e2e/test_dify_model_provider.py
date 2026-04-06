"""
Dify E2E test: Login → Add Embedding + LLM Model Provider → Verify
Uses Playwright headless browser to automate Dify UI.
"""
# /// script
# requires-python = ">=3.12"
# dependencies = ["playwright"]
# ///

import json
import os
import sys
from playwright.sync_api import sync_playwright

DIFY_URL = os.environ.get("DIFY_URL", "http://dify.llmops.local")
DIFY_API_URL = os.environ.get("DIFY_API_URL", DIFY_URL)
ADMIN_EMAIL = "admin@kube-llmops.local"
ADMIN_PASSWORD = "Admin123!"
LITELLM_API_BASE = "http://kube-llmops-litellm:4000/v1"
LITELLM_API_KEY = "sk-kube-llmops-dev"
LLM_MODEL = os.environ.get("LLM_MODEL", "gemma-4-26b-a4b")


def api_call(page, csrf, method, path, payload=None):
    """Make an authenticated API call via the browser's fetch."""
    url = f"{DIFY_API_URL}{path}"
    body_js = f", body: JSON.stringify({json.dumps(payload)})" if payload else ""
    ct = "'Content-Type': 'application/json'," if payload else ""
    return page.evaluate(f"""async () => {{
        const r = await fetch('{url}', {{
            method: '{method}',
            headers: {{ {ct} 'X-CSRF-Token': '{csrf}' }},
            credentials: 'include'{body_js}
        }});
        return {{ status: r.status, body: await r.text() }};
    }}""")


def test_dify_model_providers():
    passed = 0
    failed = 0

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(ignore_https_errors=True)
        page = context.new_page()
        page.set_default_timeout(30000)

        # ── Step 1: Login ──
        print("[1/5] Logging in...")
        page.goto(f"{DIFY_URL}/signin")
        page.wait_for_load_state("networkidle")
        page.fill('input[type="email"]', ADMIN_EMAIL)
        page.fill('input[type="password"]', ADMIN_PASSWORD)
        page.get_by_role("button", name="Sign in").click()
        page.wait_for_url("**/apps**", timeout=15000)
        print("  PASS: Login OK")
        passed += 1

        csrf = next((c["value"] for c in context.cookies() if c["name"] == "csrf_token"), None)
        assert csrf, "No CSRF token"

        creds_url = "/console/api/workspaces/current/model-providers/openai_api_compatible/models/credentials"

        # ── Step 2: Add embedding model ──
        print("[2/5] Adding embedding model (bge-small-en)...")
        resp = api_call(page, csrf, "POST", creds_url, {
            "model": "bge-small-en",
            "model_type": "text-embedding",
            "credentials": {
                "api_key": LITELLM_API_KEY,
                "endpoint_url": LITELLM_API_BASE,
                "mode": "embedding",
                "context_size": "512",
            },
        })
        s, b = resp["status"], resp["body"]
        print(f"  Response: {s} {b[:150]}")
        if s in (200, 201):
            print("  PASS: Embedding model added")
            passed += 1
        elif "already" in b.lower():
            print("  PASS: Embedding model already configured")
            passed += 1
        else:
            print(f"  FAIL: Unexpected {s}")
            failed += 1

        # ── Step 3: Add LLM model ──
        print(f"[3/5] Adding LLM model ({LLM_MODEL})...")
        resp2 = api_call(page, csrf, "POST", creds_url, {
            "model": LLM_MODEL,
            "model_type": "llm",
            "credentials": {
                "api_key": LITELLM_API_KEY,
                "endpoint_url": LITELLM_API_BASE,
                "mode": "chat",
                "context_size": "4096",
            },
        })
        s2, b2 = resp2["status"], resp2["body"]
        print(f"  Response: {s2} {b2[:150]}")
        if s2 in (200, 201):
            print("  PASS: LLM model added")
            passed += 1
        elif "already" in b2.lower():
            print("  PASS: LLM model already configured")
            passed += 1
        else:
            print(f"  FAIL: Unexpected {s2}")
            failed += 1

        # ── Step 4: Verify embedding models visible ──
        print("[4/5] Verifying embedding models...")
        vr = api_call(page, csrf, "GET", "/console/api/workspaces/current/models/model-types/text-embedding")
        if vr["status"] == 200:
            providers = json.loads(vr["body"]).get("data", [])
            all_models = [m["model"] for p in providers for m in p.get("models", [])]
            print(f"  Embedding models: {all_models}")
            if any("bge-small" in m for m in all_models):
                print("  PASS: bge-small-en found")
                passed += 1
            else:
                print("  FAIL: bge-small-en not found in model list")
                failed += 1
        else:
            print(f"  FAIL: API returned {vr['status']}")
            failed += 1

        # ── Step 5: Verify LLM models visible ──
        print("[5/5] Verifying LLM models...")
        vl = api_call(page, csrf, "GET", "/console/api/workspaces/current/models/model-types/llm")
        if vl["status"] == 200:
            providers = json.loads(vl["body"]).get("data", [])
            all_models = [m["model"] for p in providers for m in p.get("models", [])]
            print(f"  LLM models: {all_models}")
            if any(LLM_MODEL in m for m in all_models):
                print(f"  PASS: {LLM_MODEL} found")
                passed += 1
            else:
                print(f"  FAIL: {LLM_MODEL} not found in model list")
                failed += 1
        else:
            print(f"  FAIL: API returned {vl['status']}")
            failed += 1

        browser.close()

    print(f"\n{'='*40}")
    print(f"Results: {passed} passed, {failed} failed")
    if failed == 0:
        print("=== ALL TESTS PASSED ===")
    else:
        print("=== SOME TESTS FAILED ===")
    return failed == 0


if __name__ == "__main__":
    success = test_dify_model_providers()
    sys.exit(0 if success else 1)
