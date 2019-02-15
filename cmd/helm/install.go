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
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/chartutil"
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

	cfg *action.Configuration

	// LEGACY: Here until we get upgrade converted
	client helm.Interface
}

func newInstallCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	o := &installOptions{cfg: cfg}

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
		fmt.Fprintf(out, "FINAL NAME: %s\n", o.name)
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(o.chartPath)
	if err != nil {
		return err
	}

	validInstallableChart, err := chartutil.IsChartInstallable(chartRequested)
	if !validInstallableChart {
		return err
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
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

	inst := action.NewInstall(o.cfg)
	inst.DryRun = o.dryRun
	inst.DisableHooks = o.disableHooks
	inst.Replace = o.replace
	inst.Wait = o.wait
	inst.Devel = o.devel
	inst.Timeout = o.timeout
	inst.Namespace = getNamespace()
	inst.ReleaseName = o.name
	rel, err := inst.Run(chartRequested, rawVals)
	if err != nil {
		return err
	}

	o.printRelease(out, rel)
	return nil
}

// printRelease prints info about a release
func (o *installOptions) printRelease(out io.Writer, rel *release.Release) {
	if rel == nil {
		return
	}
	fmt.Fprintf(out, "NAME:   %s\n", rel.Name)
	if settings.Debug {
		printRelease(out, rel)
	}
	if !rel.Info.LastDeployed.IsZero() {
		fmt.Fprintf(out, "LAST DEPLOYED: %s\n", rel.Info.LastDeployed)
	}
	fmt.Fprintf(out, "NAMESPACE: %s\n", rel.Namespace)
	fmt.Fprintf(out, "STATUS: %s\n", rel.Info.Status.String())
	fmt.Fprintf(out, "\n")
	if len(rel.Info.Resources) > 0 {
		re := regexp.MustCompile("  +")

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.TabIndent)
		fmt.Fprintf(w, "RESOURCES:\n%s\n", re.ReplaceAllString(rel.Info.Resources, "\t"))
		w.Flush()
	}
	if rel.Info.LastTestSuiteRun != nil {
		lastRun := rel.Info.LastTestSuiteRun
		fmt.Fprintf(out, "TEST SUITE:\n%s\n%s\n\n%s\n",
			fmt.Sprintf("Last Started: %s", lastRun.StartedAt),
			fmt.Sprintf("Last Completed: %s", lastRun.CompletedAt),
			formatTestResults(lastRun.Results))
	}

	if len(rel.Info.Notes) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", rel.Info.Notes)
	}
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
		return errors.Errorf("found in Chart.yaml, but missing in charts/ directory: %s", strings.Join(missing, ", "))
	}
	return nil
}
