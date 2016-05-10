package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kubernetes/helm/pkg/chart"
	"gopkg.in/yaml.v2"
)

var localRepoPath string

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

// StartLocalRepo starts a web server and serves files from the given path
func StartLocalRepo(path string) {
	fmt.Println("Now serving you on localhost:8879...")
	localRepoPath = path
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/charts/", indexHandler)
	http.ListenAndServe(":8879", nil)
}
func rootHandler(w http.ResponseWriter, r *http.Request) {
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
	name, err := chart.Save(ch, path)
	if err != nil {
		return err
	}
	err = Reindex(ch, path+"/index.yaml")
	if err != nil {
		return err
	}
	fmt.Printf("Saved %s to $HELM_HOME/local", name)
	return nil
}

// LoadIndexFile takes a file at the given path and returns an IndexFile object
func LoadIndexFile(path string) (*IndexFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	//TODO: change variable name - y is not helpful :P
	var y IndexFile
	err = yaml.Unmarshal(b, &y)
	if err != nil {
		return nil, err
	}
	return &y, nil
}

// Reindex adds an entry to the index file at the given path
func Reindex(ch *chart.Chart, path string) error {
	name := ch.Chartfile().Name + "-" + ch.Chartfile().Version
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
		url := "localhost:8879/charts/" + name + ".tgz"

		out, err := y.insertChartEntry(name, url)
		if err != nil {
			return err
		}

		ioutil.WriteFile(path, out, 0644)
	}
	return nil
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

func (i *IndexFile) insertChartEntry(name string, url string) ([]byte, error) {
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
