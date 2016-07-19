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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/timeconv"
)

const installDesc = `
This command installs a chart archive.

The install argument must be either a relative
path to a chart directory or the name of a
chart in the current working directory.
`

type installCmd struct {
	name         string
	namespace    string
	valuesFile   string
	chartPath    string
	dryRun       bool
	disableHooks bool
	out          io.Writer
	client       helm.Interface
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
			if err := checkArgsLength(1, len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(args[0])
			if err != nil {
				return err
			}
			inst.chartPath = cp
			inst.client = ensureHelmClient(inst.client)
			return inst.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&inst.valuesFile, "values", "f", "", "path to a values YAML file")
	f.StringVarP(&inst.name, "name", "n", "", "the release name. If unspecified, it will autogenerate one for you.")
	// TODO use kubeconfig default
	f.StringVar(&inst.namespace, "namespace", "default", "the namespace to install the release into")
	f.BoolVar(&inst.dryRun, "dry-run", false, "simulate an install")
	f.BoolVar(&inst.disableHooks, "no-hooks", false, "prevent hooks from running during install")
	return cmd
}

func (i *installCmd) run() error {
	if flagDebug {
		fmt.Printf("Chart path: %s\n", i.chartPath)
	}

	rawVals, err := i.vals()
	if err != nil {
		return err
	}

	res, err := i.client.InstallRelease(i.chartPath, i.namespace, helm.ValueOverrides(rawVals), helm.ReleaseName(i.name), helm.InstallDryRun(i.dryRun), helm.InstallDisableHooks(i.disableHooks))
	if err != nil {
		return prettyError(err)
	}

	i.printRelease(res.GetRelease())

	return nil
}

func (i *installCmd) vals() ([]byte, error) {
	if i.valuesFile == "" {
		return []byte{}, nil
	}
	return ioutil.ReadFile(i.valuesFile)
}

func (i *installCmd) printRelease(rel *release.Release) {
	if rel == nil {
		return
	}
	// TODO: Switch to text/template like everything else.
	if flagDebug {
		fmt.Fprintf(i.out, "NAME:   %s\n", rel.Name)
		fmt.Fprintf(i.out, "NAMESPACE:   %s\n", rel.Namespace)
		fmt.Fprintf(i.out, "INFO:   %s %s\n", timeconv.String(rel.Info.LastDeployed), rel.Info.Status)
		fmt.Fprintf(i.out, "CHART:  %s %s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
		fmt.Fprintf(i.out, "MANIFEST: %s\n", rel.Manifest)
	} else {
		fmt.Fprintln(i.out, rel.Name)
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
func locateChartPath(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return filepath.Abs(name)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(repositoryDirectory(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	// Try fetching the chart from a remote repo into a tmpdir
	origname := name
	if filepath.Ext(name) != ".tgz" {
		name += ".tgz"
	}
	if err := fetchChart(name); err == nil {
		lname, err := filepath.Abs(filepath.Base(name))
		if err != nil {
			return lname, err
		}
		fmt.Printf("Fetched %s to %s\n", origname, lname)
		return lname, nil
	}

	return name, fmt.Errorf("file %q not found", origname)
}
