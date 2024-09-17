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
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/repo"
)

const (
	outputFlag         = "output"
	postRenderFlag     = "post-renderer"
	postRenderArgsFlag = "post-renderer-args"
)

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.FileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.StringArrayVar(&v.JSONValues, "set-json", []string{}, "set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)")
	f.StringArrayVar(&v.LiteralValues, "set-literal", []string{}, "set a literal STRING value on the command line")
}

func addChartPathOptionsFlags(f *pflag.FlagSet, c *action.ChartPathOptions) {
	f.StringVar(&c.Version, "version", "", "specify a version constraint for the chart version to use. This constraint can be a specific tag (e.g. 1.1.1) or it may reference a valid range (e.g. ^2.0.0). If this is not specified, the latest version is used")
	f.BoolVar(&c.Verify, "verify", false, "verify the package before using it")
	f.StringVar(&c.Keyring, "keyring", defaultKeyring(), "location of public keys used for verification")
	f.StringVar(&c.RepoURL, "repo", "", "chart repository url where to locate the requested chart")
	f.StringVar(&c.Username, "username", "", "chart repository username where to locate the requested chart")
	f.StringVar(&c.Password, "password", "", "chart repository password where to locate the requested chart")
	f.StringVar(&c.CertFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&c.KeyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.BoolVar(&c.InsecureSkipTLSverify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the chart download")
	f.BoolVar(&c.PlainHTTP, "plain-http", false, "use insecure HTTP connections for the chart download")
	f.StringVar(&c.CaFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&c.PassCredentialsAll, "pass-credentials", false, "pass credentials to all domains")
}

// bindOutputFlag will add the output flag to the given command and bind the
// value to the given format pointer
func bindOutputFlag(cmd *cobra.Command, varRef *output.Format) {
	cmd.Flags().VarP(newOutputValue(output.Table, varRef), outputFlag, "o",
		fmt.Sprintf("prints the output in the specified format. Allowed values: %s", strings.Join(output.Formats(), ", ")))

	err := cmd.RegisterFlagCompletionFunc(outputFlag, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		var formatNames []string
		for format, desc := range output.FormatsWithDesc() {
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
	p := &postRendererOptions{varRef, "", []string{}}
	cmd.Flags().Var(&postRendererString{p}, postRenderFlag, "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
	cmd.Flags().Var(&postRendererArgsSlice{p}, postRenderArgsFlag, "an argument to the post-renderer (can specify multiple)")
}

type postRendererOptions struct {
	renderer   *postrender.PostRenderer
	binaryPath string
	args       []string
}

type postRendererString struct {
	options *postRendererOptions
}

func (p *postRendererString) String() string {
	return p.options.binaryPath
}

func (p *postRendererString) Type() string {
	return "postRendererString"
}

func (p *postRendererString) Set(val string) error {
	if val == "" {
		return nil
	}
	p.options.binaryPath = val
	pr, err := postrender.NewExec(p.options.binaryPath, p.options.args...)
	if err != nil {
		return err
	}
	*p.options.renderer = pr
	return nil
}

type postRendererArgsSlice struct {
	options *postRendererOptions
}

func (p *postRendererArgsSlice) String() string {
	return "[" + strings.Join(p.options.args, ",") + "]"
}

func (p *postRendererArgsSlice) Type() string {
	return "postRendererArgsSlice"
}

func (p *postRendererArgsSlice) Set(val string) error {

	// a post-renderer defined by a user may accept empty arguments
	p.options.args = append(p.options.args, val)

	if p.options.binaryPath == "" {
		return nil
	}
	// overwrite if already create PostRenderer by `post-renderer` flags
	pr, err := postrender.NewExec(p.options.binaryPath, p.options.args...)
	if err != nil {
		return err
	}
	*p.options.renderer = pr
	return nil
}

func (p *postRendererArgsSlice) Append(val string) error {
	p.options.args = append(p.options.args, val)
	return nil
}

func (p *postRendererArgsSlice) Replace(val []string) error {
	p.options.args = val
	return nil
}

func (p *postRendererArgsSlice) GetSlice() []string {
	return p.options.args
}

func compVersionFlag(chartRef string, _ string) ([]string, cobra.ShellCompDirective) {
	chartInfo := strings.Split(chartRef, "/")
	if len(chartInfo) != 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	repoName := chartInfo[0]
	chartName := chartInfo[1]

	path := filepath.Join(settings.RepositoryCache, helmpath.CacheIndexFile(repoName))

	var versions []string
	if indexFile, err := repo.LoadIndexFile(path); err == nil {
		for _, details := range indexFile.Entries[chartName] {
			appVersion := details.Metadata.AppVersion
			appVersionDesc := ""
			if appVersion != "" {
				appVersionDesc = fmt.Sprintf("App: %s, ", appVersion)
			}
			created := details.Created.Format("January 2, 2006")
			createdDesc := ""
			if created != "" {
				createdDesc = fmt.Sprintf("Created: %s ", created)
			}
			deprecated := ""
			if details.Metadata.Deprecated {
				deprecated = "(deprecated)"
			}
			versions = append(versions, fmt.Sprintf("%s\t%s%s%s", details.Metadata.Version, appVersionDesc, createdDesc, deprecated))
		}
	}

	return versions, cobra.ShellCompDirectiveNoFileComp
}

// addKlogFlags adds flags from k8s.io/klog
// marks the flags as hidden to avoid polluting the help text
func addKlogFlags(fs *pflag.FlagSet) {
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = normalize(fl.Name)
		if fs.Lookup(fl.Name) != nil {
			return
		}
		newflag := pflag.PFlagFromGoFlag(fl)
		newflag.Hidden = true
		fs.AddFlag(newflag)
	})
}

// normalize replaces underscores with hyphens
func normalize(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}
