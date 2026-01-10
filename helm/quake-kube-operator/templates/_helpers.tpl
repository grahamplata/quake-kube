{{/*
Expand the name of the chart.
*/}}
{{- define "quake-kube-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "quake-kube-operator.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "quake-kube-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "quake-kube-operator.labels" -}}
helm.sh/chart: {{ include "quake-kube-operator.chart" . }}
{{ include "quake-kube-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: controller
app.kubernetes.io/part-of: quake-kube
{{- end }}

{{/*
Selector labels
*/}}
{{- define "quake-kube-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "quake-kube-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "quake-kube-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "quake-kube-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "quake-kube-operator.webhookServiceName" -}}
{{- include "quake-kube-operator.fullname" . }}-webhook
{{- end }}

{{/*
Webhook certificate secret name
*/}}
{{- define "quake-kube-operator.webhookCertSecretName" -}}
{{- include "quake-kube-operator.fullname" . }}-webhook-cert
{{- end }}

{{/*
Operator image
*/}}
{{- define "quake-kube-operator.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
