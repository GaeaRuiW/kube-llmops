FROM python:3.13-slim

RUN pip install --no-cache-dir minio huggingface_hub hf-transfer

ENV HF_HUB_ENABLE_HF_TRANSFER=1
ENV HF_TRANSFER_CONCURRENCY=32

ENTRYPOINT ["bash", "-c"]
