/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/repo"
)

type repoIndexCmd struct {
	dir string
	url string
	out io.Writer
}

func newRepoIndexCmd(out io.Writer) *cobra.Command {
	index := &repoIndexCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "index [flags] [DIR]",
		Short: "generate an index file given a directory containing packaged charts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "path to a directory"); err != nil {
				return err
			}

			index.dir = args[0]

			return index.run()
		},
	}

	f := cmd.Flags()
	f.StringVar(&index.url, "url", "", "url of chart repository")

	return cmd
}

func (i *repoIndexCmd) run() error {
	path, err := filepath.Abs(i.dir)
	if err != nil {
		return err
	}

	return index(path, i.url)
}

func index(dir, url string) error {
	chartRepo, err := repo.LoadChartRepository(dir, url)
	if err != nil {
		return err
	}

	return chartRepo.Index()
}
