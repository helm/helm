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
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"k8s.io/helm/pkg/repo"
)

type repoAddCmd struct {
	name string
	url  string
	out  io.Writer
}

func newRepoAddCmd(out io.Writer) *cobra.Command {
	add := &repoAddCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "add [flags] [NAME] [URL]",
		Short: "add a chart repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "name for the chart repository", "the url of the chart repository"); err != nil {
				return err
			}

			add.name = args[0]
			add.url = args[1]

			return add.run()
		},
	}
	return cmd
}

func (a *repoAddCmd) run() error {
	if err := addRepository(a.name, a.url); err != nil {
		return err
	}

	fmt.Println(a.name + " has been added to your repositories")
	return nil
}

func addRepository(name, url string) error {
	if err := repo.DownloadIndexFile(name, url, cacheIndexFile(name)); err != nil {
		return errors.New("Looks like " + url + " is not a valid chart repository or cannot be reached: " + err.Error())
	}

	return insertRepoLine(name, url)
}

func insertRepoLine(name, url string) error {
	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}
	_, ok := f.Repositories[name]
	if ok {
		return fmt.Errorf("The repository name you provided (%s) already exists. Please specify a different name.", name)
	}

	if f.Repositories == nil {
		f.Repositories = make(map[string]string)
	}

	f.Repositories[name] = url

	b, _ := yaml.Marshal(&f.Repositories)
	return ioutil.WriteFile(repositoriesFile(), b, 0666)
}
