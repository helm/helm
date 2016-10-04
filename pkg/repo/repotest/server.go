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

package repotest

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/repo"
)

// NewServer creates a repository server for testing.
//
// docroot should be a temp dir managed by the caller.
//
// This will start the server, serving files off of the docroot.
//
// Use CopyCharts to move charts into the repository and then index them
// for service.
func NewServer(docroot string) *Server {
	root, err := filepath.Abs(docroot)
	if err != nil {
		panic(err)
	}
	srv := &Server{
		docroot: root,
	}
	srv.start()
	// Add the testing repository as the only repo.
	if err := setTestingRepository(docroot, "test", srv.URL()); err != nil {
		panic(err)
	}
	return srv
}

// Server is an implementaiton of a repository server for testing.
type Server struct {
	docroot string
	srv     *httptest.Server
}

// Root gets the docroot for the server.
func (s *Server) Root() string {
	return s.docroot
}

// CopyCharts takes a glob expression and copies those charts to the server root.
func (s *Server) CopyCharts(origin string) ([]string, error) {
	files, err := filepath.Glob(origin)
	if err != nil {
		return []string{}, err
	}
	copied := make([]string, len(files))
	for i, f := range files {
		base := filepath.Base(f)
		newname := filepath.Join(s.docroot, base)
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return []string{}, err
		}
		if err := ioutil.WriteFile(newname, data, 0755); err != nil {
			return []string{}, err
		}
		copied[i] = newname
	}

	err = s.CreateIndex()
	return copied, err
}

// CreateIndex will read docroot and generate an index.yaml file.
func (s *Server) CreateIndex() error {
	// generate the index
	index, err := repo.IndexDirectory(s.docroot, s.URL())
	if err != nil {
		return err
	}

	d, err := yaml.Marshal(index)
	if err != nil {
		return err
	}

	println(string(d))

	ifile := filepath.Join(s.docroot, "index.yaml")
	return ioutil.WriteFile(ifile, d, 0755)
}

func (s *Server) start() {
	s.srv = httptest.NewServer(http.FileServer(http.Dir(s.docroot)))
}

// Stop stops the server and closes all connections.
//
// It should be called explicitly.
func (s *Server) Stop() {
	s.srv.Close()
}

// URL returns the URL of the server.
//
// Example:
//	http://localhost:1776
func (s *Server) URL() string {
	return s.srv.URL
}

// setTestingRepository sets up a testing repository.yaml with only the given name/URL.
func setTestingRepository(helmhome, name, url string) error {
	rf := repo.NewRepoFile()
	rf.Add(&repo.Entry{Name: name, URL: url})
	os.MkdirAll(filepath.Join(helmhome, "repository", name), 0755)
	dest := filepath.Join(helmhome, "repository/repositories.yaml")
	return rf.WriteFile(dest, 0644)
}
