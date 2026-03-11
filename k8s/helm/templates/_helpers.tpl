{{/*
Expand the name of the chart.
*/}}
{{- define "aidevops-platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aidevops-platform.fullname" -}}
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
Common labels
*/}}
{{- define "aidevops-platform.labels" -}}
helm.sh/chart: {{ include "aidevops-platform.name" . }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{ include "aidevops-platform.selectorLabels" . }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aidevops-platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aidevops-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Image helper: returns full image reference
Usage: {{ include "aidevops-platform.image" (dict "registry" .Values.global.registry "name" .Values.operator.image "tag" .Values.global.tag) }}
*/}}
{{- define "aidevops-platform.image" -}}
{{- printf "%s/%s:%s" .registry .name .tag }}
{{- end }}
