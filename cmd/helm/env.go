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
	"log"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/cli/output"
)

var envHelp = `
Env prints out all the environment information in use by Helm.
`

const envOutputFlag string = "output"

func newEnvCmd(out io.Writer) *cobra.Command {
	outfmtEnv := keyValueENV
	cmd := &cobra.Command{
		Use:   "env",
		Short: "helm client environment information",
		Long:  envHelp,
		Args:  require.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				keys := getSortedEnvVarKeys()
				return keys, cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			envVars := settings.EnvVars()

			if len(args) == 0 {
				return outfmtEnv.WriteEnvs(out, envVars)
			}

			key := args[0]
			return outfmtEnv.WriteSingleEnv(out, key, envVars[key])
		},
	}

	bindEnvOutputFlag(cmd, &outfmtEnv)

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

type envs map[string]string
type envFormat string

const (
	keyValueENV envFormat = "env"
	jsonENV     envFormat = "json"
	yamlENV     envFormat = "yaml"
)

func envFormats() []string {
	return []string{keyValueENV.String(), jsonENV.String(), yamlENV.String()}
}

func envFormatWithDesc() map[string]string {
	return map[string]string{
		keyValueENV.String(): "Output result in KEY=VALUE format",
		jsonENV.String():     "Output result in JSON format",
		yamlENV.String():     "Output result in YAML format",
	}
}

func (o envFormat) String() string {
	return string(o)
}

func (o *envFormat) Set(s string) error {
	outfmt, err := parseFormat(s)
	if err != nil {
		return err
	}
	*o = outfmt
	return nil
}

func (o envFormat) Type() string {
	return "format"
}

func parseFormat(s string) (out envFormat, err error) {
	switch s {
	case keyValueENV.String():
		out, err = keyValueENV, nil
	case jsonENV.String():
		out, err = jsonENV, nil
	case yamlENV.String():
		out, err = yamlENV, nil
	default:
		out, err = "", output.ErrInvalidFormatType
	}
	return
}

func bindEnvOutputFlag(cmd *cobra.Command, varRef *envFormat) {
	cmd.Flags().VarP(varRef, envOutputFlag, "o",
		fmt.Sprintf("prints the output in the specified format. Allowed values: %s", strings.Join(envFormats(), ", ")))

	err := cmd.RegisterFlagCompletionFunc(envOutputFlag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var formatNames []string
		for format, desc := range envFormatWithDesc() {
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

func (o envFormat) WriteEnvs(out io.Writer, e envs) error {
	switch o {
	case keyValueENV:
		return writeKeyValues(out, e)
	case jsonENV:
		return output.EncodeJSON(out, e)
	case yamlENV:
		return output.EncodeYAML(out, e)
	default:
		return output.ErrInvalidFormatType
	}
}

func (o envFormat) WriteSingleEnv(out io.Writer, key, value string) error {
	switch o {
	case keyValueENV:
		fmt.Fprintf(out, "%s\n", value)
		return nil
	case jsonENV:
		return output.EncodeJSON(out, map[string]string{key: value})
	case yamlENV:
		return output.EncodeYAML(out, map[string]string{key: value})
	default:
		return output.ErrInvalidFormatType
	}
}

func writeKeyValues(out io.Writer, e envs) error {
	keys := getSortedEnvVarKeys()

	// Sort the variables by alphabetical order.
	// This allows for a constant output across calls to 'helm env'.
	for _, k := range keys {
		fmt.Fprintf(out, "%s=\"%s\"\n", k, e[k])
	}
	return nil

}
