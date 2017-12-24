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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/timeconv"
	tversion "k8s.io/helm/pkg/version"
)

const defaultDirectoryPermission = 0755

var (
	whitespaceRegex = regexp.MustCompile(`^\s*$`)

	// defaultKubeVersion is the default value of --kube-version flag
	defaultKubeVersion = fmt.Sprintf("%s.%s", chartutil.DefaultKubeVersion.Major, chartutil.DefaultKubeVersion.Minor)
)

const templateDesc = `
Render chart templates locally and display the output.

This does not require Tiller. However, any values that would normally be
looked up or retrieved in-cluster will be faked locally. Additionally, none
of the server-side testing of chart validity (e.g. whether an API is supported)
is done.

To render just one template in a chart, use '-x':

	$ helm template mychart -x templates/deployment.yaml
`

type templateCmd struct {
	namespace    string
	valueFiles   valueFiles
	chartPath    string
	out          io.Writer
	values       []string
	nameTemplate string
	showNotes    bool
	releaseName  string
	renderFiles  []string
	kubeVersion  string
	outputDir    string
}

func newTemplateCmd(out io.Writer) *cobra.Command {

	t := &templateCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "template [flags] CHART",
		Short: fmt.Sprintf("locally render templates"),
		Long:  templateDesc,
		RunE:  t.run,
	}

	f := cmd.Flags()
	f.BoolVar(&t.showNotes, "notes", false, "show the computed NOTES.txt file as well")
	f.StringVarP(&t.releaseName, "name", "n", "RELEASE-NAME", "release name")
	f.StringArrayVarP(&t.renderFiles, "execute", "x", []string{}, "only execute the given templates")
	f.VarP(&t.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringVar(&t.namespace, "namespace", "", "namespace to install the release into")
	f.StringArrayVar(&t.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringVar(&t.nameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&t.kubeVersion, "kube-version", defaultKubeVersion, "kubernetes version used as Capabilities.KubeVersion.Major/Minor")
	f.StringVar(&t.outputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")

	return cmd
}

func (t *templateCmd) run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("chart is required")
	}
	// verify chart path exists
	if _, err := os.Stat(args[0]); err == nil {
		if t.chartPath, err = filepath.Abs(args[0]); err != nil {
			return err
		}
	} else {
		return err
	}
	// verify specified templates exist relative to chart
	rf := []string{}
	var af string
	var err error
	if len(t.renderFiles) > 0 {
		for _, f := range t.renderFiles {
			if !filepath.IsAbs(f) {
				af, err = filepath.Abs(t.chartPath + "/" + f)
				if err != nil {
					return fmt.Errorf("could not resolve template path: %s", err)
				}
			} else {
				af = f
			}
			rf = append(rf, af)

			if _, err := os.Stat(af); err != nil {
				return fmt.Errorf("could not resolve template path: %s", err)
			}
		}
	}

	// verify that output-dir exists if provided
	if t.outputDir != "" {
		_, err = os.Stat(t.outputDir)
		if os.IsNotExist(err) {
			return fmt.Errorf("output-dir '%s' does not exist", t.outputDir)
		}
	}

	if t.namespace == "" {
		t.namespace = defaultNamespace()
	}
	// get combined values and create config
	rawVals, err := vals(t.valueFiles, t.values)
	if err != nil {
		return err
	}
	config := &chart.Config{Raw: string(rawVals), Values: map[string]*chart.Value{}}

	// If template is specified, try to run the template.
	if t.nameTemplate != "" {
		t.releaseName, err = generateName(t.nameTemplate)
		if err != nil {
			return err
		}
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	c, err := chartutil.Load(t.chartPath)
	if err != nil {
		return prettyError(err)
	}

	if req, err := chartutil.LoadRequirements(c); err == nil {
		if err := checkDependencies(c, req); err != nil {
			return prettyError(err)
		}
	} else if err != chartutil.ErrRequirementsNotFound {
		return fmt.Errorf("cannot load requirements: %v", err)
	}
	options := chartutil.ReleaseOptions{
		Name:      t.releaseName,
		Time:      timeconv.Now(),
		Namespace: t.namespace,
	}

	err = chartutil.ProcessRequirementsEnabled(c, config)
	if err != nil {
		return err
	}
	err = chartutil.ProcessRequirementsImportValues(c)
	if err != nil {
		return err
	}

	// Set up engine.
	renderer := engine.New()

	caps := &chartutil.Capabilities{
		APIVersions:   chartutil.DefaultVersionSet,
		KubeVersion:   chartutil.DefaultKubeVersion,
		TillerVersion: tversion.GetVersionProto(),
	}

	// kubernetes version
	kv, err := semver.NewVersion(t.kubeVersion)
	if err != nil {
		return fmt.Errorf("could not parse a kubernetes version: %v", err)
	}
	caps.KubeVersion.Major = fmt.Sprint(kv.Major())
	caps.KubeVersion.Minor = fmt.Sprint(kv.Minor())
	caps.KubeVersion.GitVersion = fmt.Sprintf("v%d.%d.0", kv.Major(), kv.Minor())

	vals, err := chartutil.ToRenderValuesCaps(c, config, options, caps)
	if err != nil {
		return err
	}

	out, err := renderer.Render(c, vals)
	listManifests := []tiller.Manifest{}
	if err != nil {
		return err
	}
	// extract kind and name
	re := regexp.MustCompile("kind:(.*)\n")
	for k, v := range out {
		match := re.FindStringSubmatch(v)
		h := "Unknown"
		if len(match) == 2 {
			h = strings.TrimSpace(match[1])
		}
		m := tiller.Manifest{Name: k, Content: v, Head: &util.SimpleHead{Kind: h}}
		listManifests = append(listManifests, m)
	}
	in := func(needle string, haystack []string) bool {
		// make needle path absolute
		d := strings.Split(needle, "/")
		dd := d[1:]
		an := t.chartPath + "/" + strings.Join(dd, "/")

		for _, h := range haystack {
			if h == an {
				return true
			}
		}
		return false
	}
	if settings.Debug {
		rel := &release.Release{
			Name:      t.releaseName,
			Chart:     c,
			Config:    config,
			Version:   1,
			Namespace: t.namespace,
			Info:      &release.Info{LastDeployed: timeconv.Timestamp(time.Now())},
		}
		printRelease(os.Stdout, rel)
	}

	for _, m := range tiller.SortByKind(listManifests) {
		if len(t.renderFiles) > 0 && !in(m.Name, rf) {
			continue
		}
		data := m.Content
		b := filepath.Base(m.Name)
		if !t.showNotes && b == "NOTES.txt" {
			continue
		}
		if strings.HasPrefix(b, "_") {
			continue
		}

		if t.outputDir != "" {
			// blank template after execution
			if whitespaceRegex.MatchString(data) {
				continue
			}
			err = writeToFile(t.outputDir, m.Name, data)
			if err != nil {
				return err
			}
			continue
		}
		fmt.Printf("---\n# Source: %s\n", m.Name)
		fmt.Println(data)
	}
	return nil
}

// write the <data> to <output-dir>/<name>
func writeToFile(outputDir string, name string, data string) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := os.Create(outfileName)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("##---\n# Source: %s\n%s", name, data))

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

// check if the directory exists to create file. creates if don't exists
func ensureDirectoryForFile(file string) error {
	baseDir := path.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(baseDir, defaultDirectoryPermission)
}
