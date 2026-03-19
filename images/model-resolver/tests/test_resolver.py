"""
Unit tests for engine resolver logic.
Tests the mapping from (model_meta, hardware) -> engine selection.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

import yaml
from format_detector import ModelMeta
from hardware_probe import HardwareInfo, GPUInfo
from resolver import resolve_engine, match_rule


def load_test_engine_map():
    """Load the real engine_map.yaml for testing."""
    map_path = Path(__file__).parent.parent / "engine_map.yaml"
    with open(map_path) as f:
        return yaml.safe_load(f)


def make_meta(format="safetensors", quant="none", model_type="text-generation", model_id="test/model"):
    return ModelMeta(
        model_id=model_id,
        format=format,
        quant_method=quant,
        model_type=model_type,
        files=[],
        estimated_size_gb=10.0,
    )


def gpu_hardware(count=1, vram_mb=81920):
    return HardwareInfo(
        gpus=[GPUInfo(index=i, name="A100", vram_mb=vram_mb) for i in range(count)],
        total_vram_mb=vram_mb * count,
        gpu_count=count,
    )


def no_gpu():
    return HardwareInfo(gpus=[], total_vram_mb=0, gpu_count=0)


class TestEngineResolver:
    def setup_method(self):
        self.engine_map = load_test_engine_map()

    def test_gguf_selects_llamacpp(self):
        meta = make_meta(format="gguf")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "llamacpp"

    def test_safetensors_gptq_selects_vllm(self):
        meta = make_meta(format="safetensors", quant="gptq")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "vllm"
        assert result["args"]["--quantization"] == "gptq"

    def test_safetensors_awq_selects_vllm(self):
        meta = make_meta(format="safetensors", quant="awq")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "vllm"
        assert result["args"]["--quantization"] == "awq"

    def test_safetensors_fp8_selects_vllm(self):
        meta = make_meta(format="safetensors", quant="fp8")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "vllm"
        assert result["args"]["--quantization"] == "fp8"

    def test_safetensors_no_quant_selects_vllm(self):
        meta = make_meta(format="safetensors", quant="none")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "vllm"

    def test_embedding_selects_tei(self):
        meta = make_meta(model_type="embedding")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "tei"

    def test_reranker_selects_tei(self):
        meta = make_meta(model_type="reranker")
        result = resolve_engine(meta, gpu_hardware(), self.engine_map)
        assert result["engine"] == "tei"

    def test_no_gpu_forces_llamacpp(self):
        meta = make_meta(format="safetensors", quant="none")
        result = resolve_engine(meta, no_gpu(), self.engine_map)
        assert result["engine"] == "llamacpp"

    def test_no_gpu_even_for_gptq(self):
        """Even quantized models fall back to llama.cpp without GPU."""
        meta = make_meta(format="safetensors", quant="gptq")
        result = resolve_engine(meta, no_gpu(), self.engine_map)
        assert result["engine"] == "llamacpp"
