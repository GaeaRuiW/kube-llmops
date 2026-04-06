{{- define "dashboard.fullname" -}}
{{ .Release.Name }}-dashboard
{{- end }}

{{- define "dashboard.labels" -}}
app.kubernetes.io/name: dashboard
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: dashboard
app.kubernetes.io/part-of: kube-llmops
{{- end }}

{{- define "dashboard.selectorLabels" -}}
app.kubernetes.io/name: dashboard
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
