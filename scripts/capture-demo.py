#!/usr/bin/env python3
"""
Generate demo screenshots and GIFs for kube-llmops README.
Captures: Grafana dashboards, Langfuse traces, LiteLLM UI, Keycloak, MinIO.
"""
import asyncio
import json
import os
import subprocess
import time
from pathlib import Path

# First, send a few API requests to generate trace data
BASE = "http://192.168.1.37"
LITELLM = f"{BASE}:30400"

def send_test_requests():
    """Send test requests through LiteLLM to generate metrics and traces."""
    import urllib.request, urllib.error
    
    prompts = [
        ("What is machine learning? Explain in 2 sentences.", 80),
        ("Write a Python function to calculate fibonacci numbers.", 120),
        ("Translate 'Hello World' to Chinese, Japanese, and Korean.", 60),
    ]
    
    for prompt, max_tokens in prompts:
        data = json.dumps({
            "model": "qwen3-5-122b-gptq",
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": max_tokens,
            "chat_template_kwargs": {"enable_thinking": False}
        }).encode()
        
        req = urllib.request.Request(
            f"{LITELLM}/v1/chat/completions",
            data=data,
            headers={
                "Authorization": "Bearer sk-kube-llmops-dev",
                "Content-Type": "application/json"
            }
        )
        try:
            resp = urllib.request.urlopen(req, timeout=60)
            result = json.loads(resp.read())
            content = result["choices"][0]["message"]["content"][:60]
            print(f"  ✓ '{prompt[:30]}...' → '{content}...'")
        except Exception as e:
            print(f"  ✗ '{prompt[:30]}...' → {e}")
        time.sleep(2)


async def capture_screenshots():
    from playwright.async_api import async_playwright
    
    out = Path("images/demo")
    out.mkdir(parents=True, exist_ok=True)
    
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        
        # ---- Grafana GPU Dashboard ----
        print("Capturing Grafana GPU dashboard...")
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()
        
        # Login to Grafana
        await page.goto(f"{BASE}:30300/login")
        await page.fill('input[name="user"]', 'admin')
        await page.fill('input[name="password"]', 'admin123!')
        await page.click('button[type="submit"]')
        await page.wait_for_timeout(3000)
        
        # Navigate to GPU dashboard
        await page.goto(f"{BASE}:30300/d/gpu-overview/gpu-infrastructure-overview?orgId=1&refresh=10s")
        await page.wait_for_timeout(5000)
        await page.screenshot(path=str(out / "grafana-gpu-dashboard.png"), full_page=False)
        print("  ✓ grafana-gpu-dashboard.png")
        
        # Navigate to vLLM dashboard
        await page.goto(f"{BASE}:30300/d/vllm-overview/vllm-model-serving-overview?orgId=1&refresh=10s")
        await page.wait_for_timeout(5000)
        await page.screenshot(path=str(out / "grafana-vllm-dashboard.png"), full_page=False)
        print("  ✓ grafana-vllm-dashboard.png")
        
        # Navigate to LiteLLM dashboard
        await page.goto(f"{BASE}:30300/d/litellm-gateway/litellm-ai-gateway?orgId=1&refresh=10s")
        await page.wait_for_timeout(5000)
        await page.screenshot(path=str(out / "grafana-litellm-dashboard.png"), full_page=False)
        print("  ✓ grafana-litellm-dashboard.png")
        
        await ctx.close()
        
        # ---- Langfuse Traces ----
        print("Capturing Langfuse traces...")
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()
        
        await page.goto(f"{BASE}:30301")
        await page.wait_for_timeout(2000)
        
        # Login
        try:
            await page.fill('input[name="email"]', 'admin@kube-llmops.local', timeout=5000)
            await page.fill('input[name="password"]', 'admin123!')
            await page.click('button[type="submit"]')
            await page.wait_for_timeout(3000)
        except:
            pass  # might already be logged in or different auth flow
        
        # Go to traces page
        await page.goto(f"{BASE}:30301/project/kube-llmops/traces")
        await page.wait_for_timeout(5000)
        await page.screenshot(path=str(out / "langfuse-traces.png"), full_page=False)
        print("  ✓ langfuse-traces.png")
        
        await ctx.close()
        
        # ---- Keycloak ----
        print("Capturing Keycloak...")
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()
        
        await page.goto(f"{BASE}:30808/admin/master/console/")
        await page.wait_for_timeout(2000)
        try:
            await page.fill('#username', 'admin', timeout=5000)
            await page.fill('#password', 'admin123!')
            await page.click('#kc-login')
            await page.wait_for_timeout(3000)
        except:
            pass
        
        # Navigate to kube-llmops realm clients
        await page.goto(f"{BASE}:30808/admin/kube-llmops/console/#/kube-llmops/clients")
        await page.wait_for_timeout(3000)
        await page.screenshot(path=str(out / "keycloak-clients.png"), full_page=False)
        print("  ✓ keycloak-clients.png")
        
        await ctx.close()
        
        # ---- MinIO Console ----
        print("Capturing MinIO...")
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()
        
        await page.goto(f"{BASE}:30901/login")
        await page.wait_for_timeout(2000)
        try:
            await page.fill('#accessKey', 'minioadmin', timeout=5000)
            await page.fill('#secretKey', 'minioadmin')
            await page.click('button[type="submit"]')
            await page.wait_for_timeout(3000)
        except:
            pass
        
        # Navigate to models bucket
        await page.goto(f"{BASE}:30901/browser/models")
        await page.wait_for_timeout(3000)
        await page.screenshot(path=str(out / "minio-models.png"), full_page=False)
        print("  ✓ minio-models.png")
        
        await ctx.close()
        
        # ---- LiteLLM UI ----
        print("Capturing LiteLLM...")
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()
        
        await page.goto(f"{BASE}:30400/ui")
        await page.wait_for_timeout(2000)
        await page.screenshot(path=str(out / "litellm-ui.png"), full_page=False)
        print("  ✓ litellm-ui.png")
        
        await ctx.close()
        await browser.close()

    # List all captured files
    print("\n=== Captured screenshots ===")
    for f in sorted(out.glob("*.png")):
        size = f.stat().st_size
        print(f"  {f.name}: {size // 1024} KB")


def create_gif(image_dir, output_path, delay=150):
    """Create animated GIF from PNG screenshots using pillow."""
    from PIL import Image
    
    images = sorted(Path(image_dir).glob("grafana-*.png"))
    if not images:
        print("No images for GIF")
        return
    
    frames = [Image.open(img) for img in images]
    frames[0].save(
        output_path,
        save_all=True,
        append_images=frames[1:],
        duration=delay * 10,  # milliseconds per frame
        loop=0
    )
    size = Path(output_path).stat().st_size
    print(f"  ✓ {output_path}: {size // 1024} KB ({len(frames)} frames)")


if __name__ == "__main__":
    print("=== Sending test requests for metrics/traces ===")
    send_test_requests()
    
    print("\n=== Waiting 10s for metrics to propagate ===")
    time.sleep(10)
    
    print("\n=== Capturing screenshots ===")
    asyncio.run(capture_screenshots())
    
    print("\n=== Creating GIFs ===")
    create_gif("images/demo", "images/demo/grafana-dashboards.gif", delay=200)
    
    print("\nDone!")
