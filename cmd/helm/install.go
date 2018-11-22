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
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

const installDesc = `
This command installs a chart archive.

The install argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'.

	$ helm install -f myvalues.yaml myredis ./redis

or

	$ helm install --set name=prod myredis ./redis

or

	$ helm install --set-string long_int=1234567890 myredis ./redis

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

	$ helm install -f myvalues.yaml -f override.yaml  myredis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

	$ helm install --set foo=bar --set foo=newbar  myredis ./redis


To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined. This will still require a
round-trip to the Tiller server.

If --verify is set, the chart MUST have a provenance file, and the provenance
file MUST pass all verification steps.

There are five different ways you can express the chart you want to install:

1. By chart reference: helm install stable/mariadb
2. By path to a packaged chart: helm install ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install ./nginx
4. By absolute URL: helm install https://example.com/charts/nginx-1.2.3.tgz
5. By chart reference and repo url: helm install --repo https://example.com/charts/ nginx

CHART REFERENCES

A chart reference is a convenient way of reference a chart in a chart repository.

When you use a chart reference with a repo prefix ('stable/mariadb'), Helm will look in the local
configuration for a chart repository named 'stable', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest
version of that chart unless you also supply a version number with the
'--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`

type installOptions struct {
	name         string // arg 0
	dryRun       bool   // --dry-run
	disableHooks bool   // --disable-hooks
	replace      bool   // --replace
	nameTemplate string // --name-template
	timeout      int64  // --timeout
	wait         bool   // --wait
	devel        bool   // --devel
	depUp        bool   // --dep-up
	chartPath    string // arg 1
	generateName bool   // --generate-name

	valuesOptions
	chartPathOptions

	client helm.Interface
}

func newInstallCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &installOptions{client: c}

	cmd := &cobra.Command{
		Use:   "install [NAME] [CHART]",
		Short: "install a chart",
		Long:  installDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			debug("Original chart version: %q", o.version)
			if o.version == "" && o.devel {
				debug("setting version to >0.0.0-0")
				o.version = ">0.0.0-0"
			}

			name, chart, err := o.nameAndChart(args)
			if err != nil {
				return err
			}
			o.name = name // FIXME

			cp, err := o.locateChart(chart)
			if err != nil {
				return err
			}
			o.chartPath = cp
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.generateName, "generate-name", "g", false, "generate the name (and omit the NAME parameter)")
	f.BoolVar(&o.dryRun, "dry-run", false, "simulate an install")
	f.BoolVar(&o.disableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&o.replace, "replace", false, "re-use the given name, even if that name is already used. This is unsafe in production")
	f.StringVar(&o.nameTemplate, "name-template", "", "specify template used to name the release")
	f.Int64Var(&o.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&o.wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&o.devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")
	f.BoolVar(&o.depUp, "dep-up", false, "run helm dependency update before installing the chart")
	o.valuesOptions.addFlags(f)
	o.chartPathOptions.addFlags(f)

	return cmd
}

// nameAndChart returns the name of the release and the chart that should be used.
//
// This will read the flags and handle name generation if necessary.
func (o *installOptions) nameAndChart(args []string) (string, string, error) {
	flagsNotSet := func() error {
		if o.generateName {
			return errors.New("cannot set --generate-name and also specify a name")
		}
		if o.nameTemplate != "" {
			return errors.New("cannot set --name-template and also specify a name")
		}
		return nil
	}
	if len(args) == 2 {
		return args[0], args[1], flagsNotSet()
	}

	if o.nameTemplate != "" {
		newName, err := templateName(o.nameTemplate)
		return newName, args[0], err
	}

	if !o.generateName {
		return "", args[0], errors.New("must either provide a name or specify --generate-name")
	}

	base := filepath.Base(args[0])
	if base == "." || base == "" {
		base = "chart"
	}
	newName := fmt.Sprintf("%s-%d", base, time.Now().Unix())

	return newName, args[0], nil
}

func (o *installOptions) run(out io.Writer) error {
	debug("CHART PATH: %s\n", o.chartPath)

	rawVals, err := o.mergedValues()
	if err != nil {
		return err
	}

	// If template is specified, try to run the template.
	if o.nameTemplate != "" {
		o.name, err = templateName(o.nameTemplate)
		if err != nil {
			return err
		}
		// Print the final name so the user knows what the final name of the release is.
		fmt.Printf("FINAL NAME: %s\n", o.name)
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	chartRequested, err := loader.Load(o.chartPath)
	if err != nil {
		return err
	}

	if req := chartRequested.Metadata.Requirements; req != nil {
		// If checkDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := checkDependencies(chartRequested, req); err != nil {
			if o.depUp {
				man := &downloader.Manager{
					Out:        out,
					ChartPath:  o.chartPath,
					HelmHome:   settings.Home,
					Keyring:    o.keyring,
					SkipUpdate: false,
					Getters:    getter.All(settings),
				}
				if err := man.Update(); err != nil {
					return err
				}
			} else {
				return err
			}

		}
	}

	rel, err := o.client.InstallReleaseFromChart(
		chartRequested,
		getNamespace(),
		helm.ValueOverrides(rawVals),
		helm.ReleaseName(o.name),
		helm.InstallDryRun(o.dryRun),
		helm.InstallReuseName(o.replace),
		helm.InstallDisableHooks(o.disableHooks),
		helm.InstallTimeout(o.timeout),
		helm.InstallWait(o.wait))
	if err != nil {
		return err
	}

	if rel == nil {
		return nil
	}
	o.printRelease(out, rel)

	// If this is a dry run, we can't display status.
	if o.dryRun {
		return nil
	}

	// Print the status like status command does
	status, err := o.client.ReleaseStatus(rel.Name, 0)
	if err != nil {
		return err
	}
	PrintStatus(out, status)
	return nil
}

// Merges source and destination map, preferring values from the source map
func mergeValues(dest, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}

// printRelease prints info about a release if the Debug is true.
func (o *installOptions) printRelease(out io.Writer, rel *release.Release) {
	if rel == nil {
		return
	}
	fmt.Fprintf(out, "NAME:   %s\n", rel.Name)
	if settings.Debug {
		printRelease(out, rel)
	}
}

func templateName(nameTemplate string) (string, error) {
	t, err := template.New("name-template").Funcs(sprig.TxtFuncMap()).Parse(nameTemplate)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, nil)
	return b.String(), err
}

func checkDependencies(ch *chart.Chart, reqs []*chart.Dependency) error {
	var missing []string

OUTER:
	for _, r := range reqs {
		for _, d := range ch.Dependencies() {
			if d.Name() == r.Name {
				continue OUTER
			}
		}
		missing = append(missing, r.Name)
	}

	if len(missing) > 0 {
		return errors.Errorf("found in requirements.yaml, but missing in charts/ directory: %s", strings.Join(missing, ", "))
	}
	return nil
}
