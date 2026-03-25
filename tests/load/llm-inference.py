# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""LLM inference load test — measures chat-completion latency with stdlib only."""

import argparse
import json
import statistics
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

DEFAULT_URL = "http://localhost:4000/v1/chat/completions"
PAYLOAD = json.dumps({
    "model": "default",
    "messages": [{"role": "user", "content": "Say hello in one sentence."}],
    "max_tokens": 32,
}).encode()


def send_request(url: str, api_key: str) -> float:
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    req = urllib.request.Request(url, data=PAYLOAD, headers=headers, method="POST")
    start = time.perf_counter()
    with urllib.request.urlopen(req, timeout=60) as resp:
        resp.read()
    return time.perf_counter() - start


def percentile(data: list[float], p: int) -> float:
    k = (len(data) - 1) * (p / 100)
    f, c = int(k), int(k) + 1
    if f == c or c >= len(data):
        return data[f]
    return data[f] * (c - k) + data[c] * (k - f)


def main() -> None:
    ap = argparse.ArgumentParser(description="LLM inference load test")
    ap.add_argument("--url", default=DEFAULT_URL)
    ap.add_argument("--api-key", default="")
    ap.add_argument("--concurrency", type=int, default=4)
    ap.add_argument("--requests", type=int, default=50)
    args = ap.parse_args()

    latencies: list[float] = []
    errors = 0
    t0 = time.perf_counter()

    with ThreadPoolExecutor(max_workers=args.concurrency) as pool:
        futures = [pool.submit(send_request, args.url, args.api_key) for _ in range(args.requests)]
        for f in as_completed(futures):
            try:
                latencies.append(f.result())
            except Exception as exc:  # noqa: BLE001
                errors += 1
                print(f"ERROR: {exc}")

    wall = time.perf_counter() - t0
    latencies.sort()
    summary = {
        "test": "llm-inference",
        "total_requests": args.requests,
        "concurrency": args.concurrency,
        "errors": errors,
        "wall_seconds": round(wall, 2),
        "rps": round(len(latencies) / wall, 2) if wall else 0,
        "latency_p50": round(percentile(latencies, 50), 4) if latencies else None,
        "latency_p95": round(percentile(latencies, 95), 4) if latencies else None,
        "latency_p99": round(percentile(latencies, 99), 4) if latencies else None,
        "latency_mean": round(statistics.mean(latencies), 4) if latencies else None,
    }
    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
