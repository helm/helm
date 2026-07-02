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
	"log"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
)

var envHelp = `
Env prints out all the environment information in use by Helm.
`

func newEnvCmd(out io.Writer) *cobra.Command {
	outfmt := envFormatKeyValue
	cmd := &cobra.Command{
		Use:   "env",
		Short: "helm client environment information",
		Long:  envHelp,
		Args:  require.MaximumNArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				keys := getSortedEnvVarKeys()
				return keys, cobra.ShellCompDirectiveNoFileComp
			}

			return noMoreArgsComp()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			envVars := settings.EnvVars()

			if len(args) == 1 {
				return outfmt.writeSingle(out, args[0], envVars[args[0]])
			}
			return outfmt.write(out, envVars)
		},
	}

	bindEnvOutputFlag(cmd, &outfmt)

	return cmd
}

func getSortedEnvVarKeys() []string {
	envVars := settings.EnvVars()

	var keys []string
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

// envFormat is the set of output formats supported by 'helm env'. The
// KEY="VALUE" shell format predates the structured formats and stays the
// default; it is not part of output.Format because no other command emits it.
type envFormat string

const (
	envFormatKeyValue envFormat = "env"
	envFormatJSON     envFormat = "json"
	envFormatYAML     envFormat = "yaml"
)

func envFormats() []string {
	return []string{string(envFormatKeyValue), string(envFormatJSON), string(envFormatYAML)}
}

func envFormatsWithDesc() map[string]string {
	return map[string]string{
		string(envFormatKeyValue): `Output result in KEY="VALUE" format`,
		string(envFormatJSON):     "Output result in JSON format",
		string(envFormatYAML):     "Output result in YAML format",
	}
}

// String, Set and Type make envFormat usable as a pflag.Value.
func (o *envFormat) String() string { return string(*o) }

func (o *envFormat) Type() string { return "format" }

func (o *envFormat) Set(s string) error {
	if !slices.Contains(envFormats(), s) {
		return output.ErrInvalidFormatType
	}
	*o = envFormat(s)
	return nil
}

// write prints every variable in the given format.
func (o envFormat) write(out io.Writer, envVars map[string]string) error {
	switch o {
	case envFormatKeyValue:
		// Sort the variables by alphabetical order.
		// This allows for a constant output across calls to 'helm env'.
		for _, k := range slices.Sorted(maps.Keys(envVars)) {
			fmt.Fprintf(out, "%s=\"%s\"\n", k, envVars[k])
		}
		return nil
	case envFormatJSON:
		return output.EncodeJSON(out, envVars)
	case envFormatYAML:
		return output.EncodeYAML(out, envVars)
	}
	return output.ErrInvalidFormatType
}

// writeSingle prints one variable. The key=value format keeps the historic
// behavior of printing the bare value only.
func (o envFormat) writeSingle(out io.Writer, key, value string) error {
	switch o {
	case envFormatKeyValue:
		fmt.Fprintf(out, "%s\n", value)
		return nil
	case envFormatJSON:
		return output.EncodeJSON(out, map[string]string{key: value})
	case envFormatYAML:
		return output.EncodeYAML(out, map[string]string{key: value})
	}
	return output.ErrInvalidFormatType
}

func bindEnvOutputFlag(cmd *cobra.Command, varRef *envFormat) {
	cmd.Flags().VarP(varRef, outputFlag, "o",
		"prints the output in the specified format. Allowed values: "+strings.Join(envFormats(), ", "))

	err := cmd.RegisterFlagCompletionFunc(outputFlag, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		var formatNames []string
		for format, desc := range envFormatsWithDesc() {
			formatNames = append(formatNames, fmt.Sprintf("%s\t%s", format, desc))
		}

		// Sort the results to get a deterministic order for the tests
		sort.Strings(formatNames)
		return formatNames, cobra.ShellCompDirectiveNoFileComp
	})

	if err != nil {
		log.Fatal(err)
	}
}
