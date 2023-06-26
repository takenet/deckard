{{/*
Expand the name of the chart.
*/}}
{{- define "deckard.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 50 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 50 chars because some Kubernetes name fields are limited to 63 (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "deckard.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 50 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "deckard.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 50 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "deckard.selectorLabels" -}}
app.kubernetes.io/name: {{ include "deckard.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "deckard.labels" -}}
helm.sh/chart: {{ include "deckard.chart" . }}
{{ include "deckard.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.labels }}
{{ toYaml .Values.labels }}
{{- end }}
{{- end }}

{{/*
Housekeeper fullname
*/}}
{{- define "deckard.housekeeper.fullname" -}}
{{- printf "%s-housekeeper" (include "deckard.fullname" .) }}
{{- end }}

{{/*
Housekeeper selector labels
*/}}
{{- define "deckard.housekeeper.selectorLabels" -}}
app.kubernetes.io/name: {{ include "deckard.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-housekeeper
{{- end }}

{{/*
Housekeeper common labels
*/}}
{{- define "deckard.housekeeper.labels" -}}
helm.sh/chart: {{ include "deckard.chart" . }}
{{ include "deckard.housekeeper.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.housekeeper.labels }}
{{ toYaml .Values.housekeeper.labels }}
{{- end }}
{{- end }}
