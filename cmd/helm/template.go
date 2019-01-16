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
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	fakedisc "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller/environment"
)

// defaultKubeVersion is the default value of --kube-version flag
var defaultKubeVersion = fmt.Sprintf("%s.%s", chartutil.DefaultKubeVersion.Major, chartutil.DefaultKubeVersion.Minor)

const templateDesc = `
Render chart templates locally and display the output.

This does not require a Kubernetes connection. However, any values that would normally
be retrieved in-cluster will be faked locally. Additionally, no validation is
performed on the resulting manifest files. As a result, there is no assurance that a
file generated from this command will be valid to Kubernetes.

`

type templateOptions struct {
	nameTemplate string // --name-template
	showNotes    bool   // --notes
	releaseName  string // --name
	kubeVersion  string // --kube-version
	outputDir    string // --output-dir

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
	f.StringVar(&o.nameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&o.kubeVersion, "kube-version", defaultKubeVersion, "kubernetes version used as Capabilities.KubeVersion.Major/Minor")
	f.StringVar(&o.outputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	o.valuesOptions.addFlags(f)

	return cmd
}

func (o *templateOptions) run(out io.Writer) error {
	// get combined values and create config
	config, err := o.mergedValues()
	if err != nil {
		return err
	}

	// If template is specified, try to run the template.
	if o.nameTemplate != "" {
		o.releaseName, err = templateName(o.nameTemplate)
		if err != nil {
			return err
		}
	}

	// Check chart dependencies to make sure all are present in /charts
	c, err := loader.Load(o.chartPath)
	if err != nil {
		return err
	}

	if req := c.Metadata.Dependencies; req != nil {
		if err := checkDependencies(c, req); err != nil {
			return err
		}
	}

	if err := chartutil.ProcessDependencies(c, config); err != nil {
		return err
	}

	// MPB: It appears that we can now do everything we need with an install
	// dry-run using the Kubernetes mock
	disc, err := createFakeDiscovery(o.kubeVersion)
	if err != nil {
		return err
	}
	customConfig := &action.Configuration{
		// Add mock objects in here so it doesn't use Kube API server
		Releases:   storage.Init(driver.NewMemory()),
		KubeClient: &environment.PrintingKubeClient{Out: ioutil.Discard},
		Discovery:  disc,
		Log: func(format string, v ...interface{}) {
			fmt.Fprintf(out, format, v...)
		},
	}
	inst := action.NewInstall(customConfig)
	inst.DryRun = true
	inst.Replace = true // Skip running the name check
	inst.ReleaseName = o.releaseName
	rel, err := inst.Run(c, config)
	if err != nil {
		return err
	}

	if o.outputDir != "" {
		return o.writeAsFiles(rel)

	}
	fmt.Fprintln(out, rel.Manifest)
	if o.showNotes {
		fmt.Fprintf(out, "---\n# Source: %s/templates/NOTES.txt\n", c.Name())
		fmt.Fprintln(out, rel.Info.Notes)
	}
	return nil

	/*

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
	*/
}

func (o *templateOptions) writeAsFiles(rel *release.Release) error {
	if _, err := os.Stat(o.outputDir); os.IsNotExist(err) {
		return errors.Errorf("output-dir '%s' does not exist", o.outputDir)
	}
	// At one point we parsed out the returned manifest and created multiple files.
	// I'm not totally sure what the use case was for that.
	filename := filepath.Join(o.outputDir, rel.Name+".yaml")
	return ioutil.WriteFile(filename, []byte(rel.Manifest), 0644)
}

// createFakeDiscovery creates a discovery client and seeds it with mock data.
func createFakeDiscovery(verStr string) (discovery.DiscoveryInterface, error) {
	disc := fake.NewSimpleClientset().Discovery()
	if verStr != "" {
		kv, err := semver.NewVersion(verStr)
		if err != nil {
			return disc, errors.Wrap(err, "could not parse a kubernetes version")
		}
		disc.(*fakedisc.FakeDiscovery).FakedServerVersion = &version.Info{
			Major:      fmt.Sprintf("%d", kv.Major()),
			Minor:      fmt.Sprintf("%d", kv.Minor()),
			GitVersion: fmt.Sprintf("v%d.%d.0", kv.Major(), kv.Minor()),
		}
	}
	return disc, nil
}

// write the <data> to <output-dir>/<name>
/*
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
*/
