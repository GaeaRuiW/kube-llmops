{{/*
vLLM chart helpers
*/}}

{{- define "vllm.name" -}}
{{- printf "vllm-%s" .modelName | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "vllm.labels" -}}
app.kubernetes.io/name: vllm
app.kubernetes.io/instance: {{ .releaseName }}
app.kubernetes.io/component: model-serving
app.kubernetes.io/part-of: kube-llmops
kube-llmops/model: {{ .modelName }}
kube-llmops/engine: vllm
{{- end }}

{{- define "vllm.selectorLabels" -}}
app.kubernetes.io/name: vllm
app.kubernetes.io/instance: {{ .releaseName }}
kube-llmops/model: {{ .modelName }}
{{- end }}
