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
	"io/ioutil"
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

const moduleNameTemplate = "<MODULE>_"

type ManifestFile struct {
	path    string
	content []byte
}

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
	IngressFileName = TemplatesDir + sep + moduleNameTemplate + "_ingress.yaml"
	// DeploymentName is the name of the example deployment file.
	DeploymentName = TemplatesDir + sep + moduleNameTemplate + "_deployment.yaml"
	// ServiceName is the name of the example service file.
	ServiceName = TemplatesDir + sep + moduleNameTemplate + "_service.yaml"
	// ServiceAccountName is the name of the example serviceaccount file.
	ServiceAccountName = TemplatesDir + sep + moduleNameTemplate + "_serviceaccount.yaml"
	// HorizontalPodAutoscalerName is the name of the example hpa file.
	HorizontalPodAutoscalerName = TemplatesDir + sep + moduleNameTemplate + "_hpa.yaml"
	// NotesName is the name of the example NOTES.txt file.
	NotesName = TemplatesDir + sep + "NOTES.txt"
	// HelpersName is the name of the example helpers file.
	HelpersName = TemplatesDir + sep + "_" + moduleNameTemplate + "_helpers.tpl"
	// TestConnectionName is the name of the example test file.
	TestConnectionName = TemplatesTestsDir + sep + moduleNameTemplate + "_test-connection.yaml"
)

func getIngressFileName(module string) string {
	return TemplatesDir + sep + moduleNameTemplate + "_ingress.yaml"
}

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

<MODULE_NAME>:
  replicaCount: 1

  image:
    repository: nginx
    pullPolicy: IfNotPresent
    # Overrides the image tag whose default is the chart appVersion.
    tag: ""

  imagePullSecrets: []
  nameOverride: ""
  fullnameOverride: ""

  serviceAccount:
    # Specifies whether a service account should be created
    create: true
    # Annotations to add to the service account
    annotations: {}
    # The name of the service account to use.
    # If not set and create is true, a name is generated using the fullname template
    name: ""

  podAnnotations: {}

  podSecurityContext: {}
    # fsGroup: 2000

  securityContext: {}
    # capabilities:
    #   drop:
    #   - ALL
    # readOnlyRootFilesystem: true
    # runAsNonRoot: true
    # runAsUser: 1000

  service:
    type: ClusterIP
    port: 80

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

  autoscaling:
    enabled: false
    minReplicas: 1
    maxReplicas: 100
    targetCPUUtilizationPercentage: 80
    # targetMemoryUtilizationPercentage: 80

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

const defaultIngress = `{{- if .Values.<MODULE_NAME>.ingress.enabled -}}
{{- $fullName := include "<MODULE_NAME>.fullname" . -}}
{{- $svcPort := .Values.<MODULE_NAME>.service.port -}}
{{- if and .Values.<MODULE_NAME>.ingress.className (not (semverCompare ">=1.18-0" .Capabilities.KubeVersion.GitVersion)) }}
  {{- if not (hasKey .Values.<MODULE_NAME>.ingress.annotations "kubernetes.io/ingress.class") }}
  {{- $_ := set .Values.<MODULE_NAME>.ingress.annotations "kubernetes.io/ingress.class" .Values.<MODULE_NAME>.ingress.className}}
  {{- end }}
{{- end }}
{{- if semverCompare ">=1.19-0" .Capabilities.KubeVersion.GitVersion -}}
apiVersion: networking.k8s.io/v1
{{- else if semverCompare ">=1.14-0" .Capabilities.KubeVersion.GitVersion -}}
apiVersion: networking.k8s.io/v1beta1
{{- else -}}
apiVersion: extensions/v1beta1
{{- end }}
kind: Ingress
metadata:
  name: {{ $fullName }}
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
  {{- with .Values.<MODULE_NAME>.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if and .Values.<MODULE_NAME>.ingress.className (semverCompare ">=1.18-0" .Capabilities.KubeVersion.GitVersion) }}
  ingressClassName: {{ .Values.<MODULE_NAME>.ingress.className }}
  {{- end }}
  {{- if .Values.<MODULE_NAME>.ingress.tls }}
  tls:
    {{- range .Values.<MODULE_NAME>.ingress.tls }}
    - hosts:
        {{- range .hosts }}
        - {{ . | quote }}
        {{- end }}
      secretName: {{ .secretName }}
    {{- end }}
  {{- end }}
  rules:
    {{- range .Values.<MODULE_NAME>.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            {{- if and .pathType (semverCompare ">=1.18-0" $.Capabilities.KubeVersion.GitVersion) }}
            pathType: {{ .pathType }}
            {{- end }}
            backend:
              {{- if semverCompare ">=1.19-0" $.Capabilities.KubeVersion.GitVersion }}
              service:
                name: {{ $fullName }}
                port:
                  number: {{ $svcPort }}
              {{- else }}
              serviceName: {{ $fullName }}
              servicePort: {{ $svcPort }}
              {{- end }}
          {{- end }}
    {{- end }}
{{- end }}
`

const defaultDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "<MODULE_NAME>.fullname" . }}
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
spec:
  {{- if not .Values.<MODULE_NAME>.autoscaling.enabled }}
  replicas: {{ .Values.<MODULE_NAME>.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "<MODULE_NAME>.name" . }}
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      {{- with .Values.<MODULE_NAME>.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        app.kubernetes.io/name: {{ include "<MODULE_NAME>.name" . }}
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      {{- with .Values.<MODULE_NAME>.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "<MODULE_NAME>.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.<MODULE_NAME>.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.<MODULE_NAME>.securityContext | nindent 12 }}
          image: "{{ .Values.<MODULE_NAME>.image.repository }}:{{ .Values.<MODULE_NAME>.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.<MODULE_NAME>.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 80
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /
              port: http
          readinessProbe:
            httpGet:
              path: /
              port: http
          resources:
            {{- toYaml .Values.<MODULE_NAME>.resources | nindent 12 }}
      {{- with .Values.<MODULE_NAME>.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.<MODULE_NAME>.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.<MODULE_NAME>.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
`

const defaultService = `apiVersion: v1
kind: Service
metadata:
  name: {{ include "<MODULE_NAME>.fullname" . }}
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
spec:
  type: {{ .Values.<MODULE_NAME>.service.type }}
  ports:
    - port: {{ .Values.<MODULE_NAME>.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app.kubernetes.io/name: {{ include "<MODULE_NAME>.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
`

const defaultServiceAccount = `{{- if .Values.<MODULE_NAME>.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "<MODULE_NAME>.serviceAccountName" . }}
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
  {{- with .Values.<MODULE_NAME>.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
`

const defaultHorizontalPodAutoscaler = `{{- if .Values.<MODULE_NAME>.autoscaling.enabled }}
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "<MODULE_NAME>.fullname" . }}
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "<MODULE_NAME>.fullname" . }}
  minReplicas: {{ .Values.<MODULE_NAME>.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.<MODULE_NAME>.autoscaling.maxReplicas }}
  metrics:
    {{- if .Values.<MODULE_NAME>.autoscaling.targetCPUUtilizationPercentage }}
    - type: Resource
      resource:
        name: cpu
        targetAverageUtilization: {{ .Values.<MODULE_NAME>.autoscaling.targetCPUUtilizationPercentage }}
    {{- end }}
    {{- if .Values.<MODULE_NAME>.autoscaling.targetMemoryUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        targetAverageUtilization: {{ .Values.<MODULE_NAME>.autoscaling.targetMemoryUtilizationPercentage }}
    {{- end }}
{{- end }}
`

const defaultNotes = `1. Get the application URL by running these commands:
{{- if .Values.<MODULE_NAME>.ingress.enabled }}
{{- range $host := .Values.<MODULE_NAME>.ingress.hosts }}
  {{- range .paths }}
  http{{ if $.Values.<MODULE_NAME>.ingress.tls }}s{{ end }}://{{ $host.host }}{{ .path }}
  {{- end }}
{{- end }}
{{- else if contains "NodePort" .Values.<MODULE_NAME>.service.type }}
  export NODE_PORT=$(kubectl get --namespace {{ .Release.Namespace }} -o jsonpath="{.spec.ports[0].nodePort}" services {{ include "<MODULE_NAME>.fullname" . }})
  export NODE_IP=$(kubectl get nodes --namespace {{ .Release.Namespace }} -o jsonpath="{.items[0].status.addresses[0].address}")
  echo http://$NODE_IP:$NODE_PORT
{{- else if contains "LoadBalancer" .Values.<MODULE_NAME>.service.type }}
     NOTE: It may take a few minutes for the LoadBalancer IP to be available.
           You can watch the status of by running 'kubectl get --namespace {{ .Release.Namespace }} svc -w {{ include "<MODULE_NAME>.fullname" . }}'
  export SERVICE_IP=$(kubectl get svc --namespace {{ .Release.Namespace }} {{ include "<MODULE_NAME>.fullname" . }} --template "{{"{{ range (index .status.loadBalancer.ingress 0) }}{{.}}{{ end }}"}}")
  echo http://$SERVICE_IP:{{ .Values.<MODULE_NAME>.service.port }}
{{- else if contains "ClusterIP" .Values.<MODULE_NAME>.service.type }}
  export POD_NAME=$(kubectl get pods --namespace {{ .Release.Namespace }} -l "app.kubernetes.io/name={{ include "<MODULE_NAME>.name" . }},app.kubernetes.io/instance={{ .Release.Name }}" -o jsonpath="{.items[0].metadata.name}")
  export CONTAINER_PORT=$(kubectl get pod --namespace {{ .Release.Namespace }} $POD_NAME -o jsonpath="{.spec.containers[0].ports[0].containerPort}")
  echo "Visit http://127.0.0.1:8080 to use your application"
  kubectl --namespace {{ .Release.Namespace }} port-forward $POD_NAME 8080:$CONTAINER_PORT
{{- end }}
`

const defaultHelpers = `{{/*
Expand the name of the chart.
*/}}
{{- define "<MODULE_NAME>.name" -}}
{{- default .Chart.Name .Values.<MODULE_NAME>.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "<MODULE_NAME>.fullname" -}}
{{- if .Values.<MODULE_NAME>.fullnameOverride }}
{{- .Values.<MODULE_NAME>.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.<MODULE_NAME>.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s-%s" .Release.Name $name "<MODULE_NAME>" | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "<MODULE_NAME>.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "<MODULE_NAME>.labels" -}}
helm.sh/chart: {{ include "<MODULE_NAME>.chart" . }}
{{ include "<MODULE_NAME>.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "<MODULE_NAME>.selectorLabels" -}}
app.kubernetes.io/name: {{ include "<MODULE_NAME>.name" . }}-<MODULE_NAME>
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "<MODULE_NAME>.serviceAccountName" -}}
{{- if .Values.<MODULE_NAME>.serviceAccount.create }}
{{- default (include "<MODULE_NAME>.fullname" .) .Values.<MODULE_NAME>.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.<MODULE_NAME>.serviceAccount.name }}
{{- end }}
{{- end }}
`

const defaultTestConnection = `apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "<MODULE_NAME>.fullname" . }}-test-connection"
  labels:
    {{- include "<MODULE_NAME>.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": test
spec:
  containers:
    - name: wget
      image: busybox
      command: ['wget']
      args: ['{{ include "<MODULE_NAME>.fullname" . }}:{{ .Values.<MODULE_NAME>.service.port }}']
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
		newData := transform(string(template.Data), "main")
		updatedTemplates = append(updatedTemplates, &chart.File{Name: template.Name, Data: newData})
	}

	schart.Templates = updatedTemplates
	b, err := yaml.Marshal(schart.Values)
	if err != nil {
		return errors.Wrap(err, "reading values file")
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(transform(string(b), "main"), &m); err != nil {
		return errors.Wrap(err, "transforming values file")
	}
	schart.Values = m

	// SaveDir looks for the file values.yaml when saving rather than the values
	// key in order to preserve the comments in the YAML. The name placeholder
	// needs to be replaced on that file.
	for _, f := range schart.Raw {
		if f.Name == ValuesfileName {
			f.Data = transform(string(f.Data), "main")
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
func Create(chartname, dir string) (string, error) {

	// Sanity-check the name of a chart so user doesn't create one that causes problems.
	if err := validateChartName(chartname); err != nil {
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

	cdir := filepath.Join(path, chartname)
	if fi, err := os.Stat(cdir); err == nil && !fi.IsDir() {
		return cdir, errors.Errorf("file %s already exists and is not a directory", cdir)
	}

	var module = "main"

	// if we are "inside" a helm chart we generate a module with the name from args
	if _, err := os.Stat(ValuesfileName); err == nil {
		// create module with "chartname"
		module = chartname
		writeFiles(getFiles("", module))
		appendToValuesFile(module)
	} else {
		// create helm chart with module main
		writeFiles(getBasefiles(cdir, module, chartname))
		writeFiles(getFiles(cdir, module))
		// Need to add the ChartsDir explicitly as it does not contain any file OOTB
		if err := os.MkdirAll(filepath.Join(cdir, ChartsDir), 0755); err != nil {
			return cdir, err
		}
	}

	return cdir, nil
}

func getFiles(cdir string, module string) []ManifestFile {
	return []ManifestFile{
		{
			// ingress.yaml
			path:    filepath.Join(cdir, transformModuleName(IngressFileName, module)),
			content: transform(defaultIngress, module),
		},
		{
			// deployment.yaml
			path:    filepath.Join(cdir, transformModuleName(DeploymentName, module)),
			content: transform(defaultDeployment, module),
		},
		{
			// service.yaml
			path:    filepath.Join(cdir, transformModuleName(ServiceName, module)),
			content: transform(defaultService, module),
		},
		{
			// serviceaccount.yaml
			path:    filepath.Join(cdir, transformModuleName(ServiceAccountName, module)),
			content: transform(defaultServiceAccount, module),
		},
		{
			// hpa.yaml
			path:    filepath.Join(cdir, transformModuleName(HorizontalPodAutoscalerName, module)),
			content: transform(defaultHorizontalPodAutoscaler, module),
		},
		{
			// _helpers.tpl
			path:    filepath.Join(cdir, transformModuleName(HelpersName, module)),
			content: transform(defaultHelpers, module),
		},
		{
			// test-connection.yaml
			path:    filepath.Join(cdir, transformModuleName(TestConnectionName, module)),
			content: transform(defaultTestConnection, module),
		},
	}
}

func getBasefiles(cdir string, module string, chartname string) []ManifestFile {
	return []ManifestFile{
		{
			// values.yaml
			path:    filepath.Join(cdir, ValuesfileName),
			content: transform(defaultValues, module),
		},
		{
			// Chart.yaml
			path:    filepath.Join(cdir, ChartfileName),
			content: []byte(fmt.Sprintf(defaultChartfile, chartname)),
		},
		{
			// .helmignore
			path:    filepath.Join(cdir, IgnorefileName),
			content: []byte(defaultIgnore),
		},
		{
			// NOTES.txt
			path:    filepath.Join(cdir, transformModuleName(NotesName, module)),
			content: transform(defaultNotes, module),
		},
	}
}

func writeFiles(files []ManifestFile) error {
	for _, file := range files {
		if _, err := os.Stat(file.path); err == nil {
			// There is no handle to a preferred output stream here.
			fmt.Fprintf(Stderr, "WARNING: File %q already exists. Overwriting.\n", file.path)
		}
		if err := writeFile(file.path, file.content); err != nil {
			return err
		}
	}
	return nil
}

func appendToValuesFile(module string) {
	f, err := os.OpenFile(ValuesfileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(Stderr, "ERROR: Opening to %q.\n", ValuesfileName)
	}
	defer f.Close()
	if _, err := f.Write(transform(defaultValues, module)); err != nil {
		fmt.Fprintf(Stderr, "ERROR: Writing to %q.\n", ValuesfileName)
	}
}

// transform performs a string replacement of the specified source for
// a given key with the replacement string
func transform(src, module string) []byte {
	return []byte(strings.ReplaceAll(src, "<MODULE_NAME>", module))
}

func transformModuleName(src, moduleName string) string {
	return strings.ReplaceAll(src, moduleNameTemplate, moduleName+"_")
}

func writeFile(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(name, content, 0644)
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
