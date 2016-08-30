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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"k8s.io/helm/pkg/repo"
)

func init() {
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoIndexCmd)
	RootCommand.AddCommand(repoCmd)
}

var repoCmd = &cobra.Command{
	Use:   "repo add|remove|list [ARG]",
	Short: "add, list, or remove chart repositories",
}

var repoAddCmd = &cobra.Command{
	Use:   "add [flags] [NAME] [URL]",
	Short: "add a chart repository",
	RunE:  runRepoAdd,
}

var repoListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "list chart repositories",
	RunE:  runRepoList,
}

var repoRemoveCmd = &cobra.Command{
	Use:     "remove [flags] [NAME]",
	Aliases: []string{"rm"},
	Short:   "remove a chart repository",
	RunE:    runRepoRemove,
}

var repoIndexCmd = &cobra.Command{
	Use:   "index [flags] [DIR] [REPO_URL]",
	Short: "generate an index file for a chart repository given a directory",
	RunE:  runRepoIndex,
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(2, len(args), "name for the chart repository", "the url of the chart repository"); err != nil {
		return err
	}
	name, url := args[0], args[1]

	if err := addRepository(name, url); err != nil {
		return err
	}

	fmt.Println(name + " has been added to your repositories")
	return nil
}

func runRepoList(cmd *cobra.Command, args []string) error {
	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}
	if len(f.Repositories) == 0 {
		return errors.New("no repositories to show")
	}
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "URL")
	for k, v := range f.Repositories {
		table.AddRow(k, v)
	}
	fmt.Println(table)
	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(1, len(args), "name of chart repository"); err != nil {
		return err
	}
	return removeRepoLine(args[0])
}

func runRepoIndex(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("This command needs at minimum 1 argument:  a path to a directory")
	}

	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	url := ""
	if len(args) == 2 {
		url = args[1]
	}

	return index(path, url)
}

func index(dir, url string) error {
	chartRepo, err := repo.LoadChartRepository(dir, url)
	if err != nil {
		return err
	}

	return chartRepo.Index()
}

func addRepository(name, url string) error {
	if err := repo.DownloadIndexFile(name, url, cacheIndexFile(name)); err != nil {
		return errors.New("Looks like " + url + " is not a valid chart repository or cannot be reached: " + err.Error())
	}

	return insertRepoLine(name, url)
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
