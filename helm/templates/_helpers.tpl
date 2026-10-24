{{- define "yarilo-autoconfig.labels" -}}
app.kubernetes.io/name: yarilo-autoconfig
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{- define "yarilo-autoconfig.selector" -}}
app.kubernetes.io/name: yarilo-autoconfig
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
