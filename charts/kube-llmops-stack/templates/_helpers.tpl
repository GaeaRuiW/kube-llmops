{{/*
Expand the name of the chart.
*/}}
{{- define "kube-llmops.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kube-llmops.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kube-llmops.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kube-llmops.labels" -}}
helm.sh/chart: {{ include "kube-llmops.chart" . }}
{{ include "kube-llmops.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: kube-llmops
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kube-llmops.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-llmops.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Resolve engine for a model based on its source name.
Input: a model dict with .source (required) and .engine (optional).
Output: one of "vllm", "tei", "llamacpp".

Priority:
  1. Explicit .engine value (if set and not "" or "auto") — user override
  2. Heuristic detection from .source name patterns
  3. Fallback: vllm

Usage:
  {{- $engine := include "kube-llmops.resolveEngine" $model | trim -}}
*/}}
{{- define "kube-llmops.resolveEngine" -}}
{{- $engine := default "" .engine -}}
{{- if and (ne $engine "") (ne $engine "auto") -}}
  {{- $engine -}}
{{- else -}}
  {{- $src := default "" .source | lower -}}
  {{- if or (contains "gguf" $src) (hasSuffix "-gguf" $src) -}}
    llamacpp
  {{- else if or (contains "rerank" $src) -}}
    tei
  {{- else if or
    (contains "/bge-" $src)
    (contains "/e5-" $src)
    (contains "/gte-" $src)
    (contains "minilm" $src)
    (contains "/jina-embed" $src)
    (contains "/nomic-embed" $src)
    (contains "/all-mpnet" $src)
    (contains "embedding" $src)
  -}}
    tei
  {{- else -}}
    vllm
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Resolve model type: "embedding", "reranker", or "llm".
Used by LiteLLM configmap to choose prefix (huggingface/ vs openai/).
*/}}
{{- define "kube-llmops.resolveModelType" -}}
{{- $src := default "" .source | lower -}}
{{- if contains "rerank" $src -}}
  reranker
{{- else if or
  (contains "/bge-" $src)
  (contains "/e5-" $src)
  (contains "/gte-" $src)
  (contains "minilm" $src)
  (contains "/jina-embed" $src)
  (contains "/nomic-embed" $src)
  (contains "/all-mpnet" $src)
  (contains "embedding" $src)
-}}
  embedding
{{- else -}}
  llm
{{- end -}}
{{- end -}}

{{/*
Shared model-loader init-container Python script.
Flow: MinIO cache → HuggingFace fallback → upload back to MinIO.

Expects env vars: MODEL_SOURCE, MODEL_DIR, S3_ENDPOINT, S3_ACCESS_KEY,
                  S3_SECRET_KEY, S3_BUCKET, HF_TOKEN (optional)
*/}}
{{- define "kube-llmops.modelLoaderScript" -}}
pip install -q minio huggingface_hub 2>/dev/null
python3 -c "
import os, sys, logging
from pathlib import Path

logging.basicConfig(level=logging.INFO, format='%(asctime)s [model-loader] %(levelname)s %(message)s')
log = logging.getLogger('model-loader')

source   = os.environ.get('MODEL_SOURCE', '')
target   = Path(os.environ.get('MODEL_DIR', '/models'))
endpoint = os.environ.get('S3_ENDPOINT', '')
bucket   = os.environ.get('S3_BUCKET', 'models')
ak       = os.environ.get('S3_ACCESS_KEY', 'minioadmin')
sk       = os.environ.get('S3_SECRET_KEY', 'minioadmin')

target.mkdir(parents=True, exist_ok=True)
if not source:
    log.error('MODEL_SOURCE is required'); sys.exit(1)

# Normalise: 'Qwen/Qwen2.5-0.5B-Instruct' → 'Qwen--Qwen2.5-0.5B-Instruct'
slug = source.replace('/', '--')
local_dir = target / slug

# ── Helper: MinIO client (lazy) ──
_mc = None
def mc():
    global _mc
    if _mc is None:
        if not endpoint:
            return None
        from minio import Minio
        _mc = Minio(endpoint, access_key=ak, secret_key=sk, secure=False)
        # Ensure bucket exists
        if not _mc.bucket_exists(bucket):
            _mc.make_bucket(bucket)
    return _mc

def s3_prefix():
    return slug + '/'

def model_in_minio():
    c = mc()
    if c is None:
        return False
    objs = list(c.list_objects(bucket, prefix=s3_prefix(), recursive=False))
    return len(objs) > 0

def download_from_minio():
    c = mc()
    local_dir.mkdir(parents=True, exist_ok=True)
    for obj in c.list_objects(bucket, prefix=s3_prefix(), recursive=True):
        rel = obj.object_name[len(s3_prefix()):]
        if not rel:
            continue
        fpath = local_dir / rel
        fpath.parent.mkdir(parents=True, exist_ok=True)
        if fpath.exists() and fpath.stat().st_size == obj.size:
            log.info(f'  Cached: {rel}')
            continue
        log.info(f'  Downloading from MinIO: {rel} ({obj.size/(1024**3):.2f} GB)')
        c.fget_object(bucket, obj.object_name, str(fpath))
    log.info(f'MinIO sync complete → {local_dir}')

def download_from_hf():
    from huggingface_hub import snapshot_download
    # If HF_HOME is set to model dir, use default cache layout (for TEI compatibility)
    hf_home = os.environ.get('HF_HOME', '')
    if hf_home and hf_home == str(target):
        cache_dir = target / 'hub'
        marker = cache_dir / ('models--' + source.replace('/', '--'))
        if marker.exists() and any(marker.rglob('*')):
            log.info(f'Using HF cache: {marker}')
            return
        log.info(f'Downloading from HuggingFace (cache mode): {source}')
        snapshot_download(repo_id=source, cache_dir=str(cache_dir))
    else:
        if local_dir.exists() and any(local_dir.glob('*')):
            log.info(f'Using PVC cache: {local_dir}')
            return
        log.info(f'Downloading from HuggingFace: {source}')
        snapshot_download(repo_id=source, local_dir=str(local_dir))

def upload_to_minio():
    c = mc()
    if c is None:
        return
    log.info(f'Uploading to MinIO s3://{bucket}/{s3_prefix()}...')
    count = 0
    for fpath in local_dir.rglob('*'):
        if not fpath.is_file():
            continue
        obj_name = s3_prefix() + str(fpath.relative_to(local_dir))
        # Skip if already exists with same size
        try:
            stat = c.stat_object(bucket, obj_name)
            if stat.size == fpath.stat().st_size:
                continue
        except Exception:
            pass
        c.fput_object(bucket, obj_name, str(fpath))
        count += 1
    log.info(f'Uploaded {count} files to MinIO')

# ── Main flow ──
try:
    if model_in_minio():
        log.info(f'Found in MinIO: s3://{bucket}/{s3_prefix()}')
        download_from_minio()
    else:
        log.info(f'Not in MinIO, fetching from HuggingFace...')
        download_from_hf()
        upload_to_minio()
except Exception as e:
    log.warning(f'MinIO unavailable ({e}), falling back to HuggingFace')
    download_from_hf()

log.info('Model loader done.')
"
{{- end -}}

{{/*
Model-loader init-container env vars.
Call with: include "kube-llmops.modelLoaderEnv" (dict "model" $model "root" $)
*/}}
{{- define "kube-llmops.modelLoaderEnv" -}}
- name: MODEL_SOURCE
  value: {{ .model.source | quote }}
- name: MODEL_DIR
  value: {{ .mountPath | default "/models" | quote }}
- name: HF_HOME
  value: {{ .mountPath | default "/models" | quote }}
{{- if and .root.Values.global .root.Values.global.modelStore }}
- name: S3_ENDPOINT
  value: {{ .root.Values.global.modelStore.endpoint | quote }}
- name: S3_BUCKET
  value: {{ .root.Values.global.modelStore.bucket | default "models" | quote }}
- name: S3_ACCESS_KEY
  value: {{ .root.Values.global.modelStore.accessKey | default "minioadmin" | quote }}
- name: S3_SECRET_KEY
  value: {{ .root.Values.global.modelStore.secretKey | default "minioadmin" | quote }}
{{- end }}
{{- if or (and .root.Values.global .root.Values.global.hfToken) .hfToken }}
- name: HF_TOKEN
  valueFrom:
    secretKeyRef:
      name: {{ .root.Release.Name }}-hf-token
      key: token
{{- end }}
{{- end -}}
