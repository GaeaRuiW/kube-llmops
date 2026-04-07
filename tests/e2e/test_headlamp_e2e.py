# /// script
# dependencies = ["playwright"]
# ///
"""Full Headlamp + Plugin E2E test."""
import subprocess, sys
from playwright.sync_api import sync_playwright

BASE = "http://172.29.193.187:30302"
PASS = 0
FAIL = 0

def get_token():
    result = subprocess.run(
        ["kubectl", "create", "token", "kube-llmops-headlamp", "--duration=87600h"],
        capture_output=True, text=True
    )
    return result.stdout.strip()

def check(name, condition):
    global PASS, FAIL
    if condition:
        PASS += 1
        print(f"  PASS: {name}")
    else:
        FAIL += 1
        print(f"  FAIL: {name}")

def main():
    global PASS, FAIL
    token = get_token()
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        # --- Test 1: Headlamp loads ---
        print("[1] Headlamp loads")
        resp = page.goto(BASE, wait_until="networkidle", timeout=30000)
        check("HTTP 200", resp.status == 200)
        check("Title is 'Token'", page.title() == "Token")

        # --- Test 2: Token auth works ---
        print("\n[2] Token authentication")
        token_input = page.query_selector('input')
        token_input.fill(token)
        page.query_selector('button:has-text("Authenticate"), button[type="submit"]').click()
        page.wait_for_load_state("networkidle", timeout=15000)
        page.wait_for_timeout(3000)
        check("Redirected to cluster page", "/c/main" in page.url)
        check("Title is 'Cluster'", page.title() == "Cluster")

        # --- Test 3: Plugin sidebar registered ---
        print("\n[3] Plugin sidebar")
        body = page.inner_text("body")
        check("'LLMOps' in sidebar", "LLMOps" in body)
        links = page.query_selector_all('a[href*="kube-llmops/services"]')
        check("Service Links sidebar link exists", len(links) > 0)
        links2 = page.query_selector_all('a[href*="kube-llmops/monitoring"]')
        check("Monitoring sidebar link exists", len(links2) > 0)

        # --- Test 4: Service Links page ---
        print("\n[4] Service Links page")
        page.goto(f"{BASE}/c/main/kube-llmops/services", wait_until="networkidle", timeout=15000)
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        check("Page title 'llmops-services'", page.title() == "llmops-services")
        for svc in ["Grafana", "LiteLLM", "Langfuse", "Dify", "Keycloak", "Prometheus", "MinIO"]:
            check(f"'{svc}' card visible", svc in body)

        # --- Test 5: Monitoring page ---
        print("\n[5] Monitoring page")
        page.goto(f"{BASE}/c/main/kube-llmops/monitoring", wait_until="networkidle", timeout=15000)
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        check("Page title 'llmops-monitoring'", page.title() == "llmops-monitoring")
        for tab in ["vLLM", "LiteLLM Gateway", "System", "GPU", "SLO", "Cost"]:
            check(f"'{tab}' tab visible", tab in body)

        # --- Test 6: Grafana iframe ---
        print("\n[6] Grafana iframe")
        iframes = page.query_selector_all("iframe")
        check("iframe present", len(iframes) > 0)
        if iframes:
            src = iframes[0].get_attribute("src")
            check("iframe src contains grafana:30300", ":30300" in src)
            check("iframe src contains kiosk mode", "kiosk" in src)
            check("iframe src contains vllm-overview", "vllm-overview" in src)
            print(f"  iframe src: {src}")

        # --- Test 7: K8s native pages still work ---
        print("\n[7] K8s native pages")
        page.goto(f"{BASE}/c/main/namespaces", wait_until="networkidle", timeout=15000)
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        check("Namespaces page loads", "default" in body or "kube-system" in body)

        print(f"\n{'='*50}")
        print(f"Results: {PASS} passed, {FAIL} failed")
        print(f"{'='*50}")

        browser.close()
        return 1 if FAIL > 0 else 0

sys.exit(main())
