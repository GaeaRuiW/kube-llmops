FROM python:3.11-slim

LABEL org.opencontainers.image.source="https://github.com/GaeaRuiW/kube-llmops"
LABEL org.opencontainers.image.description="kube-llmops model-resolver: auto-detect model format and select optimal engine"

RUN pip install --no-cache-dir huggingface_hub==0.28.1 pyyaml==6.0.2

COPY format_detector.py hardware_probe.py resolver.py engine_map.yaml /app/

WORKDIR /app
ENTRYPOINT ["python", "resolver.py"]
