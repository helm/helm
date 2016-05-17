package repo

import (
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"
)

// IndexFile represents the index file in a chart repository
type IndexFile struct {
	Entries map[string]*ChartRef
}

// ChartRef represents a chart entry in the IndexFile
type ChartRef struct {
	Name     string   `yaml:"name"`
	URL      string   `yaml:"url"`
	Keywords []string `yaml:"keywords"`
	Removed  bool     `yaml:"removed,omitempty"`
}

// DownloadIndexFile uses
func DownloadIndexFile(repoName, url, indexFileName string) error {
	var indexURL string

	indexURL = strings.TrimSuffix(url, "/") + "/index.yaml"
	resp, err := http.Get(indexURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r IndexFile

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(b, &r); err != nil {
		return err
	}

	if err := ioutil.WriteFile(indexFileName, b, 0644); err != nil {
		return err
	}

	return nil
}
