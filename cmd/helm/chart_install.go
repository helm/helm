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
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/cli/values"
)

const chartInstallDesc = `
This command installs a chart from registry.

The install argument must be an existing chart name from the registry and a valid tag.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'. In case a value is large and therefore
you want not to use neither '--values' nor '--set', use '--set-file' to read the
single large value from file.

    $ helm install -f myvalues.yaml myredis redis:5

or

    $ helm install --set name=prod myredis redis:5

or

    $ helm install --set-string long_int=1234567890 myredis  redis:5

or
    $ helm install --set-file my_script=dothings.sh myredis  redis:5

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

    $ helm install -f myvalues.yaml -f override.yaml  myredis  redis:5

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

    $ helm install --set foo=bar --set foo=newbar  myredis  redis:5


To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined.

If --verify is set, the chart MUST have a provenance file, and the provenance
file MUST pass all verification steps.
`

func newChartInstallCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewChartInstall(cfg)
	valueOpts := &values.Options{}
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:     "install [NAME] [CHART:TAG]",
		Aliases: []string{"i"},
		Short:   "install a chart from registry",
		Long:    chartInstallDesc,
		Args:    require.MinimumNArgs(2),
		Hidden:  !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ref := args[1]

			return client.Run(out, name, ref)
		},
	}

	addChartInstallFlags(cmd.Flags(), client, valueOpts)
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

func addChartInstallFlags(f *pflag.FlagSet, client *action.ChartInstall, valueOpts *values.Options) {
	f.BoolVar(&client.DryRun, "dry-run", false, "simulate an install")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&client.Replace, "replace", false, "re-use the given name, even if that name is already used. This is unsafe in production")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.Wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVarP(&client.GenerateName, "generate-name", "g", false, "generate the name (and omit the NAME parameter)")
	f.StringVar(&client.NameTemplate, "name-template", "", "specify template used to name the release")
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "run helm dependency update before installing the chart")
	f.BoolVar(&client.Atomic, "atomic", false, "if set, installation process purges chart on fail. The --wait flag will be set automatically if --atomic is used")
	f.BoolVar(&client.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed. By default, CRDs are installed if not already present")
	f.BoolVar(&client.SubNotes, "render-subchart-notes", false, "if set, render subchart notes along with the parent")
	addValueOptionsFlags(f, valueOpts)
}
