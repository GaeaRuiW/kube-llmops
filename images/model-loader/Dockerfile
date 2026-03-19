FROM python:3.11-slim

LABEL org.opencontainers.image.source="https://github.com/GaeaRuiW/kube-llmops"
LABEL org.opencontainers.image.description="kube-llmops model-loader: downloads model weights"

RUN pip install --no-cache-dir huggingface_hub

COPY loader.py /app/loader.py

ENTRYPOINT ["python", "/app/loader.py"]
