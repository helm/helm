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
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/repo"
)

const repoIndexDesc = `
Read the current directory and generate an index file based on the charts found.

This tool is used for creating an 'index.yaml' file for a chart repository. To
set an absolute URL to the charts, use '--url' flag.

To merge the generated index with an existing index file, use the '--merge'
flag. In this case, the charts found in the current directory will be merged
into the existing index, with local charts taking priority over existing charts.
`

type repoIndexOptions struct {
	dir   string
	url   string
	merge string
	json  bool
}

func newRepoIndexCmd(out io.Writer) *cobra.Command {
	o := &repoIndexOptions{}

	cmd := &cobra.Command{
		Use:   "index [DIR]",
		Short: "generate an index file given a directory containing packaged charts",
		Long:  repoIndexDesc,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Allow file completion when completing the argument for the directory
				return nil, cobra.ShellCompDirectiveDefault
			}
			// No more completions, so disable file completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			o.dir = args[0]
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.url, "url", "", "url of chart repository")
	f.StringVar(&o.merge, "merge", "", "merge the generated index into the given index")
	f.BoolVar(&o.json, "json", false, "output in JSON format")

	return cmd
}

func (i *repoIndexOptions) run(_ io.Writer) error {
	path, err := filepath.Abs(i.dir)
	if err != nil {
		return err
	}

	return index(path, i.url, i.merge, i.json)
}

func index(dir, url, mergeTo string, json bool) error {
	out := filepath.Join(dir, "index.yaml")

	i, err := repo.IndexDirectory(dir, url)
	if err != nil {
		return err
	}
	if mergeTo != "" {
		// if index.yaml is missing then create an empty one to merge into
		var i2 *repo.IndexFile
		if _, err := os.Stat(mergeTo); os.IsNotExist(err) {
			i2 = repo.NewIndexFile()
			writeIndexFile(i2, mergeTo, json)
		} else {
			i2, err = repo.LoadIndexFile(mergeTo)
			if err != nil {
				return errors.Wrap(err, "merge failed")
			}
		}
		i.Merge(i2)
	}
	i.SortEntries()
	return writeIndexFile(i, out, json)
}

func writeIndexFile(i *repo.IndexFile, out string, json bool) error {
	if json {
		return i.WriteJSONFile(out, 0644)
	}
	return i.WriteFile(out, 0644)
}
