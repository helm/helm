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

package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/version"
	"helm.sh/helm/v4/pkg/cmd/require"
)

const versionDesc = `
Show the version for Helm.

This will print a representation the version of Helm.
The output will look something like this:

version.BuildInfo{Version:"v3.2.1", GitCommit:"fe51cd1e31e6a202cba7dead9552a6d418ded79a", GitTreeState:"clean", GoVersion:"go1.13.10"}

- Version is the semantic version of the release.
- GitCommit is the SHA for the commit that this version was built from.
- GitTreeState is "clean" if there are no local code changes when this binary was
  built, and "dirty" if the binary was built from locally modified code.
- GoVersion is the version of Go that was used to compile Helm.

The --output flag allows you to change the representation with the following options:

- "go": Prints the version in a raw Go struct.
- "human": Prints the version in a human-readable format.

When using the --template flag the following properties are available to use in
the template:

- .Version contains the semantic version of Helm
- .GitCommit is the git commit
- .GitTreeState is the state of the git tree when Helm was built
- .GoVersion contains the version of Go that Helm was compiled with

For example, --template='Version: {{.Version}}' outputs 'Version: v3.2.1'.
`

type versionOptions struct {
	short    bool
	template string
	output   string
}

func newVersionCmd(out io.Writer) *cobra.Command {
	o := &versionOptions{}

	cmd := &cobra.Command{
		Use:               "version",
		Short:             "print the helm version information",
		Long:              versionDesc,
		Args:              require.NoArgs,
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.run(out)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.short, "short", false, "print the version number")
	f.StringVar(&o.template, "template", "", "template for version string format")
	f.StringVarP(&o.output, "output", "o", "", "output format (options: \"go\", \"human\")")

	return cmd
}

func (o *versionOptions) run(out io.Writer) error {
	if o.template != "" {
		tt, err := template.New("_").Parse(o.template)
		if err != nil {
			return err
		}
		return tt.Execute(out, version.Get())
	}

	switch o.output {
	case "human":
		return printVersionHuman(out)
	case "go":
		fmt.Fprintln(out, formatVersion(false))
	case "":
		fmt.Fprintln(out, formatVersion(o.short))
		return nil
	default:
		return fmt.Errorf("invalid output format: %q (valid formats: \"go\", \"human\")", o.output)
	}
	return nil
}

func formatVersion(short bool) string {
	v := version.Get()
	if short {
		if len(v.GitCommit) >= 7 {
			return fmt.Sprintf("%s+g%s", v.Version, v.GitCommit[:7])
		}
		return version.GetVersion()
	}
	return fmt.Sprintf("%#v", v)
}

func printVersionHuman(out io.Writer) error {
	v := version.Get()

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Version:\t%s\n", v.Version)
	fmt.Fprintf(w, "Git Commit:\t%s\n", v.GitCommit)
	fmt.Fprintf(w, "Git Tree State:\t%s\n", v.GitTreeState)
	fmt.Fprintf(w, "Go Version:\t%s\n", v.GoVersion)
	fmt.Fprintf(w, "KubeClient Version:\t%s\n", v.KubeClientVersion)

	return w.Flush()
}
