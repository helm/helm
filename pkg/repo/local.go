package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/deis/tiller/pkg/chart"
	"gopkg.in/yaml.v2"
)

var localRepoPath string

type CacheFile struct {
	Entries map[string]*ChartRef
}

type ChartRef struct {
	Name string `yaml:name`
	Url  string `yaml:url`
}

func StartLocalRepo(path string) {
	fmt.Println("Now serving you on localhost:8879...")
	localRepoPath = path
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/charts/", indexHandler)
	http.ListenAndServe(":8879", nil)
}
func homeHandler(w http.ResponseWriter, r *http.Request) {
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

func AddChartToLocalRepo(ch *chart.Chart, path string) error {
	name, err := chart.Save(ch, path)
	if err != nil {
		return err
	}
	err = ReindexCacheFile(ch, path+"/cache.yaml")
	if err != nil {
		return nil
	}
	fmt.Printf("Saved %s to $HELM_HOME/local", name)
	return nil
}

func LoadCacheFile(path string) (*CacheFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("read file err")
		fmt.Printf("err, %s", err)
		return nil, err
	}

	var y CacheFile
	err = yaml.Unmarshal(b, &y)
	if err != nil {
		fmt.Println("error unmarshaling")
		fmt.Println("err, %s", err)
		return nil, err
	}
	return &y, nil
}

func ReindexCacheFile(ch *chart.Chart, path string) error {
	name := ch.Chartfile().Name + "-" + ch.Chartfile().Version
	y, _ := LoadCacheFile(path) //TODO: handle err later
	found := false
	for k, _ := range y.Entries {
		if k == name {
			found = true
			break
		}
	}
	if !found {
		url := "localhost:8879/charts/" + name + ".tgz"

		out, err := y.InsertChartEntry(name, url)
		if err != nil {
			return err
		}

		ioutil.WriteFile(path, out, 0644)
	}
	return nil
}
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

func (cache *CacheFile) InsertChartEntry(name string, url string) ([]byte, error) {
	if cache.Entries == nil {
		cache.Entries = make(map[string]*ChartRef)
	}
	entry := ChartRef{Name: name, Url: url}
	cache.Entries[name] = &entry
	out, err := yaml.Marshal(&cache.Entries)
	if err != nil {
		return nil, err
	}

	return out, nil
}
