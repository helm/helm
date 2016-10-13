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

package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/provenance"
)

var localRepoPath string

// StartLocalRepo starts a web server and serves files from the given path
func StartLocalRepo(path, address string) error {
	if address == "" {
		address = ":8879"
	}
	localRepoPath = path
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/charts/", indexHandler)
	return http.ListenAndServe(address, nil)
}
func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Welcome to the Kubernetes Package manager!\nBrowse charts on localhost:8879/charts!")
}
func indexHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[len("/charts/"):]
	if len(strings.Split(file, ".")) > 1 {
		serveFile(w, r, file)
	} else if file == "" {
		fmt.Fprintf(w, "list of charts should be here at some point")
	} else if file == "index" {
		fmt.Fprintf(w, "index file data should be here at some point")
	} else {
		fmt.Fprintf(w, "Ummm... Nothing to see here folks")
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, file string) {
	http.ServeFile(w, r, filepath.Join(localRepoPath, file))
}

// AddChartToLocalRepo saves a chart in the given path and then reindexes the index file
func AddChartToLocalRepo(ch *chart.Chart, path string) error {
	_, err := chartutil.Save(ch, path)
	if err != nil {
		return err
	}
	return Reindex(ch, path+"/index.yaml")
}

// Reindex adds an entry to the index file at the given path
func Reindex(ch *chart.Chart, path string) error {
	name := ch.Metadata.Name + "-" + ch.Metadata.Version
	y, err := LoadIndexFile(path)
	if err != nil {
		return err
	}
	found := false
	for k := range y.Entries {
		if k == name {
			found = true
			break
		}
	}
	if !found {
		dig, err := provenance.DigestFile(path)
		if err != nil {
			return err
		}

		y.Add(ch.Metadata, name+".tgz", "http://localhost:8879/charts", "sha256:"+dig)

		out, err := yaml.Marshal(y)
		if err != nil {
			return err
		}

		ioutil.WriteFile(path, out, 0644)
	}
	return nil
}
