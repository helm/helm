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

package repo // import "k8s.io/helm/pkg/repo"

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/tlsutil"
	"k8s.io/helm/pkg/urlutil"
)

// Entry represents a collection of parameters for chart repository
type Entry struct {
	Name     string `json:"name"`
	Cache    string `json:"cache"`
	URL      string `json:"url"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
	CAFile   string `json:"caFile"`
}

// ChartRepository represents a chart repository
type ChartRepository struct {
	Config     *Entry
	ChartPaths []string
	IndexFile  *IndexFile
	Client     *http.Client
}

// Getter is an interface to support GET to the specified URL.
type Getter interface {
	Get(url string) (*http.Response, error)
}

// NewChartRepository constructs ChartRepository
func NewChartRepository(cfg *Entry) (*ChartRepository, error) {
	var client *http.Client
	if cfg.CertFile != "" && cfg.KeyFile != "" && cfg.CAFile != "" {
		tlsConf, err := tlsutil.NewClientTLS(cfg.CertFile, cfg.KeyFile, cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %s", err.Error())
		}
		tlsConf.BuildNameToCertificate()

		sni, err := urlutil.ExtractHostname(cfg.URL)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
			},
		}
	} else {
		client = http.DefaultClient
	}

	return &ChartRepository{
		Config:    cfg,
		IndexFile: NewIndexFile(),
		Client:    client,
	}, nil
}

// Get issues a GET using configured client to the specified URL.
func (r *ChartRepository) Get(url string) (*http.Response, error) {
	resp, err := r.Client.Get(url)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Load loads a directory of charts as if it were a repository.
//
// It requires the presence of an index.yaml file in the directory.
func (r *ChartRepository) Load() error {
	dirInfo, err := os.Stat(r.Config.Name)
	if err != nil {
		return err
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("%q is not a directory", r.Config.Name)
	}

	// FIXME: Why are we recursively walking directories?
	// FIXME: Why are we not reading the repositories.yaml to figure out
	// what repos to use?
	filepath.Walk(r.Config.Name, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if strings.Contains(f.Name(), "-index.yaml") {
				i, err := LoadIndexFile(path)
				if err != nil {
					return nil
				}
				r.IndexFile = i
			} else if strings.HasSuffix(f.Name(), ".tgz") {
				r.ChartPaths = append(r.ChartPaths, path)
			}
		}
		return nil
	})
	return nil
}

// DownloadIndexFile fetches the index from a repository.
//
// cachePath is prepended to any index that does not have an absolute path. This
// is for pre-2.2.0 repo files.
func (r *ChartRepository) DownloadIndexFile(cachePath string) error {
	var indexURL string

	indexURL = strings.TrimSuffix(r.Config.URL, "/") + "/index.yaml"
	resp, err := r.Get(indexURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	index, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if _, err := loadIndex(index); err != nil {
		return err
	}

	// In Helm 2.2.0 the config.cache was accidentally switched to an absolute
	// path, which broke backward compatibility. This fixes it by prepending a
	// global cache path to relative paths.
	//
	// It is changed on DownloadIndexFile because that was the method that
	// originally carried the cache path.
	cp := r.Config.Cache
	if !filepath.IsAbs(cp) {
		cp = filepath.Join(cachePath, cp)
	}
	println("Writing to", cp)

	return ioutil.WriteFile(cp, index, 0644)
}

// Index generates an index for the chart repository and writes an index.yaml file.
func (r *ChartRepository) Index() error {
	err := r.generateIndex()
	if err != nil {
		return err
	}
	return r.saveIndexFile()
}

func (r *ChartRepository) saveIndexFile() error {
	index, err := yaml.Marshal(r.IndexFile)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(r.Config.Name, indexPath), index, 0644)
}

func (r *ChartRepository) generateIndex() error {
	for _, path := range r.ChartPaths {
		ch, err := chartutil.Load(path)
		if err != nil {
			return err
		}

		digest, err := provenance.DigestFile(path)
		if err != nil {
			return err
		}

		if !r.IndexFile.Has(ch.Metadata.Name, ch.Metadata.Version) {
			r.IndexFile.Add(ch.Metadata, path, r.Config.URL, digest)
		}
		// TODO: If a chart exists, but has a different Digest, should we error?
	}
	r.IndexFile.SortEntries()
	return nil
}
