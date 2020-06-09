package http

import (
	"fmt"
	"io"
	"os"
	"sync"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type repoUpdateOptions struct {
	update   func([]*repo.ChartRepository, io.Writer)
	repoFile string
}

func HelmRepoUpdate() error {
	o := &repoUpdateOptions{update: updateCharts}
	o.repoFile = settings.RepositoryConfig
	f, err := repo.LoadFile(o.repoFile)
	if err != nil {
		return err
	}
	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		repos = append(repos, r)
	}

	o.update(repos, os.Stdout)
	return nil
}

func updateCharts(repos []*repo.ChartRepository, out io.Writer) {
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
			} else {
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()
	fmt.Fprintln(out, "Update Complete. ⎈ Happy Helming!⎈ ")
}
