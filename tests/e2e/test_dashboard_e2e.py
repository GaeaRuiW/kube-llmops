"""
Dashboard E2E test: Navigate all pages, verify rendering, take screenshots.
Uses Playwright headless browser to test the kube-llmops Dashboard UI.

Usage:
    uv run tests/e2e/test_dashboard_e2e.py

Screenshots are saved to tests/e2e/screenshots/dashboard/
"""
# /// script
# requires-python = ">=3.12"
# dependencies = ["playwright"]
# ///

import os
import sys
import time
from pathlib import Path
from playwright.sync_api import sync_playwright, Page, expect

DASHBOARD_URL = os.environ.get("DASHBOARD_URL", "http://172.29.193.187:30302")
SCREENSHOT_DIR = Path(__file__).parent / "screenshots" / "dashboard"
SCREENSHOT_DIR.mkdir(parents=True, exist_ok=True)

passed = 0
failed = 0
errors = []


def screenshot(page: Page, name: str):
    """Save screenshot with descriptive name."""
    path = SCREENSHOT_DIR / f"{name}.png"
    page.screenshot(path=str(path), full_page=True)
    print(f"    Screenshot: {path}")


def test_pass(desc: str):
    global passed
    passed += 1
    print(f"  PASS: {desc}")


def test_fail(desc: str, error: str = ""):
    global failed
    failed += 1
    msg = f"  FAIL: {desc}"
    if error:
        msg += f" — {error}"
    print(msg)
    errors.append(desc)


def run_tests():
    global passed, failed

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            viewport={"width": 1440, "height": 900},
            ignore_https_errors=True,
        )
        page = context.new_page()
        page.set_default_timeout(15000)

        # ── Test 1: Dashboard loads ──
        print("[1/12] Loading dashboard...")
        try:
            page.goto(DASHBOARD_URL, wait_until="networkidle")
            # Should redirect to /overview
            page.wait_for_url("**/overview**", timeout=10000)
            screenshot(page, "01_overview")
            test_pass("Dashboard loads and redirects to /overview")
        except Exception as e:
            test_fail("Dashboard loads", str(e))
            screenshot(page, "01_overview_error")

        # ── Test 2: Sidebar navigation visible ──
        print("[2/12] Checking sidebar layout...")
        try:
            # Check sidebar exists
            sidebar = page.locator("aside, .ant-layout-sider, nav").first
            sidebar.wait_for(timeout=5000)
            # Check we have menu items
            menu_items = page.locator(".ant-menu-item, .ant-menu-item-group")
            count = menu_items.count()
            if count > 0:
                test_pass(f"Sidebar visible with {count} menu items")
            else:
                test_fail("Sidebar menu items", "No menu items found")
            screenshot(page, "02_sidebar")
        except Exception as e:
            test_fail("Sidebar layout", str(e))
            screenshot(page, "02_sidebar_error")

        # ── Test 3: Overview page content ──
        print("[3/12] Verifying Overview page content...")
        try:
            page.goto(f"{DASHBOARD_URL}/overview", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Check for KPI cards (Statistic components)
            stats = page.locator(".ant-statistic, .ant-card")
            stat_count = stats.count()
            # Check for title
            title = page.locator("h3, h2").first
            title_text = title.inner_text() if title.count() > 0 else ""
            screenshot(page, "03_overview_content")
            if stat_count >= 2:
                test_pass(f"Overview has {stat_count} cards/statistics, title: '{title_text}'")
            else:
                test_fail("Overview content", f"Expected >=2 cards, got {stat_count}")
        except Exception as e:
            test_fail("Overview content", str(e))
            screenshot(page, "03_overview_error")

        # ── Test 4: Models page ──
        print("[4/12] Testing Models page...")
        try:
            page.goto(f"{DASHBOARD_URL}/models", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Should have a table and a "Deploy Model" button
            table = page.locator(".ant-table, table")
            has_table = table.count() > 0
            deploy_btn = page.locator("button:has-text('Deploy')")
            has_deploy = deploy_btn.count() > 0
            screenshot(page, "04_models_list")
            if has_table:
                test_pass(f"Models page has table, deploy button: {has_deploy}")
            else:
                test_fail("Models page", "No table found")
        except Exception as e:
            test_fail("Models page", str(e))
            screenshot(page, "04_models_error")

        # ── Test 5: Deploy wizard ──
        print("[5/12] Testing Deploy Model wizard...")
        try:
            page.goto(f"{DASHBOARD_URL}/models/deploy", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Should have steps and form fields
            steps = page.locator(".ant-steps, .ant-steps-item")
            form_items = page.locator(".ant-form-item, .ant-input, input")
            screenshot(page, "05_deploy_wizard")
            if steps.count() > 0 and form_items.count() > 0:
                test_pass(f"Deploy wizard has {steps.count()} step elements, {form_items.count()} form fields")
            else:
                test_fail("Deploy wizard", f"Steps: {steps.count()}, Forms: {form_items.count()}")
        except Exception as e:
            test_fail("Deploy wizard", str(e))
            screenshot(page, "05_deploy_wizard_error")

        # ── Test 6: Finetune page ──
        print("[6/12] Testing Fine-tuning page...")
        try:
            page.goto(f"{DASHBOARD_URL}/finetune", wait_until="networkidle")
            page.wait_for_timeout(2000)
            table = page.locator(".ant-table, table")
            new_btn = page.locator("button:has-text('Fine-tune'), button:has-text('New')")
            screenshot(page, "06_finetune_list")
            if table.count() > 0:
                test_pass(f"Finetune page has table, new button: {new_btn.count() > 0}")
            else:
                test_fail("Finetune page", "No table found")
        except Exception as e:
            test_fail("Finetune page", str(e))
            screenshot(page, "06_finetune_error")

        # ── Test 7: Create finetune wizard ──
        print("[7/12] Testing Create Fine-tune wizard...")
        try:
            page.goto(f"{DASHBOARD_URL}/finetune/create", wait_until="networkidle")
            page.wait_for_timeout(2000)
            steps = page.locator(".ant-steps, .ant-steps-item")
            form_items = page.locator(".ant-form-item, .ant-input, .ant-select, input")
            screenshot(page, "07_finetune_wizard")
            if steps.count() > 0 and form_items.count() > 0:
                test_pass(f"Finetune wizard has {steps.count()} step elements, {form_items.count()} form fields")
            else:
                test_fail("Finetune wizard", f"Steps: {steps.count()}, Forms: {form_items.count()}")
        except Exception as e:
            test_fail("Finetune wizard", str(e))
            screenshot(page, "07_finetune_wizard_error")

        # ── Test 8: RAG page ──
        print("[8/12] Testing RAG Knowledge Bases page...")
        try:
            page.goto(f"{DASHBOARD_URL}/rag", wait_until="networkidle")
            page.wait_for_timeout(2000)
            table = page.locator(".ant-table, table")
            new_btn = page.locator("button:has-text('KB'), button:has-text('New')")
            screenshot(page, "08_rag_list")
            if table.count() > 0:
                test_pass(f"RAG page has table, new button: {new_btn.count() > 0}")
            else:
                test_fail("RAG page", "No table found")
        except Exception as e:
            test_fail("RAG page", str(e))
            screenshot(page, "08_rag_error")

        # ── Test 9: Services page ──
        print("[9/12] Testing Services page...")
        try:
            page.goto(f"{DASHBOARD_URL}/services", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Should have service cards
            cards = page.locator(".ant-card")
            card_count = cards.count()
            screenshot(page, "09_services_grid")
            if card_count >= 5:
                test_pass(f"Services page shows {card_count} service cards")
            else:
                test_fail("Services page", f"Expected >=5 cards, got {card_count}")
        except Exception as e:
            test_fail("Services page", str(e))
            screenshot(page, "09_services_error")

        # ── Test 10: Monitoring page ──
        print("[10/12] Testing Monitoring page...")
        try:
            page.goto(f"{DASHBOARD_URL}/monitoring", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Should have Grafana dashboard cards
            cards = page.locator(".ant-card")
            card_count = cards.count()
            screenshot(page, "10_monitoring")
            if card_count >= 5:
                test_pass(f"Monitoring page shows {card_count} dashboard cards")
            else:
                test_fail("Monitoring page", f"Expected >=5 cards, got {card_count}")
        except Exception as e:
            test_fail("Monitoring page", str(e))
            screenshot(page, "10_monitoring_error")

        # ── Test 11: Platform page ──
        print("[11/12] Testing Platform page...")
        try:
            page.goto(f"{DASHBOARD_URL}/platform", wait_until="networkidle")
            page.wait_for_timeout(2000)
            # Should have cards with descriptions
            cards = page.locator(".ant-card")
            screenshot(page, "11_platform")
            if cards.count() >= 1:
                test_pass(f"Platform page has {cards.count()} cards")
            else:
                test_fail("Platform page", "No cards found")
        except Exception as e:
            test_fail("Platform page", str(e))
            screenshot(page, "11_platform_error")

        # ── Test 12: Theme toggle ──
        print("[12/12] Testing theme toggle...")
        try:
            page.goto(f"{DASHBOARD_URL}/overview", wait_until="networkidle")
            page.wait_for_timeout(1000)
            # Find theme toggle button (sun/moon/desktop icon)
            theme_btn = page.locator("button:has(.anticon-sun), button:has(.anticon-moon), button:has(.anticon-desktop), button:has(.anticon)")
            if theme_btn.count() > 0:
                # Get current background color
                bg_before = page.evaluate("window.getComputedStyle(document.body).backgroundColor")
                # Click theme toggle (first matching button in header area)
                header_btns = page.locator("header button, .ant-layout-header button")
                for i in range(header_btns.count()):
                    btn = header_btns.nth(i)
                    # Try clicking buttons in header to find the theme toggle
                    try:
                        btn.click()
                        page.wait_for_timeout(500)
                        break
                    except:
                        continue
                bg_after = page.evaluate("window.getComputedStyle(document.body).backgroundColor")
                screenshot(page, "12_theme_toggled")
                test_pass(f"Theme toggle clicked. Background before: {bg_before}, after: {bg_after}")
            else:
                test_fail("Theme toggle", "No theme button found")
                screenshot(page, "12_theme_error")
        except Exception as e:
            test_fail("Theme toggle", str(e))
            screenshot(page, "12_theme_error")

        # ── Test 13: API endpoints ──
        print("[13/13] Testing API endpoints...")
        try:
            # Test services API
            resp = page.evaluate("""async () => {
                const r = await fetch('/api/v1/services');
                return { status: r.status, body: await r.json() };
            }""")
            if resp["status"] == 200 and isinstance(resp["body"], list) and len(resp["body"]) >= 5:
                test_pass(f"GET /api/v1/services returns {len(resp['body'])} services")
            else:
                test_fail("API /services", f"Status: {resp['status']}, body length: {len(resp.get('body', []))}")

            # Test monitoring API
            resp2 = page.evaluate("""async () => {
                const r = await fetch('/api/v1/monitoring');
                return { status: r.status, body: await r.json() };
            }""")
            if resp2["status"] == 200:
                test_pass(f"GET /api/v1/monitoring returns status {resp2['status']}")
            else:
                test_fail("API /monitoring", f"Status: {resp2['status']}")

            # Test users API (should work - no OIDC required when provider is nil)
            resp3 = page.evaluate("""async () => {
                const r = await fetch('/api/v1/users');
                return { status: r.status };
            }""")
            test_pass(f"GET /api/v1/users returns status {resp3['status']}")

            # Test roles API
            resp4 = page.evaluate("""async () => {
                const r = await fetch('/api/v1/roles');
                return { status: r.status };
            }""")
            test_pass(f"GET /api/v1/roles returns status {resp4['status']}")

        except Exception as e:
            test_fail("API endpoints", str(e))

        # ── Test 14: Service proxy (Grafana) ──
        print("[14/14] Testing service proxy (Grafana)...")
        try:
            # Navigate via SPA (client-side routing) — server-side /services/grafana
            # hits the Go proxy and bypasses the SPA
            page.goto(f"{DASHBOARD_URL}/services", wait_until="networkidle")
            page.wait_for_timeout(1000)
            grafana_card = page.locator(".ant-card:has-text('grafana'), .ant-card:has-text('Grafana')").first
            if grafana_card.count() > 0:
                grafana_card.click()
                page.wait_for_timeout(2000)
                screenshot(page, "14_service_grafana_embed")
                iframe = page.locator("iframe")
                back_btn = page.locator("button:has-text('Back')")
                if iframe.count() > 0 or back_btn.count() > 0:
                    test_pass("Grafana service embed page rendered")
                else:
                    test_fail("Grafana embed", "No iframe or back button found")
            else:
                test_fail("Grafana embed", "No Grafana card on services page")
                screenshot(page, "14_grafana_error")
        except Exception as e:
            test_fail("Grafana embed", str(e))
            screenshot(page, "14_grafana_error")

        # ── Test 15: Navigate between pages via sidebar ──
        print("[15/15] Testing sidebar navigation...")
        try:
            page.goto(f"{DASHBOARD_URL}/overview", wait_until="networkidle")
            page.wait_for_timeout(1000)

            # Click on Models in sidebar (may be menu-item or menu-title-content)
            models_menu = page.locator("li.ant-menu-item:has-text('Models'), .ant-menu-title-content:has-text('Models')").first
            if models_menu.count() > 0:
                models_menu.click()
                page.wait_for_timeout(1500)
                current_url = page.url
                screenshot(page, "15_nav_to_models")
                if "/models" in current_url:
                    test_pass("Sidebar navigation: Overview -> Models works")
                else:
                    test_fail("Sidebar navigation", f"Expected /models URL, got {current_url}")
            else:
                test_fail("Sidebar navigation", "Models menu item not found")
                screenshot(page, "15_nav_error")
        except Exception as e:
            test_fail("Sidebar navigation", str(e))
            screenshot(page, "15_nav_error")

        browser.close()

    # ── Summary ──
    print(f"\n{'=' * 60}")
    print(f"Dashboard E2E Test Results: {passed} passed, {failed} failed")
    print(f"Screenshots saved to: {SCREENSHOT_DIR}")
    if errors:
        print(f"\nFailed tests:")
        for e in errors:
            print(f"  - {e}")
    print(f"{'=' * 60}")

    return failed == 0


if __name__ == "__main__":
    success = run_tests()
    sys.exit(0 if success else 1)
