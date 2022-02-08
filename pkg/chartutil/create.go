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

type ManifestFile struct {
	path    string
	content []byte
}

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
