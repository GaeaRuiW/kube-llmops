{{/*
SGLang subchart helpers - engine resolution is in parent _helpers.tpl
*/}}

{{- define "sglang.name" -}}
{{- printf "sglang-%s" .modelName | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "sglang.labels" -}}
app.kubernetes.io/name: sglang
app.kubernetes.io/instance: {{ .releaseName }}
app.kubernetes.io/component: model-serving
app.kubernetes.io/part-of: kube-llmops
kube-llmops/model: {{ .modelName }}
kube-llmops/engine: sglang
{{- end }}

{{- define "sglang.selectorLabels" -}}
app.kubernetes.io/name: sglang
app.kubernetes.io/instance: {{ .releaseName }}
kube-llmops/model: {{ .modelName }}
{{- end }}
