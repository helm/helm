/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/strvals"
	"k8s.io/helm/pkg/timeconv"
)

const templateDesc = `
Render chart templates locally and display the output.

This does not require Tiller. However, any values that would normally be
looked up or retrieved in-cluster will be faked locally. Additionally, none
of the server-side testing of chart validity (e.g. whether an API is supported)
is done.

To render just one template in a chart, use '-x':
	$ helm template mychart -x mychart/templates/deployment.yaml
`

type templateCmd struct {
	setVals     []string
	valsFiles   valueFiles
	flagVerbose bool
	showNotes   bool
	releaseName string
	namespace   string
	renderFiles []string

	out io.Writer
}

func newTemplateCmd(out io.Writer) *cobra.Command {
	tem := &templateCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "template [flags] CHART",
		Short: "locally render templates",
		Long:  templateDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("chart is required")
			}
			return tem.run(args)
		},
	}

	f := cmd.Flags()
	f.StringArrayVar(&tem.setVals, "set", []string{}, "set values on the command line. See 'helm install -h'")
	f.VarP(&tem.valsFiles, "values", "f", "specify one or more YAML files of values")
	f.BoolVarP(&tem.flagVerbose, "verbose", "v", false, "show the computed YAML values as well.")
	f.BoolVar(&tem.showNotes, "notes", false, "show the computed NOTES.txt file as well.")
	f.StringVarP(&tem.releaseName, "release", "r", "RELEASE-NAME", "release name")
	f.StringVarP(&tem.namespace, "namespace", "n", "NAMESPACE", "namespace")
	f.StringArrayVarP(&tem.renderFiles, "execute", "x", []string{}, "only execute the given templates.")

	return cmd
}

func (tc *templateCmd) run(args []string) error {
	c, err := chartutil.Load(args[0])
	if err != nil {
		return err
	}

	vv, err := tc.vals()
	if err != nil {
		return err
	}

	config := &chart.Config{Raw: string(vv), Values: map[string]*chart.Value{}}

	if tc.flagVerbose {
		fmt.Fprintf(tc.out, "---\n# merged values")
		fmt.Fprintf(tc.out, "%s\n", string(vv))

	}

	options := chartutil.ReleaseOptions{
		Name:      tc.releaseName,
		Time:      timeconv.Now(),
		Namespace: tc.namespace,
		//Revision:  1,
		//IsInstall: true,
	}

	// Set up engine.
	renderer := engine.New()

	vals, err := chartutil.ToRenderValues(c, config, options)
	if err != nil {
		return err
	}

	out, err := renderer.Render(c, vals)
	if err != nil {
		return err
	}

	in := func(needle string, haystack []string) bool {
		for _, h := range haystack {
			if h == needle {
				return true
			}
		}
		return false
	}

	sortedKeys := make([]string, 0, len(out))
	for key := range out {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// If renderFiles is set, we ONLY print those.
	if len(tc.renderFiles) > 0 {
		for _, name := range sortedKeys {
			data := out[name]
			if in(name, tc.renderFiles) {
				fmt.Fprintf(tc.out, "---\n# Source: %s\n", name)
				fmt.Fprintf(tc.out, "%s\n", data)
			}
		}
		return nil
	}

	for _, name := range sortedKeys {
		data := out[name]
		b := filepath.Base(name)
		if !tc.showNotes && b == "NOTES.txt" {
			continue
		}
		if strings.HasPrefix(b, "_") {
			continue
		}
		fmt.Fprintf(tc.out, "---\n# Source: %s\n", name)
		fmt.Fprintf(tc.out, "%s\n", data)
	}
	return nil
}

func (tc *templateCmd) vals() ([]byte, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range tc.valsFiles {
		currentMap := map[string]interface{}{}
		bytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return []byte{}, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return []byte{}, fmt.Errorf("failed to parse %s: %s", filePath, err)
		}
		// Merge with the previous map
		base = mergeValues(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range tc.setVals {
		if err := strvals.ParseInto(value, base); err != nil {
			return []byte{}, fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	return yaml.Marshal(base)
}
