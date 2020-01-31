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

This tool is used for creating an 'index.yaml' and 'index.json' files for a chart repository. To
set an absolute URL to the charts, use '--url' flag.

To merge the generated index with an existing index file, use the '--merge'
flag for 'index.yaml' file and '--merge-json-index' for 'index.json' file. Only one of the
flags can be passed at once. In this case, the charts found in the current directory will be merged
into the existing index, with local charts taking priority over existing charts.
`

type repoIndexOptions struct {
	dir            string
	url            string
	merge          string
	mergeJSONIndex string
}

func newRepoIndexCmd(out io.Writer) *cobra.Command {
	o := &repoIndexOptions{}

	cmd := &cobra.Command{
		Use:   "index [DIR]",
		Short: "generate an index file given a directory containing packaged charts",
		Long:  repoIndexDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.dir = args[0]
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.url, "url", "", "url of chart repository")
	f.StringVar(&o.merge, "merge", "", "merge the generated index into the given index.yaml")
	f.StringVar(&o.mergeJSONIndex, "merge-json-index", "", "merge the generated index into the given index.json")

	return cmd
}

func (i *repoIndexOptions) run(out io.Writer) error {
	if i.merge != "" && i.mergeJSONIndex != "" {
		return errors.New("only one of --merge and --merge-json-index can be passed")
	}

	path, err := filepath.Abs(i.dir)
	if err != nil {
		return err
	}

	return index(path, i.url, i.merge, i.mergeJSONIndex)
}

func index(dir, url, mergeTo, mergeToJSON string) error {
	yamlIndex := filepath.Join(dir, "index.yaml")
	jsonIndex := filepath.Join(dir, "index.json")

	i, err := repo.IndexDirectory(dir, url)
	if err != nil {
		return err
	}
	if mergeTo != "" || mergeToJSON != "" {
		if err = mergeToIndex(i, mergeTo, mergeToJSON); err != nil {
			return err
		}
	}
	i.SortEntries()
	err = i.WriteFile(yamlIndex, 0644)
	if err != nil {
		return err
	}
	return i.WriteJSONFile(jsonIndex, 0644)
}

func mergeToIndex(i *repo.IndexFile, mergeTo, mergeToJSON string) error {
	// if index.yaml is missing then create an empty one to merge into
	var i2 *repo.IndexFile
	var err error
	if mergeTo != "" {
		i2, err = loadIndexForMerge(mergeTo)
		if err != nil {
			return err
		}
	} else if mergeToJSON != "" {
		i2, err = loadIndexJSONForMerge(mergeToJSON)
		if err != nil {
			return err
		}
	}
	i.Merge(i2)
	return nil
}

func loadIndexForMerge(mergeTo string) (*repo.IndexFile, error) {
	var i2 *repo.IndexFile
	if _, err := os.Stat(mergeTo); os.IsNotExist(err) {
		i2 = repo.NewIndexFile()
		i2.WriteFile(mergeTo, 0644)
	} else {
		i2, err = repo.LoadIndexFile(mergeTo)
		if err != nil {
			return nil, errors.Wrap(err, "merge failed")
		}
	}
	return i2, nil
}

func loadIndexJSONForMerge(mergeToJSON string) (*repo.IndexFile, error) {
	var i2 *repo.IndexFile
	if _, err := os.Stat(mergeToJSON); os.IsNotExist(err) {
		i2 = repo.NewIndexFile()
		i2.WriteJSONFile(mergeToJSON, 0644)
	} else {
		i2, err = repo.LoadIndexJSONFile(mergeToJSON)
		if err != nil {
			return nil, errors.Wrap(err, "merge failed")
		}
	}
	return i2, nil
}
