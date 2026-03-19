#!/usr/bin/env python3
"""
kube-llmops model-loader
Downloads model weights from HuggingFace Hub, ModelScope, S3, or OCI registry.
Used as an init-container before the inference engine starts.
"""

import os
import sys
import logging
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [model-loader] %(levelname)s %(message)s",
)
log = logging.getLogger("model-loader")


def download_huggingface(model_id: str, target_dir: Path) -> Path:
    """Download model from HuggingFace Hub."""
    from huggingface_hub import snapshot_download

    log.info("Downloading from HuggingFace Hub: %s", model_id)
    local_path = snapshot_download(
        repo_id=model_id,
        local_dir=str(target_dir / model_id.replace("/", "--")),
        local_dir_use_symlinks=False,
    )
    log.info("Download complete: %s", local_path)
    return Path(local_path)


def download_modelscope(model_id: str, target_dir: Path) -> Path:
    """Download model from ModelScope."""
    from modelscope.hub.snapshot_download import snapshot_download

    log.info("Downloading from ModelScope: %s", model_id)
    local_path = snapshot_download(
        model_id=model_id,
        cache_dir=str(target_dir),
    )
    log.info("Download complete: %s", local_path)
    return Path(local_path)


def download_s3(s3_uri: str, target_dir: Path) -> Path:
    """Download model from S3-compatible storage."""
    import subprocess

    log.info("Downloading from S3: %s", s3_uri)
    local_path = target_dir / s3_uri.split("/")[-1]
    local_path.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["aws", "s3", "sync", s3_uri, str(local_path)],
        check=True,
    )
    log.info("Download complete: %s", local_path)
    return local_path


def detect_source_type(source: str) -> str:
    """Detect the model source type from the URI."""
    if source.startswith("s3://"):
        return "s3"
    if source.startswith("oci://"):
        return "oci"
    if source.startswith("modelscope:"):
        return "modelscope"
    # Default: HuggingFace Hub (org/model format)
    return "huggingface"


def is_already_cached(model_id: str, target_dir: Path) -> bool:
    """Check if model is already downloaded."""
    model_dir = target_dir / model_id.replace("/", "--")
    if model_dir.exists():
        # Check for at least one model file
        model_files = list(model_dir.glob("*.safetensors")) + \
                      list(model_dir.glob("*.bin")) + \
                      list(model_dir.glob("*.gguf"))
        if model_files:
            log.info("Model already cached at %s (%d files)", model_dir, len(model_files))
            return True
    return False


def main():
    model_source = os.environ.get("MODEL_SOURCE", "")
    model_dir = Path(os.environ.get("MODEL_DIR", "/models"))

    if not model_source:
        log.error("MODEL_SOURCE environment variable is required")
        sys.exit(1)

    model_dir.mkdir(parents=True, exist_ok=True)

    source_type = detect_source_type(model_source)
    log.info("Source type: %s, Model: %s, Target: %s", source_type, model_source, model_dir)

    # Check cache first
    if source_type == "huggingface" and is_already_cached(model_source, model_dir):
        log.info("Using cached model, skipping download")
        return

    try:
        if source_type == "huggingface":
            download_huggingface(model_source, model_dir)
        elif source_type == "modelscope":
            model_id = model_source.replace("modelscope:", "")
            download_modelscope(model_id, model_dir)
        elif source_type == "s3":
            download_s3(model_source, model_dir)
        elif source_type == "oci":
            log.error("OCI download not yet implemented (coming in M10)")
            sys.exit(1)
        else:
            log.error("Unknown source type: %s", source_type)
            sys.exit(1)
    except Exception as e:
        log.error("Download failed: %s", e)
        sys.exit(1)

    log.info("Model loader completed successfully")


if __name__ == "__main__":
    main()
