"""
Screenshot capture for E2E test report.
Navigates through all key UI pages, waits for data to load, takes high-quality screenshots.
"""
# /// script
# requires-python = ">=3.12"
# dependencies = ["playwright"]
# ///

import json, sys, time, os
from playwright.sync_api import sync_playwright

DIFY_URL = "http://dify.llmops.local"
GRAFANA_URL = "http://grafana.llmops.local"
LANGFUSE_URL = "http://langfuse.llmops.local"
SHOTS = "tests/e2e/screenshots"
os.makedirs(SHOTS, exist_ok=True)

def shot(page, name, desc):
    path = f"{SHOTS}/{name}.png"
    page.screenshot(path=path, full_page=False)
    print(f"  [{name}] {desc} — {os.path.getsize(path)//1024}KB")

with sync_playwright() as pw:
    browser = pw.chromium.launch(headless=True)
    ctx = browser.new_context(ignore_https_errors=True, viewport={"width": 1440, "height": 900})
    page = ctx.new_page()
    page.set_default_timeout(60000)

    # ═══════════════════════════════════════
    # DIFY
    # ═══════════════════════════════════════
    print("=== Dify Screenshots ===")

    # 1. Login page
    page.goto(f"{DIFY_URL}/signin")
    page.wait_for_load_state("networkidle")
    time.sleep(2)
    shot(page, "01-dify-signin", "Dify sign-in page")

    # 2. Login
    page.fill('input[type="email"]', "admin@kube-llmops.local")
    page.fill('input[type="password"]', "Admin123!")
    page.get_by_role("button", name="Sign in").click()
    page.wait_for_url("**/apps**", timeout=15000)
    time.sleep(3)
    shot(page, "02-dify-apps", "Dify apps dashboard after login")

    csrf = next(c["value"] for c in ctx.cookies() if c["name"] == "csrf_token")

    # 3. Navigate to model provider settings
    page.goto(f"{DIFY_URL}/apps")
    time.sleep(2)
    # Go to settings page via URL
    page.goto(f"{DIFY_URL}/settings/model-provider")
    page.wait_for_load_state("networkidle")
    time.sleep(3)
    shot(page, "03-dify-model-providers", "Dify model provider settings")

    # 4. Knowledge base list
    page.goto(f"{DIFY_URL}/datasets")
    page.wait_for_load_state("networkidle")
    time.sleep(3)
    shot(page, "04-dify-knowledge-bases", "Dify knowledge base list")

    # 5. If there's a knowledge base, click into it
    try:
        # Get the first dataset via API
        r = page.evaluate(f"""async () => {{
            const r = await fetch('{DIFY_URL}/console/api/datasets?page=1&limit=10', {{
                headers: {{ 'X-CSRF-Token': '{csrf}' }}, credentials: 'include'
            }});
            return await r.json();
        }}""")
        datasets = r.get("data", [])
        if datasets:
            ds_id = datasets[0]["id"]
            page.goto(f"{DIFY_URL}/datasets/{ds_id}/documents")
            page.wait_for_load_state("networkidle")
            time.sleep(3)
            shot(page, "05-dify-kb-documents", f"Knowledge base documents ({datasets[0]['name']})")
    except Exception as e:
        print(f"  [SKIP] KB documents: {e}")

    # 6. Create a test conversation to show RAG in action
    # Find an app
    try:
        apps_r = page.evaluate(f"""async () => {{
            const r = await fetch('{DIFY_URL}/console/api/apps?page=1&limit=10', {{
                headers: {{ 'X-CSRF-Token': '{csrf}' }}, credentials: 'include'
            }});
            return await r.json();
        }}""")
        apps = apps_r.get("data", [])
        if apps:
            app_id = apps[0]["id"]
            page.goto(f"{DIFY_URL}/app/{app_id}/overview")
            page.wait_for_load_state("networkidle")
            time.sleep(3)
            shot(page, "06-dify-app-overview", f"App overview ({apps[0]['name']})")
    except Exception as e:
        print(f"  [SKIP] App overview: {e}")

    # ═══════════════════════════════════════
    # GRAFANA
    # ═══════════════════════════════════════
    print("\n=== Grafana Screenshots ===")

    page.goto(f"{GRAFANA_URL}/login")
    page.wait_for_load_state("networkidle")
    time.sleep(2)

    # Login
    page.fill('input[name="user"]', "admin")
    page.fill('input[name="password"]', "admin123!")
    page.click('button[type="submit"]')
    time.sleep(3)

    # Skip password change prompt if shown
    try:
        skip_btn = page.locator('a:has-text("Skip"), button:has-text("Skip")')
        if skip_btn.count() > 0:
            skip_btn.first.click()
            time.sleep(2)
    except:
        pass

    shot(page, "07-grafana-home", "Grafana home after login")

    # 8. RAG Quality dashboard
    page.goto(f"{GRAFANA_URL}/d/rag-quality/rag-quality-ragas-metrics?orgId=1&from=now-1h&to=now")
    time.sleep(5)
    shot(page, "08-grafana-rag-quality", "RAG Quality Ragas Metrics dashboard")

    # 9. Try other dashboards
    page.goto(f"{GRAFANA_URL}/api/search?type=dash-db")
    time.sleep(2)
    try:
        dashboards = json.loads(page.locator("body").inner_text())
        print(f"  Found {len(dashboards)} dashboards:")
        for d in dashboards:
            print(f"    - {d.get('title','?')} (uid={d.get('uid','?')})")
            if d.get("uid") not in ("rag-quality",):
                page.goto(f"{GRAFANA_URL}{d.get('url', '')}")
                time.sleep(4)
                uid = d.get('uid', 'unknown')
                shot(page, f"09-grafana-{uid}", f"Dashboard: {d.get('title','?')}")
    except Exception as e:
        print(f"  [SKIP] Dashboard list: {e}")

    # 10. Prometheus alerts in Grafana
    page.goto(f"{GRAFANA_URL}/alerting/list")
    time.sleep(4)
    shot(page, "10-grafana-alerts", "Grafana alerting rules")

    # ═══════════════════════════════════════
    # LANGFUSE
    # ═══════════════════════════════════════
    print("\n=== Langfuse Screenshots ===")

    page.goto(f"{LANGFUSE_URL}")
    page.wait_for_load_state("networkidle")
    time.sleep(3)

    # Try to login if needed
    try:
        email_input = page.locator('input[type="email"], input[name="email"]')
        if email_input.count() > 0:
            email_input.fill("admin@kube-llmops.local")
            page.fill('input[type="password"]', "admin")
            page.click('button[type="submit"]')
            time.sleep(3)
    except:
        pass

    shot(page, "11-langfuse-home", "Langfuse dashboard")

    # Navigate to traces
    try:
        page.goto(f"{LANGFUSE_URL}/project/kube-llmops/traces")
        time.sleep(4)
        shot(page, "12-langfuse-traces", "Langfuse traces list")
    except Exception as e:
        # Try different URL patterns
        try:
            page.goto(f"{LANGFUSE_URL}")
            time.sleep(2)
            # Click on Traces in navigation
            traces_link = page.locator('a:has-text("Traces"), [href*="traces"]')
            if traces_link.count() > 0:
                traces_link.first.click()
                time.sleep(4)
                shot(page, "12-langfuse-traces", "Langfuse traces list")
        except:
            print(f"  [SKIP] Traces: {e}")

    browser.close()

print(f"\n=== Done: {len(os.listdir(SHOTS))} files in {SHOTS}/ ===")
for f in sorted(os.listdir(SHOTS)):
    if f.endswith(".png"):
        size = os.path.getsize(f"{SHOTS}/{f}") // 1024
        print(f"  {f} ({size}KB)")
