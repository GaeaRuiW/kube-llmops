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
