#!/usr/bin/env python3
"""
kube-llmops Model Resolver
Detects model format and hardware, selects optimal engine + args.
Outputs /resolve/engine.env for the Helm template to consume.
"""

import logging
import os
import sys
from pathlib import Path

import yaml

from format_detector import ModelMeta, fetch_model_meta, _detect_from_model_id
from hardware_probe import probe_hardware, HardwareInfo

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [model-resolver] %(levelname)s %(message)s",
)
log = logging.getLogger("model-resolver")


def load_engine_map(path: str = "/app/engine_map.yaml") -> dict:
    """Load engine mapping rules from YAML."""
    with open(path) as f:
        return yaml.safe_load(f)


def match_rule(meta: ModelMeta, rule: dict) -> bool:
    """Check if a model matches a rule's conditions."""
    match = rule.get("match", {})
    for key, expected in match.items():
        actual = getattr(meta, key, None)
        if actual != expected:
            return False
    return True


def resolve_engine(meta: ModelMeta, hardware: HardwareInfo, engine_map: dict) -> dict:
    """Select the best engine based on model metadata and hardware."""

    # CPU-only: force llama.cpp fallback
    if not hardware.has_gpu:
        log.info("No GPU detected, using CPU fallback")
        fb = engine_map.get("cpu_fallback", engine_map.get("fallback", {}))
        return {
            "engine": fb["engine"],
            "image": fb["image"],
            "args": fb.get("args", {}),
        }

    # Try matching rules in order (first match wins)
    for rule in engine_map.get("rules", []):
        if match_rule(meta, rule):
            log.info("Matched rule: %s -> %s", rule["match"], rule["engine"])
            return {
                "engine": rule["engine"],
                "image": rule["image"],
                "args": rule.get("args", {}),
            }

    # Fallback
    fb = engine_map.get("fallback", {})
    log.info("No rule matched, using fallback: %s", fb.get("engine"))
    return {
        "engine": fb.get("engine", "vllm"),
        "image": fb.get("image", "vllm/vllm-openai:latest"),
        "args": fb.get("args", {}),
    }


def write_output(result: dict, output_dir: str = "/resolve"):
    """Write engine selection to env file for Helm consumption."""
    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)

    env_file = output_path / "engine.env"
    args_str = " ".join(
        f"{k} {v}" if v else k
        for k, v in result["args"].items()
    )

    with open(env_file, "w") as f:
        f.write(f"ENGINE={result['engine']}\n")
        f.write(f"ENGINE_IMAGE={result['image']}\n")
        f.write(f"ENGINE_ARGS={args_str}\n")

    log.info("Wrote engine config to %s", env_file)
    log.info("  ENGINE=%s", result["engine"])
    log.info("  ENGINE_IMAGE=%s", result["image"])
    log.info("  ENGINE_ARGS=%s", args_str)


def main():
    model_source = os.environ.get("MODEL_SOURCE", "")
    engine_override = os.environ.get("ENGINE_OVERRIDE", "")
    output_dir = os.environ.get("RESOLVE_OUTPUT", "/resolve")
    engine_map_path = os.environ.get("ENGINE_MAP", "/app/engine_map.yaml")

    if not model_source:
        log.error("MODEL_SOURCE is required")
        sys.exit(1)

    # If user explicitly set engine, skip detection
    if engine_override:
        log.info("Engine override: %s (skipping auto-detection)", engine_override)
        engine_map = load_engine_map(engine_map_path)
        # Find the matching engine in rules to get the image
        image = "vllm/vllm-openai:latest"
        for rule in engine_map.get("rules", []):
            if rule["engine"] == engine_override:
                image = rule["image"]
                break
        write_output({
            "engine": engine_override,
            "image": image,
            "args": {},
        }, output_dir)
        return

    # Auto-detect
    log.info("Auto-detecting engine for: %s", model_source)

    # Detect model format
    try:
        meta = fetch_model_meta(model_source)
    except Exception as e:
        log.warning("HF API detection failed (%s), falling back to ID heuristics", e)
        meta = _detect_from_model_id(model_source)

    log.info("Detected: format=%s, quant=%s, type=%s, size=%.1fGB",
             meta.format, meta.quant_method, meta.model_type, meta.estimated_size_gb)

    # Detect hardware
    hardware = probe_hardware()
    log.info("Hardware: %d GPUs, %dMB total VRAM", hardware.gpu_count, hardware.total_vram_mb)

    # Resolve engine
    engine_map = load_engine_map(engine_map_path)
    result = resolve_engine(meta, hardware, engine_map)

    # Write output
    write_output(result, output_dir)
    log.info("Model resolver completed successfully")


if __name__ == "__main__":
    main()
