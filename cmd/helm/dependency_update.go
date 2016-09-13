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

package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/resolver"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/repo"
)

const dependencyUpDesc = `
Update the on-disk dependencies to mirror the requirements.yaml file.

This command verifies that the required charts, as expressed in 'requirements.yaml',
are present in 'charts/' and are at an acceptable version.
`

// dependencyUpdateCmd describes a 'helm dependency update'
type dependencyUpdateCmd struct {
	out       io.Writer
	chartpath string
	repoFile  string
	repopath  string
	helmhome  string
	verify    bool
	keyring   string
}

// newDependencyUpdateCmd creates a new dependency update command.
func newDependencyUpdateCmd(out io.Writer) *cobra.Command {
	duc := &dependencyUpdateCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:     "update [flags] CHART",
		Aliases: []string{"up"},
		Short:   "update charts/ based on the contents of requirements.yaml",
		Long:    dependencyUpDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			cp := "."
			if len(args) > 0 {
				cp = args[0]
			}

			var err error
			duc.chartpath, err = filepath.Abs(cp)
			if err != nil {
				return err
			}

			duc.helmhome = homePath()
			duc.repoFile = repositoriesFile()
			duc.repopath = repositoryDirectory()

			return duc.run()
		},
	}

	f := cmd.Flags()
	f.BoolVar(&duc.verify, "verify", false, "Verify the package against its signature.")
	f.StringVar(&duc.keyring, "keyring", defaultKeyring(), "The keyring containing public keys.")

	return cmd
}

// run runs the full dependency update process.
func (d *dependencyUpdateCmd) run() error {
	if fi, err := os.Stat(d.chartpath); err != nil {
		return fmt.Errorf("could not find %s: %s", d.chartpath, err)
	} else if !fi.IsDir() {
		return errors.New("only unpacked charts can be updated")
	}
	c, err := chartutil.LoadDir(d.chartpath)
	if err != nil {
		return err
	}

	req, err := chartutil.LoadRequirements(c)
	if err != nil {
		if err == chartutil.ErrRequirementsNotFound {
			fmt.Fprintf(d.out, "No requirements found in %s/charts.\n", d.chartpath)
			return nil
		}
		return err
	}

	// For each repo in the file, update the cached copy of that repo
	if _, err := d.updateRepos(req.Dependencies); err != nil {
		return err
	}

	// Now we need to find out which version of a chart best satisfies the
	// requirements the requirements.yaml
	lock, err := d.resolve(req)
	if err != nil {
		return err
	}

	// Now we need to fetch every package here into charts/
	if err := d.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	// Finally, we need to write the lockfile.
	return writeLock(d.chartpath, lock)
}

// resolve takes a list of requirements and translates them into an exact version to download.
//
// This returns a lock file, which has all of the requirements normalized to a specific version.
func (d *dependencyUpdateCmd) resolve(req *chartutil.Requirements) (*chartutil.RequirementsLock, error) {
	res := resolver.New(d.chartpath, d.helmhome)
	return res.Resolve(req)
}

// downloadAll takes a list of dependencies and downloads them into charts/
func (d *dependencyUpdateCmd) downloadAll(deps []*chartutil.Dependency) error {
	repos, err := loadChartRepositories(d.repopath)
	if err != nil {
		return err
	}

	fmt.Fprintf(d.out, "Saving %d charts\n", len(deps))
	for _, dep := range deps {
		fmt.Fprintf(d.out, "Downloading %s from repo %s\n", dep.Name, dep.Repository)

		target := fmt.Sprintf("%s-%s", dep.Name, dep.Version)
		churl, err := findChartURL(target, dep.Repository, repos)
		if err != nil {
			fmt.Fprintf(d.out, "WARNING: %s (skipped)", err)
			continue
		}

		dest := filepath.Join(d.chartpath, "charts", target+".tgz")
		data, err := downloadChart(churl, d.verify, d.keyring)
		if err != nil {
			fmt.Fprintf(d.out, "WARNING: Could not download %s: %s (skipped)", churl, err)
			continue
		}
		if err := ioutil.WriteFile(dest, data.Bytes(), 0655); err != nil {
			fmt.Fprintf(d.out, "WARNING: %s (skipped)", err)
			continue
		}
	}
	return nil
}

// updateRepos updates all of the local repos to their latest.
//
// If one of the dependencies present is not in the cached repos, this will error out. The
// consequence of that is that every repository referenced in a requirements.yaml file
// must also be added with 'helm repo add'.
func (d *dependencyUpdateCmd) updateRepos(deps []*chartutil.Dependency) (*repo.RepoFile, error) {
	// TODO: In the future, we could make it so that only the repositories that
	// are used by this chart are updated. As it is, we're mainly doing some sanity
	// checking here.
	rf, err := repo.LoadRepositoriesFile(d.repoFile)
	if err != nil {
		return rf, err
	}
	repos := rf.Repositories

	// Verify that all repositories referenced in the deps are actually known
	// by Helm.
	missing := []string{}
	for _, dd := range deps {
		found := false
		if dd.Repository == "" {
			found = true
		} else {
			for _, repo := range repos {
				if urlsAreEqual(repo, dd.Repository) {
					found = true
				}
			}
		}
		if !found {
			missing = append(missing, dd.Repository)
		}
	}

	if len(missing) > 0 {
		return rf, fmt.Errorf("no repository definition for %s. Try 'helm repo add'", strings.Join(missing, ", "))
	}

	if len(repos) > 0 {
		// This prints errors straight to out.
		updateCharts(repos, flagDebug, d.out)
	}
	return rf, nil
}

// urlsAreEqual normalizes two URLs and then compares for equality.
func urlsAreEqual(a, b string) bool {
	au, err := url.Parse(a)
	if err != nil {
		return a == b
	}
	bu, err := url.Parse(b)
	if err != nil {
		return false
	}
	return au.String() == bu.String()
}

// findChartURL searches the cache of repo data for a chart that has the name and the repourl specified.
//
// In this current version, name is of the form 'foo-1.2.3'. This will change when
// the repository index stucture changes.
func findChartURL(name, repourl string, repos map[string]*repo.ChartRepository) (string, error) {
	for _, cr := range repos {
		if urlsAreEqual(repourl, cr.URL) {
			for ename, entry := range cr.IndexFile.Entries {
				if ename == name {
					return entry.URL, nil
				}
			}
		}
	}
	return "", fmt.Errorf("chart %s not found in %s", name, repourl)
}

// loadChartRepositories reads the repositories.yaml, and then builds a map of
// ChartRepositories.
//
// The key is the local name (which is only present in the repositories.yaml).
func loadChartRepositories(repodir string) (map[string]*repo.ChartRepository, error) {
	indices := map[string]*repo.ChartRepository{}
	repoyaml := repositoriesFile()

	// Load repositories.yaml file
	rf, err := repo.LoadRepositoriesFile(repoyaml)
	if err != nil {
		return indices, fmt.Errorf("failed to load %s: %s", repoyaml, err)
	}

	// localName: chartRepo
	for lname, url := range rf.Repositories {
		index, err := repo.LoadIndexFile(cacheIndexFile(lname))
		if err != nil {
			return indices, err
		}

		cr := &repo.ChartRepository{
			URL:       url,
			IndexFile: index,
		}
		indices[lname] = cr
	}
	return indices, nil
}

// writeLock writes a lockfile to disk
func writeLock(chartpath string, lock *chartutil.RequirementsLock) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	dest := filepath.Join(chartpath, "requirements.lock")
	return ioutil.WriteFile(dest, data, 0755)
}
