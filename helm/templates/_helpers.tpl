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

{{/*
Redis subchart fullname
*/}}
{{- define "deckard.redis.fullname" -}}
{{- if .Values.redis.fullnameOverride }}
{{- .Values.redis.fullnameOverride | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- $name := default "redis" .Values.redis.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 50 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
MongoDB subchart fullname
*/}}
{{- define "deckard.mongodb.fullname" -}}
{{- if .Values.mongodb.fullnameOverride }}
{{- .Values.mongodb.fullnameOverride | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- $name := default "mongodb" .Values.mongodb.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 50 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 50 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Define Cache URI
*/}}
{{- define "deckard.cache.uri" -}}
{{- if eq .Values.cache.type "REDIS" }}
{{- if .Values.redis.enabled }}
{{- if eq .Values.redis.architecture "standalone" }}
{{- printf "redis://:%s@%s-master.%s.svc:%s/%s" .Values.redis.auth.password (include "deckard.redis.fullname" .) .Release.Namespace (toString .Values.redis.service.ports.redis) (toString .Values.cache.redis.database) }}
{{- else }}
{{- printf "redis://:%s@%s-headless.%s.svc:%s/%s" .Values.redis.auth.password (include "deckard.redis.fullname" .) .Release.Namespace (toString .Values.redis.service.ports.redis) (toString .Values.cache.redis.database) }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Storage connection secret name
*/}}
{{- define "deckard.storageConnectionSecretName" -}}
{{- if .Values.connectionSecret.storage.existingSecret }}
{{- .Values.connectionSecret.storage.existingSecret -}}
{{- else }}
{{- printf "%s-storage" (include "deckard.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Cache connection secret name
*/}}
{{- define "deckard.cacheConnectionSecretName" -}}
{{- if .Values.connectionSecret.cache.existingSecret }}
{{- .Values.connectionSecret.cache.existingSecret -}}
{{- else }}
{{- printf "%s-cache" (include "deckard.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Whether the chart should create the storage connection secret
*/}}
{{- define "deckard.shouldCreateStorageConnectionSecret" -}}
{{- if .Values.connectionSecret.storage.existingSecret -}}
false
{{- else if ne (include "deckard.storage.uri" .) "" -}}
true
{{- else -}}
false
{{- end }}
{{- end }}

{{/*
Whether the chart should create the cache connection secret
*/}}
{{- define "deckard.shouldCreateCacheConnectionSecret" -}}
{{- if .Values.connectionSecret.cache.existingSecret -}}
false
{{- else if ne (include "deckard.cache.uri" .) "" -}}
true
{{- else -}}
false
{{- end }}
{{- end }}

{{/*
Whether storage URI env should be configured
*/}}
{{- define "deckard.shouldSetStorageURI" -}}
{{- if eq .Values.storage.type "MONGODB" -}}
{{- if or (ne .Values.connectionSecret.storage.existingSecret "") (and .Values.mongodb.enabled (ne (include "deckard.storage.uri" .) "")) -}}
true
{{- else -}}
false
{{- end -}}
{{- else -}}
false
{{- end -}}
{{- end }}

{{/*
Whether cache URI env should be configured
*/}}
{{- define "deckard.shouldSetCacheURI" -}}
{{- if eq .Values.cache.type "REDIS" -}}
{{- if or (ne .Values.connectionSecret.cache.existingSecret "") (and .Values.redis.enabled (ne (include "deckard.cache.uri" .) "")) -}}
true
{{- else -}}
false
{{- end -}}
{{- else -}}
false
{{- end -}}
{{- end }}

{{/*
Define Storage URI
*/}}
{{- define "deckard.storage.uri" -}}
{{- if eq .Values.storage.type "MONGODB" }}
{{- if .Values.mongodb.enabled }}
{{- if eq .Values.mongodb.architecture "standalone" }}
{{- printf "mongodb://%s:%s@%s.%s.svc:%s/admin?ssl=false" .Values.mongodb.auth.rootUser .Values.mongodb.auth.rootPassword (include "deckard.mongodb.fullname" .) .Release.Namespace (toString .Values.mongodb.service.ports.mongodb) }}
{{- else }}
{{- printf "mongodb://%s:%s@%s-headless.%s.svc:%s/admin?ssl=false" .Values.mongodb.auth.rootUser .Values.mongodb.auth.rootPassword (include "deckard.mongodb.fullname" .) .Release.Namespace (toString .Values.mongodb.service.ports.mongodb) }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
