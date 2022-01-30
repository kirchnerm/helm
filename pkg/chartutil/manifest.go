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
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const ingressValues = `
<MANIFEST_NAME>_ingress:
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
`

const ingress = `{{- if .Values.<MANIFEST_NAME>_ingress.enabled -}}
{{- $fullName := include "<CHARTNAME>.fullname" . -}}
{{- $svcPort := .Values.<MANIFEST_NAME>_service.port -}}
{{- if and .Values.<MANIFEST_NAME>_ingress.className (not (semverCompare ">=1.18-0" .Capabilities.KubeVersion.GitVersion)) }}
  {{- if not (hasKey .Values.<MANIFEST_NAME>_ingress.annotations "kubernetes.io/ingress.class") }}
  {{- $_ := set .Values.<MANIFEST_NAME>_ingress.annotations "kubernetes.io/ingress.class" .Values.<MANIFEST_NAME>_ingress.className}}
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
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
  {{- with .Values.<MANIFEST_NAME>_ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if and .Values.<MANIFEST_NAME>_ingress.className (semverCompare ">=1.18-0" .Capabilities.KubeVersion.GitVersion) }}
  ingressClassName: {{ .Values.<MANIFEST_NAME>_ingress.className }}
  {{- end }}
  {{- if .Values.<MANIFEST_NAME>_ingress.tls }}
  tls:
    {{- range .Values.<MANIFEST_NAME>_ingress.tls }}
    - hosts:
        {{- range .hosts }}
        - {{ . | quote }}
        {{- end }}
      secretName: {{ .secretName }}
    {{- end }}
  {{- end }}
  rules:
    {{- range .Values.<MANIFEST_NAME>_ingress.hosts }}
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
                name: {{ $fullName }}-<MANIFEST_NAME>
                port:
                  number: {{ $svcPort }}
              {{- else }}
              serviceName: {{ $fullName }}-<MANIFEST_NAME>
              servicePort: {{ $svcPort }}
              {{- end }}
          {{- end }}
    {{- end }}
{{- end }}
`

const serviceValues = `
<MANIFEST_NAME>_service:
  type: ClusterIP
  port: 80
`

const service = `apiVersion: v1
kind: Service
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}-<MANIFEST_NAME>
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
    app.kubernetes.io/name: {{ include "<CHARTNAME>.name" . }}-<MANIFEST_NAME>
    app.kubernetes.io/instance: {{ .Release.Name }}
`

const deploymentValues = `
<MANIFEST_NAME>_deployment:
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

  nodeSelector: {}

  tolerations: []

  affinity: {}
`

const deployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}-<MANIFEST_NAME>
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
spec:
  {{- if not .Values.<MANIFEST_NAME>_deployment.autoscaling.enabled }}
  replicas: {{ .Values.<MANIFEST_NAME>_deployment.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "<CHARTNAME>.name" . }}-<MANIFEST_NAME>
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      {{- with .Values.<MANIFEST_NAME>_deployment.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        app.kubernetes.io/name: {{ include "<CHARTNAME>.name" . }}-<MANIFEST_NAME>
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      {{- with .Values.<MANIFEST_NAME>_deployment.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "<CHARTNAME>.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.<MANIFEST_NAME>_deployment.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.<MANIFEST_NAME>_deployment.securityContext | nindent 12 }}
          image: "{{ .Values.<MANIFEST_NAME>_deployment.image.repository }}:{{ .Values.<MANIFEST_NAME>_deployment.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.<MANIFEST_NAME>_deployment.image.pullPolicy }}
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
            {{- toYaml .Values.<MANIFEST_NAME>_deployment.resources | nindent 12 }}
      {{- with .Values.<MANIFEST_NAME>_deployment.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.<MANIFEST_NAME>_deployment.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.<MANIFEST_NAME>_deployment.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
`

type Manifest struct {
	content string
	values  string
}

var manifests = map[string]Manifest{
	"ingress":    {ingress, ingressValues},
	"service":    {service, serviceValues},
	"deployment": {deployment, deploymentValues},
}

func CreateManifest(manifest string, name string) (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return path, err
	}

	// Sanity-check the name of a chart so user doesn't create one that causes problems.
	schart, err := loader.Load(path)
	if err != nil {
		return "", errors.Wrapf(err, "could not load %s", path)
	}

	chartName := schart.Name()

	if err := validateChartName(chartName); err != nil {
		return "", err
	}

	if fi, err := os.Stat(path); err != nil {
		return path, err
	} else if !fi.IsDir() {
		return path, errors.Errorf("no such directory %s", path)
	}

	cdir := filepath.Join(path)
	if fi, err := os.Stat(cdir); err == nil && !fi.IsDir() {
		return cdir, errors.Errorf("file %s already exists and is not a directory", cdir)
	}

	files := []struct {
		path    string
		content []byte
	}{
		{
			// ingress.yaml
			path:    filepath.Join(cdir, TemplatesDir+sep+name+"_"+manifest+".yaml"),
			content: transformManifestName(manifests[manifest].content, chartName, name),
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

	f, err := os.OpenFile(filepath.Join(cdir, ValuesfileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.Write(transformManifestName(manifests[manifest].values, chartName, name)); err != nil {
		log.Println(err)
	}

	return cdir, nil
}

// transform performs a string replacement of the specified source for
// a given key with the replacement string
func transformManifestName(src, chartname string, manifestName string) []byte {
	return []byte(strings.ReplaceAll(strings.ReplaceAll(src, "<MANIFEST_NAME>", manifestName), "<CHARTNAME>", chartname))
}
