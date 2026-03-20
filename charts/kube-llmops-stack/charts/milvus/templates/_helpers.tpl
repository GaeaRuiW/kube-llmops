{{/*
Milvus chart helpers
*/}}

{{- define "milvus.fullname" -}}
{{- printf "%s-milvus" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "milvus.labels" -}}
app.kubernetes.io/name: milvus
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: vector-database
app.kubernetes.io/part-of: kube-llmops
{{- end }}

{{- define "milvus.selectorLabels" -}}
app.kubernetes.io/name: milvus
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
