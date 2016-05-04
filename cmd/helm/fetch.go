package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kubernetes/helm/pkg/repo"
	"github.com/spf13/cobra"
)

func init() {
	RootCommand.AddCommand(fetchCmd)
}

var fetchCmd = &cobra.Command{
	Use:   "fetch [chart URL | repo/chartname]",
	Short: "Download a chart from a repository and (optionally) unpack it in local directory.",
	Long:  "",
	RunE:  fetch,
}

func fetch(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("This command needs at least one argument, url or repo/name of the chart.")
	}

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	// get download url
	u, err := mapRepoArg(args[0], f.Repositories)
	if err != nil {
		return err
	}

	// Grab the package name that we'll use for the name of the file to download to.
	p := strings.Split(u.String(), "/")
	chartName := p[len(p)-1]
	out, err := os.Create(chartName)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get(u)
	// unpack file
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// mapRepoArg figures out which format the argument is given, and creates a fetchable
// url from it.
func mapRepoArg(arg string, r map[string]string) (*url.URL, error) {
	// See if it's already a full URL.
	u, err := url.ParseRequestURI(arg)
	if err == nil {
		// If it has a scheme and host and path, it's a full URL
		if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
			return u, nil
		} else {
			return nil, fmt.Errorf("Invalid chart url format: %s", arg)
		}
	}
	// See if it's of the form: repo/path_to_chart
	p := strings.Split(arg, "/")
	if len(p) > 1 {
		if baseUrl, ok := r[p[0]]; ok {
			if !strings.HasSuffix(baseUrl, "/") {
				baseUrl = baseUrl + "/"
			}
			return url.ParseRequestURI(baseUrl + strings.Join(p[1:], "/"))
		} else {
			return nil, fmt.Errorf("No such repo: %s", p[0])
		}
	} else {
		return nil, fmt.Errorf("Invalid chart url format: %s", arg)
	}
}
