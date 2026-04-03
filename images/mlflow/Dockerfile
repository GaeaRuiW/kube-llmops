FROM python:3.13-slim

RUN pip install --no-cache-dir mlflow[gateway] psycopg2-binary boto3

EXPOSE 5000

ENTRYPOINT ["mlflow", "server"]
