#!/usr/bin/env python3
"""
Hardware probe: detect available GPU resources.
"""

import json
import logging
import subprocess
from dataclasses import dataclass

log = logging.getLogger("model-resolver")


@dataclass
class GPUInfo:
    index: int
    name: str
    vram_mb: int


@dataclass
class HardwareInfo:
    gpus: list[GPUInfo]
    total_vram_mb: int
    gpu_count: int

    @property
    def has_gpu(self) -> bool:
        return self.gpu_count > 0


def probe_hardware() -> HardwareInfo:
    """Detect available GPUs via nvidia-smi."""
    gpus = []
    try:
        result = subprocess.run(
            [
                "nvidia-smi",
                "--query-gpu=index,name,memory.total",
                "--format=csv,noheader,nounits",
            ],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0:
            for line in result.stdout.strip().split("\n"):
                if not line.strip():
                    continue
                parts = [p.strip() for p in line.split(",")]
                if len(parts) >= 3:
                    gpus.append(GPUInfo(
                        index=int(parts[0]),
                        name=parts[1],
                        vram_mb=int(float(parts[2])),
                    ))
    except (FileNotFoundError, subprocess.TimeoutExpired):
        log.info("nvidia-smi not found or timed out, assuming CPU-only environment")

    total_vram = sum(g.vram_mb for g in gpus)
    return HardwareInfo(gpus=gpus, total_vram_mb=total_vram, gpu_count=len(gpus))
