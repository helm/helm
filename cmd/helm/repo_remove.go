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
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"k8s.io/helm/pkg/repo"
)

type repoRemoveCmd struct {
	out  io.Writer
	name string
}

func newRepoRemoveCmd(out io.Writer) *cobra.Command {
	remove := &repoRemoveCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:     "remove [flags] [NAME]",
		Aliases: []string{"rm"},
		Short:   "remove a chart repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "name of chart repository"); err != nil {
				return err
			}
			remove.name = args[0]

			return remove.run()
		},
	}

	return cmd
}

func (r *repoRemoveCmd) run() error {
	return removeRepoLine(r.name)
}

func removeRepoLine(name string) error {
	r, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	_, ok := r.Repositories[name]
	if ok {
		delete(r.Repositories, name)
		b, err := yaml.Marshal(&r.Repositories)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(repositoriesFile(), b, 0666); err != nil {
			return err
		}
		if err := removeRepoCache(name); err != nil {
			return err
		}

	} else {
		return fmt.Errorf("The repository, %s, does not exist in your repositories list", name)
	}

	fmt.Println(name + " has been removed from your repositories")

	return nil
}

func removeRepoCache(name string) error {
	if _, err := os.Stat(cacheIndexFile(name)); err == nil {
		err = os.Remove(cacheIndexFile(name))
		if err != nil {
			return err
		}
	}
	return nil
}
