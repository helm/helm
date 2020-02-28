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
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/releaseutil"
)

const templateDesc = `
Render chart templates locally and display the output.

Any values that would normally be looked up or retrieved in-cluster will be
faked locally. Additionally, none of the server-side testing of chart validity
(e.g. whether an API is supported) is done.
`

type templateOptions struct {
	extraAPIs   []string
	showFiles   []string
	lint        bool
	includeCrds bool
	validate    bool
}

func newTemplateCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	o := &templateOptions{}
	installClient := action.NewInstall(cfg)
	lintClient := action.NewLint()
	valueOpts := &values.Options{}

	cmd := &cobra.Command{
		Use:   "template [NAME] [CHART]",
		Short: "locally render templates",
		Long:  templateDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var err error
			if o.lint {
				err := runLint(args, lintClient, valueOpts, out)
				if err != nil {
					return err
				}
			}

			err = runTemplate(args, installClient, valueOpts, o, out)
			if err != nil {
				return err
			}

			return nil
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		return compInstall(args, toComplete, installClient)
	})

	f := cmd.Flags()
	addInstallFlags(f, installClient, valueOpts)
	f.StringArrayVarP(&o.showFiles, "show-only", "s", []string{}, "only show manifests rendered from the given templates")
	f.StringVar(&installClient.OutputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	f.BoolVar(&installClient.IsUpgrade, "is-upgrade", false, "set .Release.IsUpgrade instead of .Release.IsInstall")
	f.BoolVar(&o.validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. This is the same validation performed on an install")
	f.BoolVar(&o.includeCrds, "include-crds", false, "include CRDs in the templated output")
	f.BoolVar(&o.lint, "lint", false, "examines a chart for possible issues")
	f.BoolVar(&lintClient.Strict, "strict", false, "fail on lint warnings")
	f.StringArrayVarP(&o.extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions")
	f.BoolVar(&installClient.UseReleaseName, "release-name", false, "use release name in the output-dir path.")
	bindPostRenderFlag(cmd, &installClient.PostRenderer)

	return cmd
}

func runTemplate(args []string, client *action.Install, valueOpts *values.Options, o *templateOptions, out io.Writer) error {
	client.DryRun = true
	client.ReleaseName = "RELEASE-NAME"
	client.Replace = true // Skip the name check
	client.ClientOnly = !o.validate
	client.APIVersions = chartutil.VersionSet(o.extraAPIs)
	client.IncludeCRDs = o.includeCrds
	rel, err := runInstall(args, client, valueOpts, out)
	if err != nil {
		return err
	}

	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))

	if !client.DisableHooks {
		for _, m := range rel.Hooks {
			fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
		}
	}

	// if we have a list of files to render, then check that each of the
	// provided files exists in the chart.
	if len(o.showFiles) > 0 {
		splitManifests := releaseutil.SplitManifests(manifests.String())
		manifestNameRegex := regexp.MustCompile("# Source: [^/]+/(.+)")
		var manifestsToRender []string
		for _, f := range o.showFiles {
			missing := true
			for _, manifest := range splitManifests {
				submatch := manifestNameRegex.FindStringSubmatch(manifest)
				if len(submatch) == 0 {
					continue
				}
				manifestName := submatch[1]
				// manifest.Name is rendered using linux-style filepath separators on Windows as
				// well as macOS/linux.
				manifestPathSplit := strings.Split(manifestName, "/")
				manifestPath := filepath.Join(manifestPathSplit...)

				// if the filepath provided matches a manifest path in the
				// chart, render that manifest
				if f == manifestPath {
					manifestsToRender = append(manifestsToRender, manifest)
					missing = false
				}
			}
			if missing {
				return fmt.Errorf("could not find template %s in chart", f)
			}
		}
		for _, m := range manifestsToRender {
			fmt.Fprintf(out, "---\n%s\n", m)
		}
	} else {
		fmt.Fprintf(out, "%s", manifests.String())
	}

	return nil
}
