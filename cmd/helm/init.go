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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/helm/cmd/helm/installer"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/repo"
)

const initDesc = `
This command installs Tiller (the Helm server-side component) onto your
Kubernetes Cluster and sets up local configuration in $HELM_HOME (default ~/.helm/).

As with the rest of the Helm commands, 'helm init' discovers Kubernetes clusters
by reading $KUBECONFIG (default '~/.kube/config') and using the default context.

To set up just a local environment, use '--client-only'. That will configure
$HELM_HOME, but not attempt to connect to a Kubernetes cluster and install the Tiller
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
	stableRepository         = "stable"
	localRepository          = "local"
	localRepositoryIndexFile = "index.yaml"
)

var (
	stableRepositoryURL = "https://kubernetes-charts.storage.googleapis.com"
	// This is the IPv4 loopback, not localhost, because we have to force IPv4
	// for Dockerized Helm: https://github.com/kubernetes/helm/issues/1410
	localRepositoryURL = "http://127.0.0.1:8879/charts"
)

type initCmd struct {
	image          string
	clientOnly     bool
	canary         bool
	upgrade        bool
	namespace      string
	dryRun         bool
	forceUpgrade   bool
	skipRefresh    bool
	out            io.Writer
	client         helm.Interface
	home           helmpath.Home
	opts           installer.Options
	kubeClient     kubernetes.Interface
	serviceAccount string
	maxHistory     int
	wait           bool
}

func newInitCmd(out io.Writer) *cobra.Command {
	i := &initCmd{out: out}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Helm on both client and server",
		Long:  initDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("This command does not accept arguments")
			}
			i.namespace = settings.TillerNamespace
			i.home = settings.Home
			i.client = ensureHelmClient(i.client)

			return i.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&i.image, "tiller-image", "i", "", "override Tiller image")
	f.BoolVar(&i.canary, "canary-image", false, "use the canary Tiller image")
	f.BoolVar(&i.upgrade, "upgrade", false, "upgrade if Tiller is already installed")
	f.BoolVar(&i.forceUpgrade, "force-upgrade", false, "force upgrade of Tiller to the current helm version")
	f.BoolVarP(&i.clientOnly, "client-only", "c", false, "if set does not install Tiller")
	f.BoolVar(&i.dryRun, "dry-run", false, "do not install local or remote")
	f.BoolVar(&i.skipRefresh, "skip-refresh", false, "do not refresh (download) the local repository cache")
	f.BoolVar(&i.wait, "wait", false, "block until Tiller is running and ready to receive requests")

	f.BoolVar(&tlsEnable, "tiller-tls", false, "install Tiller with TLS enabled")
	f.BoolVar(&tlsVerify, "tiller-tls-verify", false, "install Tiller with TLS enabled and to verify remote certificates")
	f.StringVar(&tlsKeyFile, "tiller-tls-key", "", "path to TLS key file to install with Tiller")
	f.StringVar(&tlsCertFile, "tiller-tls-cert", "", "path to TLS certificate file to install with Tiller")
	f.StringVar(&tlsCaCertFile, "tls-ca-cert", "", "path to CA root certificate")

	f.StringVar(&stableRepositoryURL, "stable-repo-url", stableRepositoryURL, "URL for stable repository")
	f.StringVar(&localRepositoryURL, "local-repo-url", localRepositoryURL, "URL for local repository")

	f.BoolVar(&i.opts.EnableHostNetwork, "net-host", false, "install Tiller with net=host")
	f.StringVar(&i.serviceAccount, "service-account", "", "name of service account")
	f.IntVar(&i.maxHistory, "history-max", 0, "limit the maximum number of revisions saved per release. Use 0 for no limit.")

	f.StringVar(&i.opts.NodeSelectors, "node-selectors", "", "labels to specify the node on which Tiller is installed (app=tiller,helm=rocks)")
	f.VarP(&i.opts.Output, "output", "o", "skip installation and output Tiller's manifest in specified format (json or yaml)")
	f.StringArrayVar(&i.opts.Values, "override", []string{}, "override values for the Tiller Deployment manifest (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	return cmd
}

// tlsOptions sanitizes the tls flags as well as checks for the existence of required
// tls files indicated by those flags, if any.
func (i *initCmd) tlsOptions() error {
	i.opts.EnableTLS = tlsEnable || tlsVerify
	i.opts.VerifyTLS = tlsVerify

	if i.opts.EnableTLS {
		missing := func(file string) bool {
			_, err := os.Stat(file)
			return os.IsNotExist(err)
		}
		if i.opts.TLSKeyFile = tlsKeyFile; i.opts.TLSKeyFile == "" || missing(i.opts.TLSKeyFile) {
			return errors.New("missing required TLS key file")
		}
		if i.opts.TLSCertFile = tlsCertFile; i.opts.TLSCertFile == "" || missing(i.opts.TLSCertFile) {
			return errors.New("missing required TLS certificate file")
		}
		if i.opts.VerifyTLS {
			if i.opts.TLSCaCertFile = tlsCaCertFile; i.opts.TLSCaCertFile == "" || missing(i.opts.TLSCaCertFile) {
				return errors.New("missing required TLS CA file")
			}
		}
	}
	return nil
}

// run initializes local config and installs Tiller to Kubernetes cluster.
func (i *initCmd) run() error {
	if err := i.tlsOptions(); err != nil {
		return err
	}
	i.opts.Namespace = i.namespace
	i.opts.UseCanary = i.canary
	i.opts.ImageSpec = i.image
	i.opts.ForceUpgrade = i.forceUpgrade
	i.opts.ServiceAccount = i.serviceAccount
	i.opts.MaxHistory = i.maxHistory

	writeYAMLManifest := func(apiVersion, kind, body string, first, last bool) error {
		w := i.out
		if !first {
			// YAML starting document boundary marker
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "apiVersion:", apiVersion); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "kind:", kind); err != nil {
			return err
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			return err
		}
		if !last {
			return nil
		}
		// YAML ending document boundary marker
		_, err := fmt.Fprintln(w, "...")
		return err
	}
	if len(i.opts.Output) > 0 {
		var body string
		var err error
		const tm = `{"apiVersion":"extensions/v1beta1","kind":"Deployment",`
		if body, err = installer.DeploymentManifest(&i.opts); err != nil {
			return err
		}
		switch i.opts.Output.String() {
		case "json":
			var out bytes.Buffer
			jsonb, err := yaml.ToJSON([]byte(body))
			if err != nil {
				return err
			}
			buf := bytes.NewBuffer(make([]byte, 0, len(tm)+len(jsonb)-1))
			buf.WriteString(tm)
			// Drop the opening object delimiter ('{').
			buf.Write(jsonb[1:])
			if err := json.Indent(&out, buf.Bytes(), "", "    "); err != nil {
				return err
			}
			if _, err = i.out.Write(out.Bytes()); err != nil {
				return err
			}

			return nil
		case "yaml":
			if err := writeYAMLManifest("extensions/v1beta1", "Deployment", body, true, false); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unknown output format: %q", i.opts.Output)
		}
	}
	if settings.Debug {

		var body string
		var err error

		// write Deployment manifest
		if body, err = installer.DeploymentManifest(&i.opts); err != nil {
			return err
		}
		if err := writeYAMLManifest("extensions/v1beta1", "Deployment", body, true, false); err != nil {
			return err
		}

		// write Service manifest
		if body, err = installer.ServiceManifest(i.namespace); err != nil {
			return err
		}
		if err := writeYAMLManifest("v1", "Service", body, false, !i.opts.EnableTLS); err != nil {
			return err
		}

		// write Secret manifest
		if i.opts.EnableTLS {
			if body, err = installer.SecretManifest(&i.opts); err != nil {
				return err
			}
			if err := writeYAMLManifest("v1", "Secret", body, false, true); err != nil {
				return err
			}
		}
	}

	if i.dryRun {
		return nil
	}

	if err := ensureDirectories(i.home, i.out); err != nil {
		return err
	}
	if err := ensureDefaultRepos(i.home, i.out, i.skipRefresh); err != nil {
		return err
	}
	if err := ensureRepoFileFormat(i.home.RepositoryFile(), i.out); err != nil {
		return err
	}
	fmt.Fprintf(i.out, "$HELM_HOME has been configured at %s.\n", settings.Home)

	if !i.clientOnly {
		if i.kubeClient == nil {
			_, c, err := getKubeClient(settings.KubeContext)
			if err != nil {
				return fmt.Errorf("could not get kubernetes client: %s", err)
			}
			i.kubeClient = c
		}
		if err := installer.Install(i.kubeClient, &i.opts); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("error installing: %s", err)
			}
			if i.upgrade {
				if err := installer.Upgrade(i.kubeClient, &i.opts); err != nil {
					return fmt.Errorf("error when upgrading: %s", err)
				}
				if err := i.ping(); err != nil {
					return err
				}
				fmt.Fprintln(i.out, "\nTiller (the Helm server-side component) has been upgraded to the current version.")
			} else {
				fmt.Fprintln(i.out, "Warning: Tiller is already installed in the cluster.\n"+
					"(Use --client-only to suppress this message, or --upgrade to upgrade Tiller to the current version.)")
			}
		} else {
			fmt.Fprintln(i.out, "\nTiller (the Helm server-side component) has been installed into your Kubernetes Cluster.\n\n"+
				"Please note: by default, Tiller is deployed with an insecure 'allow unauthenticated users' policy.\n"+
				"For more information on securing your installation see: https://docs.helm.sh/using_helm/#securing-your-helm-installation")
		}
		if err := i.ping(); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(i.out, "Not installing Tiller due to 'client-only' flag having been set")
	}

	fmt.Fprintln(i.out, "Happy Helming!")
	return nil
}

func (i *initCmd) ping() error {
	if i.wait {
		_, kubeClient, err := getKubeClient(settings.KubeContext)
		if err != nil {
			return err
		}
		if !watchTillerUntilReady(settings.TillerNamespace, kubeClient, settings.TillerConnectionTimeout) {
			return fmt.Errorf("tiller was not found. polling deadline exceeded")
		}

		// establish a connection to Tiller now that we've effectively guaranteed it's available
		if err := setupConnection(); err != nil {
			return err
		}
		i.client = newClient()
		if err := i.client.PingTiller(); err != nil {
			return fmt.Errorf("could not ping Tiller: %s", err)
		}
	}

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
		home.LocalRepository(),
		home.Plugins(),
		home.Starters(),
		home.Archive(),
	}
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

	return nil
}

func ensureDefaultRepos(home helmpath.Home, out io.Writer, skipRefresh bool) error {
	repoFile := home.RepositoryFile()
	if fi, err := os.Stat(repoFile); err != nil {
		fmt.Fprintf(out, "Creating %s \n", repoFile)
		f := repo.NewRepoFile()
		sr, err := initStableRepo(home.CacheIndex(stableRepository), out, skipRefresh)
		if err != nil {
			return err
		}
		lr, err := initLocalRepo(home.LocalRepository(localRepositoryIndexFile), home.CacheIndex("local"), out)
		if err != nil {
			return err
		}
		f.Add(sr)
		f.Add(lr)
		if err := f.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", repoFile)
	}
	return nil
}

func initStableRepo(cacheFile string, out io.Writer, skipRefresh bool) (*repo.Entry, error) {
	fmt.Fprintf(out, "Adding %s repo with URL: %s \n", stableRepository, stableRepositoryURL)
	c := repo.Entry{
		Name:  stableRepository,
		URL:   stableRepositoryURL,
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
		return nil, fmt.Errorf("Looks like %q is not a valid chart repository or cannot be reached: %s", stableRepositoryURL, err.Error())
	}

	return &c, nil
}

func initLocalRepo(indexFile, cacheFile string, out io.Writer) (*repo.Entry, error) {
	if fi, err := os.Stat(indexFile); err != nil {
		fmt.Fprintf(out, "Adding %s repo with URL: %s \n", localRepository, localRepositoryURL)
		i := repo.NewIndexFile()
		if err := i.WriteFile(indexFile, 0644); err != nil {
			return nil, err
		}

		//TODO: take this out and replace with helm update functionality
		createLink(indexFile, cacheFile)
	} else if fi.IsDir() {
		return nil, fmt.Errorf("%s must be a file, not a directory", indexFile)
	}

	return &repo.Entry{
		Name:  localRepository,
		URL:   localRepositoryURL,
		Cache: cacheFile,
	}, nil
}

func ensureRepoFileFormat(file string, out io.Writer) error {
	r, err := repo.LoadRepositoriesFile(file)
	if err == repo.ErrRepoOutOfDate {
		fmt.Fprintln(out, "Updating repository file format...")
		if err := r.WriteFile(file, 0644); err != nil {
			return err
		}
	}

	return nil
}

// watchTillerUntilReady waits for the tiller pod to become available. This is useful in situations where we
// want to wait before we call New().
//
// Returns true if it exists. If the timeout was reached and it could not find the pod, it returns false.
func watchTillerUntilReady(namespace string, client kubernetes.Interface, timeout int64) bool {
	deadlinePollingChan := time.NewTimer(time.Duration(timeout) * time.Second).C
	checkTillerPodTicker := time.NewTicker(500 * time.Millisecond)
	doneChan := make(chan bool)

	defer checkTillerPodTicker.Stop()

	go func() {
		for range checkTillerPodTicker.C {
			_, err := portforwarder.GetTillerPodName(client.CoreV1(), namespace)
			if err == nil {
				doneChan <- true
				break
			}
		}
	}()

	for {
		select {
		case <-deadlinePollingChan:
			return false
		case <-doneChan:
			return true
		}
	}
}
