package repo

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

var localRepoPath string

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
