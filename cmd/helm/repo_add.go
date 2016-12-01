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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

type repoAddCmd struct {
	name     string
	url      string
	home     helmpath.Home
	out      io.Writer
	noupdate bool
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
			add.home = helmpath.Home(homePath())

			return add.run()
		},
	}
	f := cmd.Flags()
	f.BoolVar(&add.noupdate, "no-update", false, "raise error if repo is already registered")
	return cmd
}

func (a *repoAddCmd) run() error {
	var err error
	if a.noupdate {
		err = addRepository(a.name, a.url, a.home)
	} else {
		err = updateRepository(a.name, a.url, a.home)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%q has been added to your repositories\n", a.name)
	return nil
}

func addRepository(name, url string, home helmpath.Home) error {
	cif := home.CacheIndex(name)
	if err := repo.DownloadIndexFile(name, url, cif); err != nil {
		return fmt.Errorf("Looks like %q is not a valid chart repository or cannot be reached: %s", url, err.Error())
	}

	return insertRepoLine(name, url, home)
}

func insertRepoLine(name, url string, home helmpath.Home) error {
	cif := home.CacheIndex(name)
	f, err := repo.LoadRepositoriesFile(home.RepositoryFile())
	if err != nil {
		return err
	}

	if f.Has(name) {
		return fmt.Errorf("The repository name you provided (%s) already exists. Please specify a different name.", name)
	}
	f.Add(&repo.Entry{
		Name:  name,
		URL:   strings.TrimSuffix(url, "/"),
		Cache: filepath.Base(cif),
	})
	return f.WriteFile(home.RepositoryFile(), 0644)
}

func updateRepository(name, url string, home helmpath.Home) error {
	cif := home.CacheIndex(name)
	if err := repo.DownloadIndexFile(name, url, cif); err != nil {
		return err
	}

	return updateRepoLine(name, url, home)
}

func updateRepoLine(name, url string, home helmpath.Home) error {
	cif := home.CacheIndex(name)
	f, err := repo.LoadRepositoriesFile(home.RepositoryFile())
	if err != nil {
		return err
	}

	f.Update(&repo.Entry{
		Name:  name,
		URL:   url,
		Cache: filepath.Base(cif),
	})

	return f.WriteFile(home.RepositoryFile(), 0666)
}
