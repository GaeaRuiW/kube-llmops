FROM python:3.13-slim

RUN pip install --no-cache-dir \
    mlflow==3.10.2 \
    psycopg2-binary==2.9.10 \
    boto3==1.37.1

RUN groupadd -r mlflow && useradd -r -g mlflow -d /home/mlflow -s /sbin/nologin mlflow \
    && mkdir -p /home/mlflow && chown mlflow:mlflow /home/mlflow

USER mlflow

EXPOSE 5000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD python3 -c "import urllib.request; urllib.request.urlopen('http://localhost:5000/health')" || exit 1

ENTRYPOINT ["mlflow", "server"]
