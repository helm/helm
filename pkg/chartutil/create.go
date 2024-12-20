/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chartutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// chartName is a regular expression for testing the supplied name of a chart.
// This regular expression is probably stricter than it needs to be. We can relax it
// somewhat. Newline characters, as well as $, quotes, +, parens, and % are known to be
// problematic.
var chartName = regexp.MustCompile("^[a-zA-Z0-9._-]+$")

const (
	// ChartfileName is the default Chart file name.
	ChartfileName = "Chart.yaml"
	// ValuesfileName is the default values file name.
	ValuesfileName = "values.yaml"
	// SchemafileName is the default values schema file name.
	SchemafileName = "values.schema.json"
	// TemplatesDir is the relative directory name for templates.
	TemplatesDir = "templates"
	// ChartsDir is the relative directory name for charts dependencies.
	ChartsDir = "charts"
	// TemplatesTestsDir is the relative directory name for tests.
	TemplatesTestsDir = TemplatesDir + sep + "tests"
	// IgnorefileName is the name of the Helm ignore file.
	IgnorefileName = ".helmignore"
	// IngressFileName is the name of the example ingress file.
	IngressFileName = TemplatesDir + sep + "ingress.yaml"
	// DeploymentName is the name of the example deployment file.
	DeploymentName = TemplatesDir + sep + "deployment.yaml"
	// ServiceName is the name of the example service file.
	ServiceName = TemplatesDir + sep + "service.yaml"
	// ServiceAccountName is the name of the example serviceaccount file.
	ServiceAccountName = TemplatesDir + sep + "serviceaccount.yaml"
	// HorizontalPodAutoscalerName is the name of the example hpa file.
	HorizontalPodAutoscalerName = TemplatesDir + sep + "hpa.yaml"
	// NotesName is the name of the example NOTES.txt file.
	NotesName = TemplatesDir + sep + "NOTES.txt"
	// HelpersName is the name of the example helpers file.
	HelpersName = TemplatesDir + sep + "_helpers.tpl"
	// TestConnectionName is the name of the example test file.
	TestConnectionName = TemplatesTestsDir + sep + "test-connection.yaml"
)

// maxChartNameLength is lower than the limits we know of with certain file systems,
// and with certain Kubernetes fields.
const maxChartNameLength = 250

const sep = string(filepath.Separator)

const defaultChartfile = `apiVersion: v2
name: %s
description: A Helm chart for Kubernetes

# A chart can be either an 'application' or a 'library' chart.
#
# Application charts are a collection of templates that can be packaged into versioned archives
# to be deployed.
#
# Library charts provide useful utilities or functions for the chart developer. They're included as
# a dependency of application charts to inject those utilities and functions into the rendering
# pipeline. Library charts do not define any templates and therefore cannot be deployed.
type: application

# This is the chart version. This version number should be incremented each time you make changes
# to the chart and its templates, including the app version.
# Versions are expected to follow Semantic Versioning (https://semver.org/)
version: 0.1.0

# This is the version number of the application being deployed. This version number should be
# incremented each time you make changes to the application. Versions are not expected to
# follow Semantic Versioning. They should reflect the version the application is using.
# It is recommended to use it with quotes.
appVersion: "1.16.0"
`

const defaultValues = `# Default values for %s.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

# This will set the replicaset count more information can be found here: https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/
replicaCount: 1

# This sets the container image more information can be found here: https://kubernetes.io/docs/concepts/containers/images/
image:
  repository: nginx
  # This sets the pull policy for images.
  pullPolicy: IfNotPresent
  # Overrides the image tag whose default is the chart appVersion.
  tag: ""

# This is for the secrets for pulling an image from a private repository more information can be found here: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
imagePullSecrets: []
# This is to override the chart name.
nameOverride: ""
fullnameOverride: ""

# This section builds out the service account more information can be found here: https://kubernetes.io/docs/concepts/security/service-accounts/
serviceAccount:
  # Specifies whether a service account should be created
  create: true
  # Automatically mount a ServiceAccount's API credentials?
  automount: true
  # Annotations to add to the service account
  annotations: {}
  # The name of the service account to use.
  # If not set and create is true, a name is generated using the fullname template
  name: ""

# This is for setting Kubernetes Annotations to a Pod.
# For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
podAnnotations: {}
# This is for setting Kubernetes Labels to a Pod.
# For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
podLabels: {}

podSecurityContext: {}
  # fsGroup: 2000

securityContext: {}
  # capabilities:
  #   drop:
  #   - ALL
  # readOnlyRootFilesystem: true
  # runAsNonRoot: true
  # runAsUser: 1000

# This is for setting up a service more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/
service:
  # This sets the service type more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
  type: ClusterIP
  # This sets the ports more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports
  port: 80

# This block is for setting up the ingress for more information can be found here: https://kubernetes.io/docs/concepts/services-networking/ingress/
ingress:
  enabled: false
  className: ""
  annotations: {}
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  hosts:
    - host: chart-example.local
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls: []
  #  - secretName: chart-example-tls
  #    hosts:
  #      - chart-example.local

resources: {}
  # We usually recommend not to specify default resources and to leave this as a conscious
  # choice for the user. This also increases chances charts run on environments with little
  # resources, such as Minikube. If you do want to specify resources, uncomment the following
  # lines, adjust them as necessary, and remove the curly braces after 'resources:'.
  # limits:
  #   cpu: 100m
  #   memory: 128Mi
  # requests:
  #   cpu: 100m
  #   memory: 128Mi

# This is to setup the liveness and readiness probes more information can be found here: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
livenessProbe:
  httpGet:
    path: /
    port: http
readinessProbe:
  httpGet:
    path: /
    port: http

# This section is for setting up autoscaling more information can be found here: https://kubernetes.io/docs/concepts/workloads/autoscaling/
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 100
  targetCPUUtilizationPercentage: 80
  # targetMemoryUtilizationPercentage: 80

# Additional volumes on the output Deployment definition.
volumes: []
# - name: foo
#   secret:
#     secretName: mysecret
#     optional: false

# Additional volumeMounts on the output Deployment definition.
volumeMounts: []
# - name: foo
#   mountPath: "/etc/foo"
#   readOnly: true

nodeSelector: {}

tolerations: []

affinity: {}
`

const defaultIgnore = `# Patterns to ignore when building packages.
# This supports shell glob matching, relative path matching, and
# negation (prefixed with !). Only one pattern per line.
.DS_Store
# Common VCS dirs
.git/
.gitignore
.bzr/
.bzrignore
.hg/
.hgignore
.svn/
# Common backup files
*.swp
*.bak
*.tmp
*.orig
*~
# Various IDEs
.project
.idea/
*.tmproj
.vscode/
`

const defaultIngress = `{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
  {{- with .Values.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- with .Values.ingress.className }}
  ingressClassName: {{ . }}
  {{- end }}
  {{- if .Values.ingress.tls }}
  tls:
    {{- range .Values.ingress.tls }}
    - hosts:
        {{- range .hosts }}
        - {{ . | quote }}
        {{- end }}
      secretName: {{ .secretName }}
    {{- end }}
  {{- end }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            {{- with .pathType }}
            pathType: {{ . }}
            {{- end }}
            backend:
              service:
                name: {{ include "<CHARTNAME>.fullname" $ }}
                port:
                  number: {{ $.Values.service.port }}
          {{- end }}
    {{- end }}
{{- end }}
`

const defaultDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "<CHARTNAME>.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        {{- include "<CHARTNAME>.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "<CHARTNAME>.serviceAccountName" . }}
      {{- with .Values.podSecurityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: {{ .Chart.Name }}
          {{- with .Values.securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.port }}
              protocol: TCP
          {{- with .Values.livenessProbe }}
          livenessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.readinessProbe }}
          readinessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.volumeMounts }}
          volumeMounts:
            {{- toYaml . | nindent 12 }}
          {{- end }}
      {{- with .Values.volumes }}
      volumes:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
`

const defaultService = `apiVersion: v1
kind: Service
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    {{- include "<CHARTNAME>.selectorLabels" . | nindent 4 }}
`

const defaultServiceAccount = `{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "<CHARTNAME>.serviceAccountName" . }}
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: {{ .Values.serviceAccount.automount }}
{{- end }}
`

const defaultHorizontalPodAutoscaler = `{{- if .Values.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "<CHARTNAME>.fullname" . }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.maxReplicas }}
  metrics:
    {{- if .Values.autoscaling.targetCPUUtilizationPercentage }}
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetCPUUtilizationPercentage }}
    {{- end }}
    {{- if .Values.autoscaling.targetMemoryUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetMemoryUtilizationPercentage }}
    {{- end }}
{{- end }}
`

const defaultNotes = `1. Get the application URL by running these commands:
{{- if .Values.ingress.enabled }}
{{- range $host := .Values.ingress.hosts }}
  {{- range .paths }}
  http{{ if $.Values.ingress.tls }}s{{ end }}://{{ $host.host }}{{ .path }}
  {{- end }}
{{- end }}
{{- else if contains "NodePort" .Values.service.type }}
  export NODE_PORT=$(kubectl get --namespace {{ .Release.Namespace }} -o jsonpath="{.spec.ports[0].nodePort}" services {{ include "<CHARTNAME>.fullname" . }})
  export NODE_IP=$(kubectl get nodes --namespace {{ .Release.Namespace }} -o jsonpath="{.items[0].status.addresses[0].address}")
  echo http://$NODE_IP:$NODE_PORT
{{- else if contains "LoadBalancer" .Values.service.type }}
     NOTE: It may take a few minutes for the LoadBalancer IP to be available.
           You can watch its status by running 'kubectl get --namespace {{ .Release.Namespace }} svc -w {{ include "<CHARTNAME>.fullname" . }}'
  export SERVICE_IP=$(kubectl get svc --namespace {{ .Release.Namespace }} {{ include "<CHARTNAME>.fullname" . }} --template "{{"{{ range (index .status.loadBalancer.ingress 0) }}{{.}}{{ end }}"}}")
  echo http://$SERVICE_IP:{{ .Values.service.port }}
{{- else if contains "ClusterIP" .Values.service.type }}
  export POD_NAME=$(kubectl get pods --namespace {{ .Release.Namespace }} -l "app.kubernetes.io/name={{ include "<CHARTNAME>.name" . }},app.kubernetes.io/instance={{ .Release.Name }}" -o jsonpath="{.items[0].metadata.name}")
  export CONTAINER_PORT=$(kubectl get pod --namespace {{ .Release.Namespace }} $POD_NAME -o jsonpath="{.spec.containers[0].ports[0].containerPort}")
  echo "Visit http://127.0.0.1:8080 to use your application"
  kubectl --namespace {{ .Release.Namespace }} port-forward $POD_NAME 8080:$CONTAINER_PORT
{{- end }}
`

const defaultHelpers = `{{/*
Expand the name of the chart.
*/}}
{{- define "<CHARTNAME>.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "<CHARTNAME>.fullname" -}}
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
{{- define "<CHARTNAME>.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "<CHARTNAME>.labels" -}}
helm.sh/chart: {{ include "<CHARTNAME>.chart" . }}
{{ include "<CHARTNAME>.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "<CHARTNAME>.selectorLabels" -}}
app.kubernetes.io/name: {{ include "<CHARTNAME>.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "<CHARTNAME>.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "<CHARTNAME>.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
`

const defaultTestConnection = `apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "<CHARTNAME>.fullname" . }}-test-connection"
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": test
spec:
  containers:
    - name: wget
      image: busybox
      command: ['wget']
      args: ['{{ include "<CHARTNAME>.fullname" . }}:{{ .Values.service.port }}']
  restartPolicy: Never
`

// Stderr is an io.Writer to which error messages can be written
//
// In Helm 4, this will be replaced. It is needed in Helm 3 to preserve API backward
// compatibility.
var Stderr io.Writer = os.Stderr

// CreateFrom creates a new chart, but scaffolds it from the src chart.
func CreateFrom(chartfile *chart.Metadata, dest, src string) error {
	schart, err := loader.Load(src)
	if err != nil {
		return errors.Wrapf(err, "could not load %s", src)
	}

	schart.Metadata = chartfile

	var updatedTemplates []*chart.File

	for _, template := range schart.Templates {
		newData := transform(string(template.Data), schart.Name())
		updatedTemplates = append(updatedTemplates, &chart.File{Name: template.Name, Data: newData})
	}

	schart.Templates = updatedTemplates
	b, err := yaml.Marshal(schart.Values)
	if err != nil {
		return errors.Wrap(err, "reading values file")
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(transform(string(b), schart.Name()), &m); err != nil {
		return errors.Wrap(err, "transforming values file")
	}
	schart.Values = m

	// SaveDir looks for the file values.yaml when saving rather than the values
	// key in order to preserve the comments in the YAML. The name placeholder
	// needs to be replaced on that file.
	for _, f := range schart.Raw {
		if f.Name == ValuesfileName {
			f.Data = transform(string(f.Data), schart.Name())
		}
	}

	return SaveDir(schart, dest)
}

// Create creates a new chart in a directory.
//
// Inside of dir, this will create a directory based on the name of
// chartfile.Name. It will then write the Chart.yaml into this directory and
// create the (empty) appropriate directories.
//
// The returned string will point to the newly created directory. It will be
// an absolute path, even if the provided base directory was relative.
//
// If dir does not exist, this will return an error.
// If Chart.yaml or any directories cannot be created, this will return an
// error. In such a case, this will attempt to clean up by removing the
// new chart directory.
func Create(name, dir string) (string, error) {

	// Sanity-check the name of a chart so user doesn't create one that causes problems.
	if err := validateChartName(name); err != nil {
		return "", err
	}

	path, err := filepath.Abs(dir)
	if err != nil {
		return path, err
	}

	if fi, err := os.Stat(path); err != nil {
		return path, err
	} else if !fi.IsDir() {
		return path, errors.Errorf("no such directory %s", path)
	}

	cdir := filepath.Join(path, name)
	if fi, err := os.Stat(cdir); err == nil && !fi.IsDir() {
		return cdir, errors.Errorf("file %s already exists and is not a directory", cdir)
	}

	// Note: If adding a new template below (i.e., to `helm create`) which is disabled by default (similar to hpa and
	// ingress below); or making an existing template disabled by default, add the enabling condition in
	// `TestHelmCreateChart_CheckDeprecatedWarnings` in `pkg/lint/lint_test.go` to make it run through deprecation checks
	// with latest Kubernetes version.
	files := []struct {
		path    string
		content []byte
	}{
		{
			// Chart.yaml
			path:    filepath.Join(cdir, ChartfileName),
			content: []byte(fmt.Sprintf(defaultChartfile, name)),
		},
		{
			// values.yaml
			path:    filepath.Join(cdir, ValuesfileName),
			content: []byte(fmt.Sprintf(defaultValues, name)),
		},
		{
			// .helmignore
			path:    filepath.Join(cdir, IgnorefileName),
			content: []byte(defaultIgnore),
		},
		{
			// ingress.yaml
			path:    filepath.Join(cdir, IngressFileName),
			content: transform(defaultIngress, name),
		},
		{
			// deployment.yaml
			path:    filepath.Join(cdir, DeploymentName),
			content: transform(defaultDeployment, name),
		},
		{
			// service.yaml
			path:    filepath.Join(cdir, ServiceName),
			content: transform(defaultService, name),
		},
		{
			// serviceaccount.yaml
			path:    filepath.Join(cdir, ServiceAccountName),
			content: transform(defaultServiceAccount, name),
		},
		{
			// hpa.yaml
			path:    filepath.Join(cdir, HorizontalPodAutoscalerName),
			content: transform(defaultHorizontalPodAutoscaler, name),
		},
		{
			// NOTES.txt
			path:    filepath.Join(cdir, NotesName),
			content: transform(defaultNotes, name),
		},
		{
			// _helpers.tpl
			path:    filepath.Join(cdir, HelpersName),
			content: transform(defaultHelpers, name),
		},
		{
			// test-connection.yaml
			path:    filepath.Join(cdir, TestConnectionName),
			content: transform(defaultTestConnection, name),
		},
	}

	for _, file := range files {
		if _, err := os.Stat(file.path); err == nil {
			// There is no handle to a preferred output stream here.
			fmt.Fprintf(Stderr, "WARNING: File %q already exists. Overwriting.\n", file.path)
		}
		if err := writeFile(file.path, file.content); err != nil {
			return cdir, err
		}
	}
	// Need to add the ChartsDir explicitly as it does not contain any file OOTB
	if err := os.MkdirAll(filepath.Join(cdir, ChartsDir), 0755); err != nil {
		return cdir, err
	}
	return cdir, nil
}

// transform performs a string replacement of the specified source for
// a given key with the replacement string
func transform(src, replacement string) []byte {
	return []byte(strings.ReplaceAll(src, "<CHARTNAME>", replacement))
}

func writeFile(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	return os.WriteFile(name, content, 0644)
}

func validateChartName(name string) error {
	if name == "" || len(name) > maxChartNameLength {
		return fmt.Errorf("chart name must be between 1 and %d characters", maxChartNameLength)
	}
	if !chartName.MatchString(name) {
		return fmt.Errorf("chart name must match the regular expression %q", chartName.String())
	}
	return nil
}
