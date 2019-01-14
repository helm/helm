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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/plugin"
	"helm.sh/helm/pkg/plugin/installer"
	"helm.sh/helm/pkg/repo"
)

const initDesc = `
This command sets up local configuration.

Helm stores configuration based on the XDG base directory specification, so

- cached files are stored in $XDG_CACHE_HOME/helm
- configuration is stored in $XDG_CONFIG_HOME/helm
- data is stored in $XDG_DATA_HOME/helm

By default, the default directories depend on the Operating System. The defaults are listed below:

+------------------+---------------------------+--------------------------------+-------------------------+
| Operating System | Cache Path                | Configuration Path             | Data Path               |
+------------------+---------------------------+--------------------------------+-------------------------+
| Linux            | $HOME/.cache/helm         | $HOME/.config/helm             | $HOME/.local/share/helm |
| macOS            | $HOME/Library/Caches/helm | $HOME/Library/Preferences/helm | $HOME/Library/helm      |
| Windows          | %TEMP%\helm               | %APPDATA%\helm                 | %APPDATA%\helm          |
+------------------+---------------------------+--------------------------------+-------------------------+
`

type initOptions struct {
	skipRefresh     bool   // --skip-refresh
	pluginsFilename string // --plugins
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
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.skipRefresh, "skip-refresh", false, "do not refresh (download) the local repository cache")
	f.StringVar(&o.pluginsFilename, "plugins", "", "a YAML file specifying plugins to install")

	return cmd
}

// run initializes local config.
func (o *initOptions) run(out io.Writer) error {
	if err := ensureDirectories(out); err != nil {
		return err
	}
	if err := ensureReposFile(out, o.skipRefresh); err != nil {
		return err
	}
	if err := ensureRepoFileFormat(helmpath.RepositoryFile(), out); err != nil {
		return err
	}
	if o.pluginsFilename != "" {
		if err := ensurePluginsInstalled(o.pluginsFilename, out); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "Helm is now configured to use the following directories:")
	fmt.Fprintf(out, "Cache: %s\n", helmpath.CachePath())
	fmt.Fprintf(out, "Configuration: %s\n", helmpath.ConfigPath())
	fmt.Fprintf(out, "Data: %s\n", helmpath.DataPath())
	fmt.Fprintln(out, "Happy Helming!")
	return nil
}

// ensureDirectories checks to see if the directories Helm uses exists.
//
// If they do not exist, this function will create it.
func ensureDirectories(out io.Writer) error {
	directories := []string{
		helmpath.CachePath(),
		helmpath.ConfigPath(),
		helmpath.DataPath(),
		helmpath.RepositoryCache(),
		helmpath.Plugins(),
		helmpath.PluginCache(),
		helmpath.Starters(),
		helmpath.Archive(),
	}
	for _, p := range directories {
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

func ensureReposFile(out io.Writer, skipRefresh bool) error {
	repoFile := helmpath.RepositoryFile()
	if fi, err := os.Stat(repoFile); err != nil {
		fmt.Fprintf(out, "Creating %s \n", repoFile)
		f := repo.NewFile()
		if err := f.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	} else if fi.IsDir() {
		return errors.Errorf("%s must be a file, not a directory", repoFile)
	}
	return nil
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
	i, err := installer.NewForSource(requiredPlugin.URL, requiredPlugin.Version)
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
