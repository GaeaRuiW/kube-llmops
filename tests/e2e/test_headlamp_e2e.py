# /// script
# dependencies = ["playwright"]
# ///
"""Full Headlamp + Plugin E2E test.

Supports both OIDC (Keycloak) and token authentication.
When OIDC is configured, Headlamp shows a login page with 'SIGN IN' (OIDC)
and 'USE A TOKEN' buttons. This test uses token auth as fallback.
"""
import subprocess, sys, os
from playwright.sync_api import sync_playwright

BASE = os.environ.get("HEADLAMP_URL", "http://172.29.193.187:30302")
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

def login(page, token):
    """Authenticate via token (works with or without OIDC configured)."""
    page.goto(BASE, wait_until="networkidle", timeout=30000)
    page.wait_for_timeout(2000)
    title = page.title()

    if title == "Login":
        # OIDC is configured — click 'USE A TOKEN' to switch to token auth
        token_btn = page.query_selector('button:has-text("USE A TOKEN")')
        if token_btn:
            token_btn.click()
            page.wait_for_timeout(1000)

    # Fill token and authenticate
    token_input = page.query_selector('input')
    if not token_input:
        return False
    token_input.fill(token)
    auth_btn = page.query_selector(
        'button:has-text("Authenticate"), button[type="submit"]'
    )
    if auth_btn:
        auth_btn.click()
    page.wait_for_load_state("networkidle", timeout=15000)
    page.wait_for_timeout(3000)
    return "/c/main" in page.url and "login" not in page.url

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
        title = page.title()
        check("Login page shown", title in ("Token", "Login"))

        # --- Test 2: Token auth works ---
        print("\n[2] Token authentication")
        logged_in = login(page, token)
        check("Logged in successfully", logged_in)
        check("Redirected to cluster page", "/c/main" in page.url)
        check("Title is 'Cluster'", page.title() == "Cluster")

        if not logged_in:
            print("\n  ABORT: Cannot proceed without authentication")
            browser.close()
            return 1

        # --- Test 3: Plugin sidebar registered ---
        print("\n[3] Plugin sidebar")
        body = page.inner_text("body")
        check("'LLMOps' in sidebar", "LLMOps" in body)
        links = page.query_selector_all('a[href*="kube-llmops/services"]')
        check("Service Links sidebar link exists", len(links) > 0)
        links2 = page.query_selector_all('a[href*="kube-llmops/monitoring"]')
        check("Monitoring sidebar link exists", len(links2) > 0)

        # --- Test 4: Click LLMOps opens Service Links ---
        print("\n[4] Click LLMOps sidebar")
        llmops = page.query_selector('span:text-is("LLMOps")')
        check("LLMOps sidebar entry found", llmops is not None)
        if llmops:
            llmops.click()
            page.wait_for_timeout(3000)
            check("Navigated to services page", "/kube-llmops/services" in page.url)
            # Sidebar should auto-expand children
            svc_link = page.query_selector('span:text-is("Service Links")')
            mon_link = page.query_selector('span:text-is("Monitoring")')
            svc_visible = svc_link.bounding_box() is not None if svc_link else False
            mon_visible = mon_link.bounding_box() is not None if mon_link else False
            check("'Service Links' sidebar visible", svc_visible)
            check("'Monitoring' sidebar visible", mon_visible)

        # --- Test 5: Service Links page ---
        print("\n[5] Service Links page")
        page.goto(f"{BASE}/c/main/kube-llmops/services", wait_until="networkidle", timeout=15000)
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        check("Page title 'llmops-services'", page.title() == "llmops-services")
        for svc in ["Grafana", "LiteLLM", "Langfuse", "Dify", "Keycloak", "Prometheus", "MinIO"]:
            check(f"'{svc}' card visible", svc in body)

        # --- Test 6: Monitoring page ---
        print("\n[6] Monitoring page")
        page.goto(f"{BASE}/c/main/kube-llmops/monitoring", wait_until="networkidle", timeout=15000)
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        check("Page title 'llmops-monitoring'", page.title() == "llmops-monitoring")
        for tab in ["vLLM", "LiteLLM Gateway", "System", "GPU", "SLO", "Cost"]:
            check(f"'{tab}' tab visible", tab in body)

        # --- Test 7: Grafana iframe ---
        print("\n[7] Grafana iframe")
        iframes = page.query_selector_all("iframe")
        check("iframe present", len(iframes) > 0)
        if iframes:
            src = iframes[0].get_attribute("src")
            check("iframe src contains grafana:30300", ":30300" in src)
            check("iframe src contains kiosk mode", "kiosk" in src)
            check("iframe src contains vllm-overview", "vllm-overview" in src)
            print(f"  iframe src: {src}")

        # --- Test 8: K8s native pages still work ---
        print("\n[8] K8s native pages")
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
