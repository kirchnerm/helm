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

package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/helmpath"
)

const manifestDesc = `
This command creates a kubernetes Manifest with optional dynamics.
`

type manifestOptions struct {
	starter    string // --starter
	name       string
	manifest   string
	starterDir string
}

func newManifestCmd(out io.Writer) *cobra.Command {
	o := &manifestOptions{}

	cmd := &cobra.Command{
		Use:   "manifest TYPE NAME",
		Short: "create a new kubernetes manifest (ingres, deployment, service, ...) with the given name",
		Long:  manifestDesc,
		Args:  require.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Allow file completion when completing the argument for the name
				// which could be a path
				return nil, cobra.ShellCompDirectiveDefault
			}
			// No more completions, so disable file completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.manifest = args[0]
			o.name = args[1]
			o.starterDir = helmpath.DataPath("starters")
			return o.run(out)
		},
	}

	cmd.Flags().StringVarP(&o.starter, "starter", "p", "", "the name or absolute path to Helm starter scaffold")
	return cmd
}

func (o *manifestOptions) run(out io.Writer) error {
	fmt.Fprintf(out, "Creating manifest %s\n", o.name)

	chartutil.Stderr = out
	_, err := chartutil.CreateManifest(o.manifest, o.name)
	return err
}
