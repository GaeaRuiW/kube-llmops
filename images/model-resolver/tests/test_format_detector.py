"""
Unit tests for model format detector and engine resolver.
Tests run without network or GPU -- using mock data and heuristic-only detection.
"""

import sys
from pathlib import Path

# Add parent dir to path so we can import the modules
sys.path.insert(0, str(Path(__file__).parent.parent))

from format_detector import (
    ModelMeta,
    detect_from_files,
    detect_quant_method,
    detect_model_type,
    _detect_from_model_id,
)


class TestDetectFromFiles:
    def test_gguf_format(self):
        files = ["model-q4_0.gguf", "README.md"]
        assert detect_from_files(files) == "gguf"

    def test_safetensors_format(self):
        files = ["model-00001-of-00004.safetensors", "config.json"]
        assert detect_from_files(files) == "safetensors"

    def test_onnx_format(self):
        files = ["model.onnx", "config.json"]
        assert detect_from_files(files) == "onnx"

    def test_bin_format(self):
        files = ["pytorch_model.bin", "config.json"]
        assert detect_from_files(files) == "bin"

    def test_gguf_takes_priority(self):
        """GGUF should be detected even if safetensors also present."""
        files = ["model.gguf", "model.safetensors"]
        assert detect_from_files(files) == "gguf"

    def test_unknown_format(self):
        files = ["README.md", "LICENSE"]
        assert detect_from_files(files) == "unknown"


class TestDetectQuantMethod:
    def test_gptq(self):
        config = {"quantization_config": {"quant_method": "gptq", "bits": 4}}
        assert detect_quant_method(config) == "gptq"

    def test_awq(self):
        config = {"quantization_config": {"quant_method": "awq"}}
        assert detect_quant_method(config) == "awq"

    def test_fp8(self):
        config = {"quantization_config": {"quant_method": "fp8"}}
        assert detect_quant_method(config) == "fp8"

    def test_no_quantization(self):
        config = {"model_type": "llama"}
        assert detect_quant_method(config) == "none"

    def test_empty_config(self):
        assert detect_quant_method({}) == "none"


class TestDetectModelType:
    def test_causal_lm(self):
        config = {"architectures": ["LlamaForCausalLM"]}
        assert detect_model_type(config, "meta-llama/Llama-3") == "text-generation"

    def test_qwen_moe(self):
        config = {"model_type": "qwen2_moe"}
        assert detect_model_type(config, "Qwen/Qwen3.5-122B-A10B") == "text-generation"

    def test_embedding_by_architecture(self):
        config = {"architectures": ["XLMRobertaModel"]}
        assert detect_model_type(config, "BAAI/bge-m3") == "embedding"

    def test_embedding_by_model_id(self):
        config = {}
        assert detect_model_type(config, "BAAI/bge-small-en-v1.5") == "embedding"

    def test_reranker_by_model_id(self):
        config = {"architectures": ["XLMRobertaForSequenceClassification"]}
        assert detect_model_type(config, "BAAI/bge-reranker-v2-m3") == "reranker"

    def test_default_to_text_generation(self):
        config = {}
        assert detect_model_type(config, "some-random-model") == "text-generation"


class TestDetectFromModelId:
    """Test the fallback heuristic-only detection (no API call)."""

    def test_gptq_model(self):
        meta = _detect_from_model_id("Qwen/Qwen3.5-122B-A10B-GPTQ-Int4")
        assert meta.format == "safetensors"
        assert meta.quant_method == "gptq"
        assert meta.model_type == "text-generation"

    def test_awq_model(self):
        meta = _detect_from_model_id("TheBloke/Llama-2-7B-AWQ")
        assert meta.quant_method == "awq"
        assert meta.model_type == "text-generation"

    def test_gguf_model(self):
        meta = _detect_from_model_id("bartowski/Meta-Llama-3.1-8B-Instruct-GGUF")
        assert meta.format == "gguf"
        assert meta.model_type == "text-generation"

    def test_fp8_model(self):
        meta = _detect_from_model_id("Qwen/Qwen3.5-122B-A10B-FP8")
        assert meta.quant_method == "fp8"

    def test_embedding_model(self):
        meta = _detect_from_model_id("BAAI/bge-m3")
        assert meta.model_type == "embedding"

    def test_reranker_model(self):
        meta = _detect_from_model_id("BAAI/bge-reranker-v2-m3")
        assert meta.model_type == "reranker"

    def test_plain_model(self):
        meta = _detect_from_model_id("Qwen/Qwen3.5-0.8B")
        assert meta.format == "safetensors"
        assert meta.quant_method == "none"
        assert meta.model_type == "text-generation"
