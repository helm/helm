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
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/postrender"
)

const outputFlag = "output"
const postRenderFlag = "post-renderer"

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.FileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

func addChartPathOptionsFlags(f *pflag.FlagSet, c *action.ChartPathOptions) {
	f.StringVar(&c.Version, "version", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.BoolVar(&c.Verify, "verify", false, "verify the package before installing it")
	f.StringVar(&c.Keyring, "keyring", defaultKeyring(), "location of public keys used for verification")
	f.StringVar(&c.RepoURL, "repo", "", "chart repository url where to locate the requested chart")
	f.StringVar(&c.Username, "username", "", "chart repository username where to locate the requested chart")
	f.StringVar(&c.Password, "password", "", "chart repository password where to locate the requested chart")
	f.StringVar(&c.CertFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&c.KeyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&c.CaFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
}

// bindOutputFlag will add the output flag to the given command and bind the
// value to the given format pointer
func bindOutputFlag(cmd *cobra.Command, varRef *output.Format) {
	f := cmd.Flags()
	flag := f.VarPF(newOutputValue(output.Table, varRef), outputFlag, "o",
		fmt.Sprintf("prints the output in the specified format. Allowed values: %s", strings.Join(output.Formats(), ", ")))

	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		var formatNames []string
		for _, format := range output.Formats() {
			if strings.HasPrefix(format, toComplete) {
				formatNames = append(formatNames, format)
			}
		}
		return formatNames, completion.BashCompDirectiveDefault
	})
}

type outputValue output.Format

func newOutputValue(defaultValue output.Format, p *output.Format) *outputValue {
	*p = defaultValue
	return (*outputValue)(p)
}

func (o *outputValue) String() string {
	// It is much cleaner looking (and technically less allocations) to just
	// convert to a string rather than type asserting to the underlying
	// output.Format
	return string(*o)
}

func (o *outputValue) Type() string {
	return "format"
}

func (o *outputValue) Set(s string) error {
	outfmt, err := output.ParseFormat(s)
	if err != nil {
		return err
	}
	*o = outputValue(outfmt)
	return nil
}

func bindPostRenderFlag(cmd *cobra.Command, varRef *postrender.PostRenderer) {
	cmd.Flags().Var(&postRenderer{varRef}, postRenderFlag, "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
}

type postRenderer struct {
	renderer *postrender.PostRenderer
}

func (p postRenderer) String() string {
	return "exec"
}

func (p postRenderer) Type() string {
	return "postrenderer"
}

func (p postRenderer) Set(s string) error {
	if s == "" {
		return nil
	}
	pr, err := postrender.NewExec(s)
	if err != nil {
		return err
	}
	*p.renderer = pr
	return nil
}
