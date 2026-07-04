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
{{- define "deckard.dependency.fullname" -}}
{{- $chartValues := get . "chartValues" -}}
{{- $chartName := get . "chartName" -}}
{{- $context := get . "context" -}}
{{- $fullnameOverride := get $chartValues "fullnameOverride" -}}
{{- $nameOverride := get $chartValues "nameOverride" -}}
{{- if $fullnameOverride -}}
{{- $fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default $chartName $nameOverride -}}
{{- if contains $name $context.Release.Name -}}
{{- $context.Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" $context.Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Redis subchart fullname
*/}}
{{- define "deckard.redis.fullname" -}}
{{- include "deckard.dependency.fullname" (dict "chartName" "redis" "chartValues" .Values.redis "context" .) -}}
{{- end }}

{{/*
MongoDB subchart fullname
*/}}
{{- define "deckard.mongodb.fullname" -}}
{{- include "deckard.dependency.fullname" (dict "chartName" "mongodb" "chartValues" .Values.mongodb "context" .) -}}
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
Storage connection secret existingSecret
*/}}
{{- define "deckard.storageConnectionSecretExistingSecret" -}}
{{- $storageConnectionSecret := get .Values.storage "connectionSecret" | default dict -}}
{{- $existingSecret := get $storageConnectionSecret "existingSecret" -}}
{{- if $existingSecret -}}
{{- $existingSecret -}}
{{- else -}}
{{- $legacyConnectionSecret := get .Values.connectionSecret "storage" | default dict -}}
{{- get $legacyConnectionSecret "existingSecret" -}}
{{- end -}}
{{- end }}

{{/*
Storage connection secret key
*/}}
{{- define "deckard.storageConnectionSecretKey" -}}
{{- $storageConnectionSecret := get .Values.storage "connectionSecret" | default dict -}}
{{- $secretKey := get $storageConnectionSecret "key" -}}
{{- $legacyConnectionSecret := get .Values.connectionSecret "storage" | default dict -}}
{{- $legacySecretKey := get $legacyConnectionSecret "key" -}}
{{- $defaultSecretKey := "storage-uri" -}}
{{- if and $legacySecretKey (eq $secretKey $defaultSecretKey) (ne $legacySecretKey $defaultSecretKey) -}}
{{- $legacySecretKey -}}
{{- else if $secretKey -}}
{{- $secretKey -}}
{{- else -}}
{{- $legacySecretKey -}}
{{- end -}}
{{- end }}

{{/*
Cache connection secret existingSecret
*/}}
{{- define "deckard.cacheConnectionSecretExistingSecret" -}}
{{- $cacheConnectionSecret := get .Values.cache "connectionSecret" | default dict -}}
{{- $existingSecret := get $cacheConnectionSecret "existingSecret" -}}
{{- if $existingSecret -}}
{{- $existingSecret -}}
{{- else -}}
{{- $legacyConnectionSecret := get .Values.connectionSecret "cache" | default dict -}}
{{- get $legacyConnectionSecret "existingSecret" -}}
{{- end -}}
{{- end }}

{{/*
Cache connection secret key
*/}}
{{- define "deckard.cacheConnectionSecretKey" -}}
{{- $cacheConnectionSecret := get .Values.cache "connectionSecret" | default dict -}}
{{- $secretKey := get $cacheConnectionSecret "key" -}}
{{- $legacyConnectionSecret := get .Values.connectionSecret "cache" | default dict -}}
{{- $legacySecretKey := get $legacyConnectionSecret "key" -}}
{{- $defaultSecretKey := "cache-uri" -}}
{{- if and $legacySecretKey (eq $secretKey $defaultSecretKey) (ne $legacySecretKey $defaultSecretKey) -}}
{{- $legacySecretKey -}}
{{- else if $secretKey -}}
{{- $secretKey -}}
{{- else -}}
{{- $legacySecretKey -}}
{{- end -}}
{{- end }}

{{/*
Storage connection secret name
*/}}
{{- define "deckard.storageConnectionSecretName" -}}
{{- $existingSecret := include "deckard.storageConnectionSecretExistingSecret" . -}}
{{- if $existingSecret }}
{{- $existingSecret -}}
{{- else }}
{{- printf "%s-storage" (include "deckard.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Cache connection secret name
*/}}
{{- define "deckard.cacheConnectionSecretName" -}}
{{- $existingSecret := include "deckard.cacheConnectionSecretExistingSecret" . -}}
{{- if $existingSecret }}
{{- $existingSecret -}}
{{- else }}
{{- printf "%s-cache" (include "deckard.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Whether the chart should create the storage connection secret
*/}}
{{- define "deckard.shouldCreateStorageConnectionSecret" -}}
{{- if include "deckard.storageConnectionSecretExistingSecret" . -}}
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
{{- if include "deckard.cacheConnectionSecretExistingSecret" . -}}
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
{{- if or (ne (include "deckard.storageConnectionSecretExistingSecret" .) "") (and .Values.mongodb.enabled (ne (include "deckard.storage.uri" .) "")) -}}
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
{{- if or (ne (include "deckard.cacheConnectionSecretExistingSecret" .) "") (and .Values.redis.enabled (ne (include "deckard.cache.uri" .) "")) -}}
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
