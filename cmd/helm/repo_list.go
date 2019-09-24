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
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

type repoListCmd struct {
	out    io.Writer
	home   helmpath.Home
	output string
}

type repositoryElement struct {
	Name string
	URL  string
}

type repositories struct {
	Repositories []*repositoryElement
}

func newRepoListCmd(out io.Writer) *cobra.Command {
	list := &repoListCmd{out: out}

	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: "List chart repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			list.home = settings.Home
			return list.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&list.output, "output", "o", "table", "Prints the output in the specified format (json|table|yaml)")
	return cmd
}

func (a *repoListCmd) run() error {
	repoFile, err := repo.LoadRepositoriesFile(a.home.RepositoryFile())
	if err != nil {
		return err
	}

	output, err := formatRepoListResult(a.output, repoFile)

	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, output)

	return nil
}

func formatRepoListResult(format string, repoFile *repo.RepoFile) (string, error) {
	var output string
	var err error

	if len(repoFile.Repositories) == 0 {
		err = fmt.Errorf("no repositories to show")
		return output, err
	}

	switch format {
	case "table":
		table := uitable.New()
		table.AddRow("NAME", "URL")
		for _, re := range repoFile.Repositories {
			table.AddRow(re.Name, re.URL)
		}
		output = table.String()

	case "json":
		output, err = printFormatedRepoFile(format, repoFile, json.Marshal)

	case "yaml":
		output, err = printFormatedRepoFile(format, repoFile, yaml.Marshal)
	}

	return output, err
}

func printFormatedRepoFile(format string, repoFile *repo.RepoFile, obj func(v interface{}) ([]byte, error)) (string, error) {
	var output string
	var err error
	var repolist repositories

	for _, re := range repoFile.Repositories {
		repolist.Repositories = append(repolist.Repositories, &repositoryElement{Name: re.Name, URL: re.URL})
	}

	o, e := obj(repolist)
	if e != nil {
		err = fmt.Errorf("Failed to Marshal %s output: %s", strings.ToUpper(format), e)
	} else {
		output = string(o)
	}

	return output, err
}
