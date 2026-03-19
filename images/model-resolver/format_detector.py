#!/usr/bin/env python3
"""
Model format detector.
Fetches model metadata from HuggingFace Hub API and determines:
- File format (safetensors, gguf, bin, etc.)
- Quantization method (awq, gptq, fp8, bitsandbytes, none)
- Model type (text-generation, embedding, reranker)
"""

import json
import logging
import os
from dataclasses import dataclass
from pathlib import Path

log = logging.getLogger("model-resolver")


@dataclass
class ModelMeta:
    """Detected model metadata."""
    model_id: str
    format: str             # safetensors | gguf | bin | onnx | unknown
    quant_method: str       # awq | gptq | fp8 | bitsandbytes | none
    model_type: str         # text-generation | embedding | reranker | unknown
    files: list[str]        # List of files in the repo
    estimated_size_gb: float  # Rough model size estimate


def detect_from_files(files: list[str]) -> str:
    """Detect primary format from file listing."""
    extensions = {Path(f).suffix.lower() for f in files}

    if ".gguf" in extensions:
        return "gguf"
    if ".safetensors" in extensions:
        return "safetensors"
    if ".onnx" in extensions:
        return "onnx"
    if ".bin" in extensions:
        return "bin"
    return "unknown"


def detect_quant_method(config: dict) -> str:
    """Detect quantization method from config.json."""
    # Check quantization_config
    quant_config = config.get("quantization_config", {})
    if quant_config:
        method = quant_config.get("quant_method", "").lower()
        if method in ("awq", "gptq", "fp8", "bitsandbytes"):
            return method

    return "none"


EMBEDDING_ARCHITECTURES = {
    "BertModel", "XLMRobertaModel", "DistilBertModel",
    "SentenceTransformer", "E5Model", "BGEModel",
    "XLMRobertaForSequenceClassification",
    "BertForSequenceClassification",
}

RERANKER_ARCHITECTURES = {
    "XLMRobertaForSequenceClassification",
    "BertForSequenceClassification",
    "DebertaV2ForSequenceClassification",
}


def detect_model_type(config: dict, model_id: str) -> str:
    """Detect if model is text-generation, embedding, or reranker."""
    architectures = config.get("architectures", [])

    # Check by architecture name
    for arch in architectures:
        if "ForCausalLM" in arch or "ForConditionalGeneration" in arch:
            return "text-generation"
        if arch in RERANKER_ARCHITECTURES:
            # Reranker check before embedding since some share architectures
            model_id_lower = model_id.lower()
            if "rerank" in model_id_lower:
                return "reranker"
            return "embedding"
        if arch in EMBEDDING_ARCHITECTURES:
            return "embedding"

    # Check model_type field
    model_type = config.get("model_type", "").lower()
    if model_type in ("llama", "qwen2", "qwen2_moe", "mistral", "gemma", "gpt2", "phi", "deepseek_v2"):
        return "text-generation"

    # Check by model_id heuristics
    model_id_lower = model_id.lower()
    if "rerank" in model_id_lower:
        return "reranker"
    if any(kw in model_id_lower for kw in ("embed", "bge", "e5", "gte", "sentence")):
        return "embedding"

    return "text-generation"  # safe default


def estimate_size_gb(config: dict, files: list[str]) -> float:
    """Rough estimate of model size in GB from parameters or file sizes."""
    # Try to estimate from num_parameters
    num_params = config.get("num_parameters", 0)
    if num_params > 0:
        # FP16: 2 bytes per param, FP32: 4 bytes, INT4: 0.5 bytes
        return num_params * 2 / (1024 ** 3)  # assume FP16

    # Count safetensors/bin files as rough proxy
    model_files = [f for f in files if f.endswith((".safetensors", ".bin", ".gguf"))]
    return len(model_files) * 5.0  # rough 5GB per shard


def fetch_model_meta(model_id: str) -> ModelMeta:
    """Fetch model metadata from HuggingFace Hub API (no full download)."""
    try:
        from huggingface_hub import HfApi
        api = HfApi()
        model_info = api.model_info(model_id)

        files = [s.rfilename for s in (model_info.siblings or [])]

        # Try to get config.json content
        config = {}
        try:
            config_path = api.hf_hub_download(
                repo_id=model_id,
                filename="config.json",
                local_dir="/tmp/hf_config_cache",
            )
            with open(config_path) as f:
                config = json.load(f)
        except Exception:
            log.warning("Could not fetch config.json for %s, using heuristics", model_id)

        file_format = detect_from_files(files)
        quant_method = detect_quant_method(config)
        model_type = detect_model_type(config, model_id)
        size_gb = estimate_size_gb(config, files)

        return ModelMeta(
            model_id=model_id,
            format=file_format,
            quant_method=quant_method,
            model_type=model_type,
            files=files,
            estimated_size_gb=size_gb,
        )
    except ImportError:
        log.warning("huggingface_hub not installed, using model_id heuristics only")
        return _detect_from_model_id(model_id)


def _detect_from_model_id(model_id: str) -> ModelMeta:
    """Fallback: detect from model_id string heuristics (no API call)."""
    model_id_lower = model_id.lower()

    # Format detection
    if "gguf" in model_id_lower:
        file_format = "gguf"
    else:
        file_format = "safetensors"

    # Quantization detection
    quant_method = "none"
    if "awq" in model_id_lower:
        quant_method = "awq"
    elif "gptq" in model_id_lower:
        quant_method = "gptq"
    elif "fp8" in model_id_lower:
        quant_method = "fp8"

    # Model type (reranker check BEFORE embedding -- "bge-reranker" contains both "bge" and "rerank")
    if "rerank" in model_id_lower:
        model_type = "reranker"
    elif any(kw in model_id_lower for kw in ("embed", "bge", "e5", "gte")):
        model_type = "embedding"
    else:
        model_type = "text-generation"

    return ModelMeta(
        model_id=model_id,
        format=file_format,
        quant_method=quant_method,
        model_type=model_type,
        files=[],
        estimated_size_gb=0.0,
    )
