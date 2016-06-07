package repo

import (
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

var indexPath = "index.yaml"

// IndexFile represents the index file in a chart repository
type IndexFile struct {
	Entries map[string]*ChartRef
}

// ChartRef represents a chart entry in the IndexFile
type ChartRef struct {
	Name      string          `yaml:"name"`
	URL       string          `yaml:"url"`
	Created   string          `yaml:"created,omitempty"`
	Removed   bool            `yaml:"removed,omitempty"`
	Checksum  string          `yaml:"checksum,omitempty"`
	Chartfile *chart.Metadata `yaml:"chartfile"`
}

// DownloadIndexFile uses
func DownloadIndexFile(repoName, url, indexFilePath string) error {
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

	return ioutil.WriteFile(indexFilePath, b, 0644)
}

// UnmarshalYAML unmarshals the index file
func (i *IndexFile) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var refs map[string]*ChartRef
	if err := unmarshal(&refs); err != nil {
		if _, ok := err.(*yaml.TypeError); !ok {
			return err
		}
	}
	i.Entries = refs
	return nil
}

func (i *IndexFile) addEntry(name string, url string) ([]byte, error) {
	if i.Entries == nil {
		i.Entries = make(map[string]*ChartRef)
	}
	entry := ChartRef{Name: name, URL: url}
	i.Entries[name] = &entry
	out, err := yaml.Marshal(&i.Entries)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// LoadIndexFile takes a file at the given path and returns an IndexFile object
func LoadIndexFile(path string) (*IndexFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var indexfile IndexFile
	err = yaml.Unmarshal(b, &indexfile)
	if err != nil {
		return nil, err
	}

	return &indexfile, nil
}
