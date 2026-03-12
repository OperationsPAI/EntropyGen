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

{{/*
wait-for-deps: generates initContainers that block until each TCP endpoint is reachable.
Usage:
  initContainers:
    {{- include "aidevops-platform.wait-for-deps" (dict "deps" (list
      (dict "name" "redis"      "addr" .Values.redis.addr)
      (dict "name" "clickhouse" "addr" .Values.clickhouse.addr)
    )) | nindent 8 }}

Each dep needs:
  - name: human-readable name (used as initContainer name: wait-for-<name>)
  - addr: "host:port" or "http://host:port" (http:// prefix is stripped, path ignored)
*/}}
{{- define "aidevops-platform.wait-for-deps" -}}
{{- range .deps }}
- name: wait-for-{{ .name }}
  image: busybox:1.37
  command:
  - /bin/sh
  - -c
  - |
    TARGET="{{ .addr }}"
    # Strip http(s):// prefix and /path suffix to extract host:port
    TARGET=$(echo "$TARGET" | sed -e 's|^https\?://||' -e 's|/.*||')
    HOST=$(echo "$TARGET" | cut -d: -f1)
    PORT=$(echo "$TARGET" | cut -d: -f2)
    echo "Waiting for $HOST:$PORT ..."
    while ! nc -z "$HOST" "$PORT" 2>/dev/null; do
      sleep 2
    done
    echo "$HOST:$PORT is ready"
  resources:
    requests:
      cpu: "10m"
      memory: "8Mi"
    limits:
      cpu: "50m"
      memory: "16Mi"
{{- end }}
{{- end }}
