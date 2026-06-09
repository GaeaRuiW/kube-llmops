{{/*
Name of the Secret that stores Dify Plugin Daemon credentials.
*/}}
{{- define "dify.pluginDaemonSecretName" -}}
{{- .Values.pluginDaemon.existingSecret | default (printf "%s-dify-plugin-daemon" .Release.Name) -}}
{{- end -}}

{{/*
Secret key names for Plugin Daemon credentials.
*/}}
{{- define "dify.pluginDaemonKeySecretKey" -}}
{{- .Values.pluginDaemon.secretKeys.key | default "plugin-daemon-key" -}}
{{- end -}}

{{- define "dify.pluginDaemonInnerApiKeySecretKey" -}}
{{- .Values.pluginDaemon.secretKeys.innerApiKey | default "plugin-daemon-inner-api-key" -}}
{{- end -}}
