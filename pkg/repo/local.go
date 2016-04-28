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

// CacheFile represents the cache file in a chart repository
type CacheFile struct {
	Entries map[string]*ChartRef
}

// ChartRef represents a chart entry in the CacheFile
type ChartRef struct {
	Name string
	URL  string
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
	} else if file == "cache" {
		fmt.Fprintf(w, "cache file data should be here at some point")
	} else {
		fmt.Fprintf(w, "Ummm... Nothing to see here folks")
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, file string) {
	http.ServeFile(w, r, filepath.Join(localRepoPath, file))
}

// AddChartToLocalRepo saves a chart in the given path and then reindexes the cache file
func AddChartToLocalRepo(ch *chart.Chart, path string) error {
	name, err := chart.Save(ch, path)
	if err != nil {
		return err
	}
	err = ReindexCacheFile(ch, path+"/cache.yaml")
	if err != nil {
		return err
	}
	fmt.Printf("Saved %s to $HELM_HOME/local", name)
	return nil
}

// LoadCacheFile takes a file at the given path and returns a CacheFile object
func LoadCacheFile(path string) (*CacheFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	//TODO: change variable name - y is not helpful :P
	var y CacheFile
	err = yaml.Unmarshal(b, &y)
	if err != nil {
		return nil, err
	}
	return &y, nil
}

// ReindexCacheFile adds an entry to the cache file at the given path
func ReindexCacheFile(ch *chart.Chart, path string) error {
	name := ch.Chartfile().Name + "-" + ch.Chartfile().Version
	y, err := LoadCacheFile(path)
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

// UnmarshalYAML unmarshals the cache file
func (c *CacheFile) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var refs map[string]*ChartRef
	if err := unmarshal(&refs); err != nil {
		if _, ok := err.(*yaml.TypeError); !ok {
			return err
		}
	}
	c.Entries = refs
	return nil
}

func (c *CacheFile) insertChartEntry(name string, url string) ([]byte, error) {
	if c.Entries == nil {
		c.Entries = make(map[string]*ChartRef)
	}
	entry := ChartRef{Name: name, URL: url}
	c.Entries[name] = &entry
	out, err := yaml.Marshal(&c.Entries)
	if err != nil {
		return nil, err
	}

	return out, nil
}
