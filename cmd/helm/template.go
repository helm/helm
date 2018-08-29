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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/tiller"
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

type templateOptions struct {
	nameTemplate string   // --name-template
	showNotes    bool     // --notes
	releaseName  string   // --name
	renderFiles  []string // --execute
	kubeVersion  string   // --kube-version
	outputDir    string   // --output-dir

	valuesOptions

	chartPath string
}

func newTemplateCmd(out io.Writer) *cobra.Command {
	o := &templateOptions{}

	cmd := &cobra.Command{
		Use:   "template CHART",
		Short: fmt.Sprintf("locally render templates"),
		Long:  templateDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// verify chart path exists
			if _, err := os.Stat(args[0]); err == nil {
				if o.chartPath, err = filepath.Abs(args[0]); err != nil {
					return err
				}
			} else {
				return err
			}
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.showNotes, "notes", false, "show the computed NOTES.txt file as well")
	f.StringVarP(&o.releaseName, "name", "", "RELEASE-NAME", "release name")
	f.StringArrayVarP(&o.renderFiles, "execute", "x", []string{}, "only execute the given templates")
	f.StringVar(&o.nameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&o.kubeVersion, "kube-version", defaultKubeVersion, "kubernetes version used as Capabilities.KubeVersion.Major/Minor")
	f.StringVar(&o.outputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	o.valuesOptions.addFlags(f)

	return cmd
}

func (o *templateOptions) run(out io.Writer) error {
	// verify specified templates exist relative to chart
	rf := []string{}
	var af string
	var err error
	if len(o.renderFiles) > 0 {
		for _, f := range o.renderFiles {
			if !filepath.IsAbs(f) {
				af, err = filepath.Abs(filepath.Join(o.chartPath, f))
				if err != nil {
					return errors.Wrap(err, "could not resolve template path")
				}
			} else {
				af = f
			}
			rf = append(rf, af)

			if _, err := os.Stat(af); err != nil {
				return errors.Wrap(err, "could not resolve template path")
			}
		}
	}

	// verify that output-dir exists if provided
	if o.outputDir != "" {
		if _, err := os.Stat(o.outputDir); os.IsNotExist(err) {
			return errors.Errorf("output-dir '%s' does not exist", o.outputDir)
		}
	}

	// get combined values and create config
	config, err := o.mergedValues()
	if err != nil {
		return err
	}

	// If template is specified, try to run the template.
	if o.nameTemplate != "" {
		o.releaseName, err = generateName(o.nameTemplate)
		if err != nil {
			return err
		}
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	c, err := loader.Load(o.chartPath)
	if err != nil {
		return err
	}

	if req := c.Requirements; req != nil {
		if err := checkDependencies(c, req); err != nil {
			return err
		}
	}
	options := chartutil.ReleaseOptions{
		Name: o.releaseName,
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(config, &m); err != nil {
		return err
	}
	if err := chartutil.ProcessRequirementsEnabled(c, m); err != nil {
		return err
	}
	if err := chartutil.ProcessRequirementsImportValues(c); err != nil {
		return err
	}

	// Set up engine.
	renderer := engine.New()

	// kubernetes version
	kv, err := semver.NewVersion(o.kubeVersion)
	if err != nil {
		return errors.Wrap(err, "could not parse a kubernetes version")
	}

	caps := chartutil.DefaultCapabilities
	caps.KubeVersion.Major = fmt.Sprint(kv.Major())
	caps.KubeVersion.Minor = fmt.Sprint(kv.Minor())
	caps.KubeVersion.GitVersion = fmt.Sprintf("v%d.%d.0", kv.Major(), kv.Minor())

	vals, err := chartutil.ToRenderValues(c, config, options, caps)
	if err != nil {
		return err
	}

	rendered, err := renderer.Render(c, vals)
	if err != nil {
		return err
	}

	listManifests := []tiller.Manifest{}
	// extract kind and name
	re := regexp.MustCompile("kind:(.*)\n")
	for k, v := range rendered {
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
		d := strings.Split(needle, string(os.PathSeparator))
		dd := d[1:]
		an := filepath.Join(o.chartPath, strings.Join(dd, string(os.PathSeparator)))

		for _, h := range haystack {
			if h == an {
				return true
			}
		}
		return false
	}

	if settings.Debug {
		rel := &release.Release{
			Name:    o.releaseName,
			Chart:   c,
			Config:  config,
			Version: 1,
			Info:    &release.Info{LastDeployed: time.Now()},
		}
		printRelease(out, rel)
	}

	for _, m := range tiller.SortByKind(listManifests) {
		b := filepath.Base(m.Name)
		switch {
		case len(o.renderFiles) > 0 && !in(m.Name, rf):
			continue
		case !o.showNotes && b == "NOTES.txt":
			continue
		case strings.HasPrefix(b, "_"):
			continue
		case whitespaceRegex.MatchString(m.Content):
			// blank template after execution
			continue
		case o.outputDir != "":
			if err := writeToFile(out, o.outputDir, m.Name, m.Content); err != nil {
				return err
			}
		default:
			fmt.Fprintf(out, "---\n# Source: %s\n", m.Name)
			fmt.Fprintln(out, m.Content)
		}
	}
	return nil
}

// write the <data> to <output-dir>/<name>
func writeToFile(out io.Writer, outputDir, name, data string) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	if err := ensureDirectoryForFile(outfileName); err != nil {
		return err
	}

	f, err := os.Create(outfileName)
	if err != nil {
		return err
	}

	defer f.Close()

	if _, err = f.WriteString(fmt.Sprintf("##---\n# Source: %s\n%s", name, data)); err != nil {
		return err
	}

	fmt.Fprintf(out, "wrote %s\n", outfileName)
	return nil
}

// check if the directory exists to create file. creates if don't exists
func ensureDirectoryForFile(file string) error {
	baseDir := path.Dir(file)
	if _, err := os.Stat(baseDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(baseDir, defaultDirectoryPermission)
}
