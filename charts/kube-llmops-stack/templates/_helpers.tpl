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
  {{- if or (contains "gguf" $src) (hasSuffix "-gguf" $src) (contains "guff" $src) -}}
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
pip install -q minio huggingface_hub hf-transfer 2>/dev/null || true
export HF_HUB_ENABLE_HF_TRANSFER=1
python3 -c "
import os, sys, logging
from pathlib import Path

logging.basicConfig(level=logging.INFO, format='%(asctime)s [model-loader] %(levelname)s %(message)s')
log = logging.getLogger('model-loader')

# ── Download progress monitor (background thread) ──
import threading, time as _time_mod
_progress_stop = threading.Event()
def _progress_monitor(watch_dir, interval=10):
    \"\"\"Periodically log total download size so kubectl logs shows progress.\"\"\"
    last_size = 0
    while not _progress_stop.is_set():
        _progress_stop.wait(interval)
        if _progress_stop.is_set():
            break
        total = sum(f.stat().st_size for f in Path(watch_dir).rglob('*') if f.is_file())
        speed = (total - last_size) / interval / (1024**2)
        log.info(f'Download progress: {total/(1024**3):.2f} GB ({speed:.1f} MB/s)')
        last_size = total

# ── Patch hf-transfer concurrency (no env var exposed by default) ──
_hf_concurrency = int(os.environ.get('HF_TRANSFER_CONCURRENCY', '32'))
try:
    import hf_transfer as _hft
    _orig_dl = _hft.download
    def _patched_dl(*a, **kw):
        kw['max_files'] = _hf_concurrency
        return _orig_dl(*a, **kw)
    _hft.download = _patched_dl
    log.info(f'hf-transfer concurrency per file: {_hf_concurrency}')
except Exception:
    pass

source   = os.environ.get('MODEL_SOURCE', '')
target   = Path(os.environ.get('MODEL_DIR', '/models'))
endpoint = os.environ.get('S3_ENDPOINT', '')
bucket   = os.environ.get('S3_BUCKET', 'models')
ak       = os.environ.get('S3_ACCESS_KEY', 'minioadmin')
sk       = os.environ.get('S3_SECRET_KEY', 'minioadmin')

# File filter for selective download (e.g. '*Q4_K_M*' for GGUF repos with multiple quants)
_allow_raw = os.environ.get('ALLOW_PATTERNS', '')
allow_patterns = [p.strip() for p in _allow_raw.split(',') if p.strip()] or None
if allow_patterns:
    log.info(f'File filter: {allow_patterns}')

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
    import time as _time
    from huggingface_hub import snapshot_download
    import shutil
    MAX_RETRIES = int(os.environ.get('HF_DOWNLOAD_RETRIES', '5'))
    RETRY_DELAY = int(os.environ.get('HF_RETRY_DELAY', '30'))
    MAX_WORKERS = int(os.environ.get('HF_MAX_WORKERS', '3'))
    log.info(f'Download config: max_workers={MAX_WORKERS}, retries={MAX_RETRIES}, retry_delay={RETRY_DELAY}s')
    # If HF_HOME is set to model dir, use default cache layout (for TEI compatibility)
    hf_home = os.environ.get('HF_HOME', '')
    if hf_home and hf_home == str(target):
        cache_dir = target / 'hub'
        marker = cache_dir / ('models--' + source.replace('/', '--'))
        if marker.exists() and any(marker.rglob('*')):
            log.info(f'Using HF cache: {marker}')
            return
        log.info(f'Downloading from HuggingFace (cache mode): {source}')
        _pt = threading.Thread(target=_progress_monitor, args=(str(cache_dir),), daemon=True)
        _pt.start()
        for attempt in range(1, MAX_RETRIES + 1):
            try:
                snapshot_download(repo_id=source, cache_dir=str(cache_dir), max_workers=MAX_WORKERS, allow_patterns=allow_patterns)
                _progress_stop.set()
                return
            except Exception as e:
                if attempt == MAX_RETRIES:
                    _progress_stop.set()
                    raise
                log.warning(f'Download attempt {attempt}/{MAX_RETRIES} failed: {e}')
                log.info(f'Retrying in {RETRY_DELAY}s... (snapshot_download resumes partial files)')
                _time.sleep(RETRY_DELAY)
    else:
        # Check if model files already complete (has a large file >10MB)
        if local_dir.exists() and any(f.stat().st_size > 10_000_000 for f in local_dir.rglob('*') if f.is_file() and not f.name.startswith('.')):
            log.info(f'Using PVC cache: {local_dir}')
            return
        log.info(f'Downloading from HuggingFace: {source}')
        # Download to HF cache first, then copy to local_dir
        # This avoids the symlink/pointer issue with newer huggingface_hub
        cache_dir = target / '.hf_cache'
        _pt = threading.Thread(target=_progress_monitor, args=(str(cache_dir),), daemon=True)
        _pt.start()
        for attempt in range(1, MAX_RETRIES + 1):
            try:
                snap_path = snapshot_download(repo_id=source, cache_dir=str(cache_dir), max_workers=MAX_WORKERS, allow_patterns=allow_patterns)
                break
            except Exception as e:
                if attempt == MAX_RETRIES:
                    raise
                log.warning(f'Download attempt {attempt}/{MAX_RETRIES} failed: {e}')
                log.info(f'Retrying in {RETRY_DELAY}s... (snapshot_download resumes partial files)')
                _time.sleep(RETRY_DELAY)
        _progress_stop.set()
        log.info(f'Downloaded to cache: {snap_path}')
        local_dir.mkdir(parents=True, exist_ok=True)
        # Copy real files from snapshot to local_dir
        snap = Path(snap_path)
        for f in snap.rglob('*'):
            if not f.is_file(): continue
            rel = f.relative_to(snap)
            dst = local_dir / rel
            dst.parent.mkdir(parents=True, exist_ok=True)
            # Resolve symlinks to get the real file
            real_f = f.resolve()
            if dst.exists() and dst.stat().st_size == real_f.stat().st_size:
                continue
            shutil.copy2(str(real_f), str(dst))
            log.info(f'  Copied: {rel} ({real_f.stat().st_size/(1024**2):.1f} MB)')
        # Clean up cache to save space
        shutil.rmtree(str(cache_dir), ignore_errors=True)
        log.info(f'Model ready at {local_dir}')

def upload_to_minio():
    c = mc()
    if c is None:
        return
    log.info(f'Uploading to MinIO s3://{bucket}/{s3_prefix()}...')
    count = 0
    for fpath in local_dir.rglob('*'):
        if not fpath.is_file():
            continue
        rel = str(fpath.relative_to(local_dir))
        # Skip cache/metadata dirs and lock/incomplete files
        if rel.startswith('.cache') or rel.startswith('.hf_cache') or rel.endswith('.lock') or '.incomplete' in rel:
            continue
        obj_name = s3_prefix() + rel
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
Call with: include "kube-llmops.modelLoaderEnv" (dict "model" $model "root" $ "mountPath" "/models")
Optional: "hfHome" to override HF_HOME (set different from mountPath to force direct-mode download).
vLLM should use hfHome="/models/huggingface" so model-loader downloads to /models/<slug>/ directly.
TEI should omit hfHome (defaults to mountPath) to use HF cache layout that TEI expects.
*/}}
{{- define "kube-llmops.modelLoaderEnv" -}}
- name: MODEL_SOURCE
  value: {{ .model.source | quote }}
- name: MODEL_DIR
  value: {{ .mountPath | default "/models" | quote }}
- name: HF_HOME
  value: {{ .hfHome | default (.mountPath | default "/models") | quote }}
- name: HF_HUB_ENABLE_HF_TRANSFER
  value: "1"
- name: HF_TRANSFER_CONCURRENCY
  value: {{ (default dict (default dict .root.Values.global).modelStore).hfTransferConcurrency | default "32" | quote }}
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
{{- if .model.allowPatterns }}
- name: ALLOW_PATTERNS
  value: {{ .model.allowPatterns | quote }}
{{- end }}
- name: HF_HUB_DISABLE_XET
  value: "1"
{{- end -}}

{{/*
GPU resource name based on global.accelerator.
Usage: {{ include "kube-llmops.gpuResourceName" . }}
Output: "nvidia.com/gpu", "amd.com/gpu", or "habana.ai/gaudi"
*/}}
{{- define "kube-llmops.gpuResourceName" -}}
{{- $accel := (default dict .Values.global).accelerator | default "nvidia" -}}
{{- if eq $accel "amd" -}}
amd.com/gpu
{{- else if eq $accel "gaudi" -}}
habana.ai/gaudi
{{- else -}}
nvidia.com/gpu
{{- end -}}
{{- end -}}

{{/*
GPU toleration key based on global.accelerator.
Usage: {{ include "kube-llmops.gpuTolerationKey" . }}
*/}}
{{- define "kube-llmops.gpuTolerationKey" -}}
{{- $accel := (default dict .Values.global).accelerator | default "nvidia" -}}
{{- if eq $accel "amd" -}}
amd.com/gpu
{{- else if eq $accel "gaudi" -}}
habana.ai/gaudi
{{- else -}}
nvidia.com/gpu
{{- end -}}
{{- end -}}
