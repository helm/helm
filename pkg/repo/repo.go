package repo

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubernetes/helm/pkg/chart"
	"gopkg.in/yaml.v2"
)

var indexPath = "index.yaml"

// ChartRepository represents a chart repository
type ChartRepository struct {
	RootPath   string
	URL        string // URL of repository
	ChartPaths []string
	IndexFile  *IndexFile
}

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
	Chartfile chart.Chartfile `yaml:"chartfile"`
}

// RepoFile represents the repositories.yaml file in $HELM_HOME
type RepoFile struct {
	Repositories map[string]string
}

// LoadRepositoriesFile takes a file at the given path and returns a RepoFile object
func LoadRepositoriesFile(path string) (*RepoFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var r RepoFile
	err = yaml.Unmarshal(b, &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// UnmarshalYAML unmarshals the repo file
func (rf *RepoFile) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var repos map[string]string
	if err := unmarshal(&repos); err != nil {
		if _, ok := err.(*yaml.TypeError); !ok {
			return err
		}
	}
	rf.Repositories = repos
	return nil
}

// LoadChartRepository takes in a path to a local chart repository
//      which contains packaged charts and an index.yaml file
//
// This function evaluates the contents of the directory and
// returns a ChartRepository
func LoadChartRepository(dir string) (*ChartRepository, error) {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !dirInfo.IsDir() {
		return nil, errors.New(dir + "is not a directory")
	}

	r := &ChartRepository{RootPath: dir}

	filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if strings.Contains(f.Name(), "index.yaml") {
				i, err := LoadIndexFile(path)
				if err != nil {
					return nil
				}
				r.IndexFile = i
			} else {
				// TODO: check for tgz extension
				r.ChartPaths = append(r.ChartPaths, path)
			}
		}
		return nil
	})

	return r, nil
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

func (r *ChartRepository) Index() error {
	if r.IndexFile == nil {
		r.IndexFile = &IndexFile{Entries: make(map[string]*ChartRef)}
	}

	for _, path := range r.ChartPaths {
		ch, err := chart.Load(path)
		if err != nil {
			return err
		}
		chartfile := ch.Chartfile()

		key := chartfile.Name + "-" + chartfile.Version
		if r.IndexFile.Entries == nil {
			r.IndexFile.Entries = make(map[string]*ChartRef)
		}

		entry := &ChartRef{Chartfile: *chartfile, Name: chartfile.Name, URL: "", Created: "", Removed: false}

		//TODO: generate hash of contents of chart and add to the entry
		//TODO: Set created timestamp

		r.IndexFile.Entries[key] = entry

	}

	if err := r.saveIndexFile(); err != nil {
		return err
	}

	return nil
}

func (r *ChartRepository) saveIndexFile() error {
	index, err := yaml.Marshal(&r.IndexFile.Entries)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(filepath.Join(r.RootPath, indexPath), index, 0644); err != nil {
		return err
	}

	return nil
}
