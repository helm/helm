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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/repo"
)

var untarFile bool
var untarDir string

func init() {
	RootCommand.AddCommand(fetchCmd)
	fetchCmd.Flags().BoolVar(&untarFile, "untar", false, "If set to true, will untar the chart after downloading it.")
	fetchCmd.Flags().StringVar(&untarDir, "untardir", ".", "If untar is specified, this flag specifies where to untar the chart.")
}

var fetchCmd = &cobra.Command{
	Use:   "fetch [chart URL | repo/chartname]",
	Short: "download a chart from a repository and (optionally) unpack it in local directory",
	Long:  "",
	RunE:  fetch,
}

func fetch(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("This command needs at least one argument, url or repo/name of the chart.")
	}

	pname := args[0]
	if filepath.Ext(pname) != ".tgz" {
		pname += ".tgz"
	}
	return fetchChart(pname)

}

func fetchChart(pname string) error {

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	// get download url
	u, err := mapRepoArg(pname, f.Repositories)
	if err != nil {
		return err
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to fetch %s : %s", u.String(), resp.Status)
	}

	defer resp.Body.Close()
	if untarFile {
		return chartutil.Expand(untarDir, resp.Body)
	}
	p := strings.Split(u.String(), "/")
	return saveChartFile(p[len(p)-1], resp.Body)
}

// mapRepoArg figures out which format the argument is given, and creates a fetchable
// url from it.
func mapRepoArg(arg string, r map[string]string) (*url.URL, error) {
	// See if it's already a full URL.
	u, err := url.ParseRequestURI(arg)
	if err == nil {
		// If it has a scheme and host and path, it's a full URL
		if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
			return u, nil
		}
		return nil, fmt.Errorf("Invalid chart url format: %s", arg)
	}
	// See if it's of the form: repo/path_to_chart
	p := strings.Split(arg, "/")
	if len(p) > 1 {
		if baseURL, ok := r[p[0]]; ok {
			if !strings.HasSuffix(baseURL, "/") {
				baseURL = baseURL + "/"
			}
			return url.ParseRequestURI(baseURL + strings.Join(p[1:], "/"))
		}
		return nil, fmt.Errorf("No such repo: %s", p[0])
	}
	return nil, fmt.Errorf("Invalid chart url format: %s", arg)
}

func saveChartFile(c string, r io.Reader) error {
	// Grab the chart name that we'll use for the name of the file to download to.
	out, err := os.Create(c)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	return err
}
