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
	"os"

	"github.com/spf13/cobra"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	extensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/cmd/helm/installer"
	"k8s.io/helm/pkg/repo"
)

const initDesc = `
This command installs Tiller (the helm server side component) onto your
Kubernetes Cluster and sets up local configuration in $HELM_HOME (default ~/.helm/)

As with the rest of the Helm commands, 'helm init' discovers Kubernetes clusters
by reading $KUBECONFIG (default '~/.kube/config') and using the default context.

To set up just a local environment, use '--client-only'. That will configure
$HELM_HOME, but not attempt to connect to a remote cluster and install the Tiller
deployment.

When installing Tiller, 'helm init' will attempt to install the latest released
version. You can specify an alternative image with '--tiller-image'. For those
frequently working on the latest code, the flag '--canary-image' will install
the latest pre-release version of Tiller (e.g. the HEAD commit in the GitHub
repository on the master branch).

To dump a manifest containing the Tiller deployment YAML, combine the
'--dry-run' and '--debug' flags.
`

const (
	stableRepository    = "stable"
	localRepository     = "local"
	stableRepositoryURL = "https://kubernetes-charts.storage.googleapis.com/"
	// This is the IPv4 loopback, not localhost, because we have to force IPv4
	// for Dockerized Helm: https://github.com/kubernetes/helm/issues/1410
	localRepositoryURL = "http://127.0.0.1:8879/charts"
)

type initCmd struct {
	image      string
	clientOnly bool
	canary     bool
	namespace  string
	dryRun     bool
	out        io.Writer
	home       helmpath.Home
	kubeClient extensionsclient.DeploymentsGetter
}

func newInitCmd(out io.Writer) *cobra.Command {
	i := &initCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Helm on both client and server",
		Long:  initDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("This command does not accept arguments")
			}
			i.namespace = tillerNamespace
			i.home = helmpath.Home(homePath())
			return i.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&i.image, "tiller-image", "i", "", "override tiller image")
	f.BoolVar(&i.canary, "canary-image", false, "use the canary tiller image")
	f.BoolVarP(&i.clientOnly, "client-only", "c", false, "if set does not install tiller")
	f.BoolVar(&i.dryRun, "dry-run", false, "do not install local or remote")

	return cmd
}

// runInit initializes local config and installs tiller to Kubernetes Cluster
func (i *initCmd) run() error {

	if flagDebug {
		m, err := installer.DeploymentManifest(i.namespace, i.image, i.canary)
		if err != nil {
			return err
		}
		fmt.Fprintln(i.out, m)
	}
	if i.dryRun {
		return nil
	}

	if err := ensureHome(i.home, i.out); err != nil {
		return err
	}

	if !i.clientOnly {
		if i.kubeClient == nil {
			_, c, err := getKubeClient(kubeContext)
			if err != nil {
				return fmt.Errorf("could not get kubernetes client: %s", err)
			}
			i.kubeClient = c
		}
		if err := installer.Install(i.kubeClient, i.namespace, i.image, i.canary, flagDebug); err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return fmt.Errorf("error installing: %s", err)
			}
			fmt.Fprintln(i.out, "Warning: Tiller is already installed in the cluster. (Use --client-only to suppress this message.)")
		} else {
			fmt.Fprintln(i.out, "\nTiller (the helm server side component) has been installed into your Kubernetes Cluster.")
		}
	} else {
		fmt.Fprintln(i.out, "Not installing tiller due to 'client-only' flag having been set")
	}
	fmt.Fprintln(i.out, "Happy Helming!")
	return nil
}

// ensureHome checks to see if $HELM_HOME exists
//
// If $HELM_HOME does not exist, this function will create it.
func ensureHome(home helmpath.Home, out io.Writer) error {
	configDirectories := []string{home.String(), home.Repository(), home.Cache(), home.LocalRepository(), home.Plugins(), home.Starters()}
	for _, p := range configDirectories {
		if fi, err := os.Stat(p); err != nil {
			fmt.Fprintf(out, "Creating %s \n", p)
			if err := os.MkdirAll(p, 0755); err != nil {
				return fmt.Errorf("Could not create %s: %s", p, err)
			}
		} else if !fi.IsDir() {
			return fmt.Errorf("%s must be a directory", p)
		}
	}

	repoFile := home.RepositoryFile()
	if fi, err := os.Stat(repoFile); err != nil {
		fmt.Fprintf(out, "Creating %s \n", repoFile)
		r := repo.NewRepoFile()
		r.Add(&repo.Entry{
			Name:  stableRepository,
			URL:   stableRepositoryURL,
			Cache: "stable-index.yaml",
		}, &repo.Entry{
			Name:  localRepository,
			URL:   localRepositoryURL,
			Cache: "local-index.yaml",
		})
		if err := r.WriteFile(repoFile, 0644); err != nil {
			return err
		}
		cif := home.CacheIndex(stableRepository)
		if err := repo.DownloadIndexFile(stableRepository, stableRepositoryURL, cif); err != nil {
			fmt.Fprintf(out, "WARNING: Failed to download %s: %s (run 'helm repo update')\n", stableRepository, err)
		}
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", repoFile)
	}
	if r, err := repo.LoadRepositoriesFile(repoFile); err == repo.ErrRepoOutOfDate {
		fmt.Fprintln(out, "Updating repository file format...")
		if err := r.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	}

	localRepoIndexFile := home.LocalRepository(localRepoIndexFilePath)
	if fi, err := os.Stat(localRepoIndexFile); err != nil {
		fmt.Fprintf(out, "Creating %s \n", localRepoIndexFile)
		i := repo.NewIndexFile()
		if err := i.WriteFile(localRepoIndexFile, 0644); err != nil {
			return err
		}

		//TODO: take this out and replace with helm update functionality
		os.Symlink(localRepoIndexFile, home.CacheIndex("local"))
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", localRepoIndexFile)
	}

	fmt.Fprintf(out, "$HELM_HOME has been configured at %s.\n", helmHome)
	return nil
}
