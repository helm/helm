/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/downloader"
	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/cmd/helm/strvals"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/release"
)

const installDesc = `
This command installs a chart archive.

The install argument must be either a relative path to a chart directory or the
name of a chart in the current working directory.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line.

	$ helm install -f myvalues.yaml ./redis

or

	$ helm install --set name=prod ./redis

To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined. This will still require a
round-trip to the Tiller server.

If --verify is set, the chart MUST have a provenance file, and the provenenace
fall MUST pass all verification steps.

There are four different ways you can express the chart you want to install:

1. By chart reference: helm install stable/mariadb
2. By path to a packaged chart: helm install ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install ./nginx
4. By absolute URL: helm install https://example.com/charts/nginx-1.2.3.tgz

CHART REFERENCES

A chart reference is a convenient way of reference a chart in a chart repository.

When you use a chart reference ('stable/mariadb'), Helm will look in the local
configuration for a chart repository named 'stable', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest
version of that chart unless you also supply a version number with the
'--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`

type installCmd struct {
	name         string
	namespace    string
	valuesFile   string
	chartPath    string
	dryRun       bool
	disableHooks bool
	replace      bool
	verify       bool
	keyring      string
	out          io.Writer
	client       helm.Interface
	values       string
	nameTemplate string
	version      string
}

func newInstallCmd(c helm.Interface, out io.Writer) *cobra.Command {
	inst := &installCmd{
		out:    out,
		client: c,
	}

	cmd := &cobra.Command{
		Use:               "install [CHART]",
		Short:             "install a chart archive",
		Long:              installDesc,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(args[0], inst.version, inst.verify, inst.keyring)
			if err != nil {
				return err
			}
			inst.chartPath = cp
			inst.client = ensureHelmClient(inst.client)
			return inst.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&inst.valuesFile, "values", "f", "", "specify values in a YAML file")
	f.StringVarP(&inst.name, "name", "n", "", "release name. If unspecified, it will autogenerate one for you")
	f.StringVar(&inst.namespace, "namespace", "", "namespace to install the release into")
	f.BoolVar(&inst.dryRun, "dry-run", false, "simulate an install")
	f.BoolVar(&inst.disableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&inst.replace, "replace", false, "re-use the given name, even if that name is already used. This is unsafe in production")
	f.StringVar(&inst.values, "set", "", "set values on the command line. Separate values with commas: key1=val1,key2=val2")
	f.StringVar(&inst.nameTemplate, "name-template", "", "specify template used to name the release")
	f.BoolVar(&inst.verify, "verify", false, "verify the package before installing it")
	f.StringVar(&inst.keyring, "keyring", defaultKeyring(), "location of public keys used for verification")
	f.StringVar(&inst.version, "version", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")

	return cmd
}

func (i *installCmd) run() error {
	if flagDebug {
		fmt.Fprintf(i.out, "CHART PATH: %s\n", i.chartPath)
	}

	if i.namespace == "" {
		i.namespace = defaultNamespace()
	}

	rawVals, err := i.vals()
	if err != nil {
		return err
	}

	// If template is specified, try to run the template.
	if i.nameTemplate != "" {
		i.name, err = generateName(i.nameTemplate)
		if err != nil {
			return err
		}
		// Print the final name so the user knows what the final name of the release is.
		fmt.Printf("FINAL NAME: %s\n", i.name)
	}

	res, err := i.client.InstallRelease(
		i.chartPath,
		i.namespace,
		helm.ValueOverrides(rawVals),
		helm.ReleaseName(i.name),
		helm.InstallDryRun(i.dryRun),
		helm.InstallReuseName(i.replace),
		helm.InstallDisableHooks(i.disableHooks))
	if err != nil {
		return prettyError(err)
	}

	rel := res.GetRelease()
	if rel == nil {
		return nil
	}
	i.printRelease(rel)

	// If this is a dry run, we can't display status.
	if i.dryRun {
		return nil
	}

	// Print the status like status command does
	status, err := i.client.ReleaseStatus(rel.Name)
	if err != nil {
		return prettyError(err)
	}
	PrintStatus(i.out, status)
	return nil
}

func (i *installCmd) vals() ([]byte, error) {
	base := map[string]interface{}{}

	// User specified a values file via -f/--values
	if i.valuesFile != "" {
		bytes, err := ioutil.ReadFile(i.valuesFile)
		if err != nil {
			return []byte{}, err
		}

		if err := yaml.Unmarshal(bytes, &base); err != nil {
			return []byte{}, fmt.Errorf("failed to parse %s: %s", i.valuesFile, err)
		}
	}

	if err := strvals.ParseInto(i.values, base); err != nil {
		return []byte{}, fmt.Errorf("failed parsing --set data: %s", err)
	}

	return yaml.Marshal(base)
}

// printRelease prints info about a release if the flagDebug is true.
func (i *installCmd) printRelease(rel *release.Release) {
	if rel == nil {
		return
	}
	// TODO: Switch to text/template like everything else.
	if flagDebug {
		fmt.Fprintf(i.out, "NAME:   %s\n", rel.Name)
		fmt.Fprintf(i.out, "TARGET NAMESPACE:   %s\n", rel.Namespace)
		fmt.Fprintf(i.out, "CHART:  %s %s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
		fmt.Fprintf(i.out, "MANIFEST: %s\n", rel.Manifest)
	} else {
		fmt.Fprintf(i.out, "NAME: %s\n", rel.Name)
	}
}

// locateChartPath looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - current working directory
// - if path is absolute or begins with '.', error out here
// - chart repos in $HELM_HOME
// - URL
//
// If 'verify' is true, this will attempt to also verify the chart.
func locateChartPath(name, version string, verify bool, keyring string) (string, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if fi, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if verify {
			if fi.IsDir() {
				return "", errors.New("cannot verify a directory")
			}
			if _, err := downloader.VerifyChart(abs, keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(helmpath.Home(homePath()).Repository(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	dl := downloader.ChartDownloader{
		HelmHome: helmpath.Home(homePath()),
		Out:      os.Stdout,
		Keyring:  keyring,
	}
	if verify {
		dl.Verify = downloader.VerifyAlways
	}

	filename, _, err := dl.DownloadTo(name, version, ".")
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		fmt.Printf("Fetched %s to %s\n", name, filename)
		return lname, nil
	} else if flagDebug {
		return filename, err
	}

	return filename, fmt.Errorf("file %q not found", name)
}

func generateName(nameTemplate string) (string, error) {
	t, err := template.New("name-template").Funcs(sprig.TxtFuncMap()).Parse(nameTemplate)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, nil)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func defaultNamespace() string {
	if ns, _, err := kube.GetConfig(kubeContext).Namespace(); err == nil {
		return ns
	}
	return "default"
}
