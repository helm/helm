/*
Copyright The Helm Authors.

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
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/plugin"
	"k8s.io/helm/pkg/plugin/installer"
	"k8s.io/helm/pkg/repo"
)

const initDesc = `
This command sets up local configuration in $XDG_CONFIG_HOME/helm
($HOME/.config/helm on Linux, %APPDATA%\helm on Windows and
$HOME/Library/Preferences on OSX).
NOTE: if there is old-style "$HOME/.helm" directory, it will be preffered.
`

const (
	stableRepository           = "stable"
	defaultStableRepositoryURL = "https://kubernetes-charts.storage.googleapis.com"
)

type initOptions struct {
	skipRefresh         bool   // --skip-refresh
	stableRepositoryURL string // --stable-repo-url
	pluginsFilename     string // --plugins

	home helmpath.Home
}

type pluginsFileEntry struct {
	URL     string `json:"url"`
	Version string `json:"version,omitempty"`
}

type pluginsFile struct {
	Plugins []*pluginsFileEntry `json:"plugins"`
}

func newInitCmd(out io.Writer) *cobra.Command {
	o := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Helm client",
		Long:  initDesc,
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.home = settings.Home
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.skipRefresh, "skip-refresh", false, "do not refresh (download) the local repository cache")
	f.StringVar(&o.stableRepositoryURL, "stable-repo-url", defaultStableRepositoryURL, "URL for stable repository")
	f.StringVar(&o.pluginsFilename, "plugins", "", "a YAML file specifying plugins to install")

	return cmd
}

// run initializes local config.
func (o *initOptions) run(out io.Writer) error {
	if err := ensureDirectories(o.home, out); err != nil {
		return err
	}
	if err := ensureDefaultRepos(o.home, out, o.skipRefresh, o.stableRepositoryURL); err != nil {
		return err
	}
	if err := ensureRepoFileFormat(o.home.RepositoryFile(), out); err != nil {
		return err
	}
	if o.pluginsFilename != "" {
		if err := ensurePluginsInstalled(o.pluginsFilename, out); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "$HELM_HOME has been configured at %s.\n", settings.Home)
	fmt.Fprintln(out, "Happy Helming!")
	return nil
}

// ensureDirectories checks to see if $HELM_HOME exists.
//
// If $HELM_HOME does not exist, this function will create it.
func ensureDirectories(home helmpath.Home, out io.Writer) error {
	configDirectories := []string{
		home.String(),
		home.Repository(),
		home.Cache(),
		home.Plugins(),
		home.Starters(),
		home.Archive(),
	}
	for _, p := range configDirectories {
		if fi, err := os.Stat(p); err != nil {
			fmt.Fprintf(out, "Creating %s \n", p)
			if err := os.MkdirAll(p, 0755); err != nil {
				return errors.Wrapf(err, "could not create %s", p)
			}
		} else if !fi.IsDir() {
			return errors.Errorf("%s must be a directory", p)
		}
	}

	return nil
}

func ensureDefaultRepos(home helmpath.Home, out io.Writer, skipRefresh bool, url string) error {
	repoFile := home.RepositoryFile()
	if fi, err := os.Stat(repoFile); err != nil {
		fmt.Fprintf(out, "Creating %s \n", repoFile)
		f := repo.NewFile()
		sr, err := initRepo(url, home.CacheIndex(stableRepository), out, skipRefresh, home)
		if err != nil {
			return err
		}
		f.Add(sr)
		if err := f.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	} else if fi.IsDir() {
		return errors.Errorf("%s must be a file, not a directory", repoFile)
	}
	return nil
}

func initRepo(url, cacheFile string, out io.Writer, skipRefresh bool, home helmpath.Home) (*repo.Entry, error) {
	fmt.Fprintf(out, "Adding %s repo with URL: %s \n", stableRepository, url)
	c := repo.Entry{
		Name:  stableRepository,
		URL:   url,
		Cache: cacheFile,
	}
	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return nil, err
	}

	if skipRefresh {
		return &c, nil
	}

	// In this case, the cacheFile is always absolute. So passing empty string
	// is safe.
	if err := r.DownloadIndexFile(""); err != nil {
		return nil, errors.Wrapf(err, "%s is not a valid chart repository or cannot be reached", url)
	}

	return &c, nil
}

func ensureRepoFileFormat(file string, out io.Writer) error {
	r, err := repo.LoadFile(file)
	if err == repo.ErrRepoOutOfDate {
		fmt.Fprintln(out, "Updating repository file format...")
		if err := r.WriteFile(file, 0644); err != nil {
			return err
		}
	}
	return nil
}

func ensurePluginsInstalled(pluginsFilename string, out io.Writer) error {
	bytes, err := ioutil.ReadFile(pluginsFilename)
	if err != nil {
		return err
	}

	pf := new(pluginsFile)
	if err := yaml.Unmarshal(bytes, &pf); err != nil {
		return errors.Wrapf(err, "failed to parse %s", pluginsFilename)
	}

	for _, requiredPlugin := range pf.Plugins {
		if err := ensurePluginInstalled(requiredPlugin, pluginsFilename, out); err != nil {
			return errors.Wrapf(err, "failed to install plugin from %s", requiredPlugin.URL)
		}
	}

	return nil
}

func ensurePluginInstalled(requiredPlugin *pluginsFileEntry, pluginsFilename string, out io.Writer) error {
	i, err := installer.NewForSource(requiredPlugin.URL, requiredPlugin.Version, settings.Home)
	if err != nil {
		return err
	}

	if _, pathErr := os.Stat(i.Path()); os.IsNotExist(pathErr) {
		if err := installer.Install(i); err != nil {
			return err
		}

		p, err := plugin.LoadDir(i.Path())
		if err != nil {
			return err
		}

		if err := runHook(p, plugin.Install); err != nil {
			return err
		}

		fmt.Fprintf(out, "Installed plugin: %s\n", p.Metadata.Name)
	} else if requiredPlugin.Version != "" {
		p, err := plugin.LoadDir(i.Path())
		if err != nil {
			return err
		}

		if p.Metadata.Version != "" {
			pluginVersion, err := semver.NewVersion(p.Metadata.Version)
			if err != nil {
				return err
			}

			constraint, err := semver.NewConstraint(requiredPlugin.Version)
			if err != nil {
				return err
			}

			if !constraint.Check(pluginVersion) {
				fmt.Fprintf(out, "WARNING: Installed plugin '%s' is at version %s, while %s specifies %s\n",
					p.Metadata.Name, p.Metadata.Version, pluginsFilename, requiredPlugin.Version)
			}
		}
	}

	return nil
}
