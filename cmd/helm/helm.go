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

package main // import "helm.sh/helm/cmd/helm"

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/gates"
	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"
)

// FeatureGateOCI is the feature gate for checking if `helm chart` and `helm registry` commands should work
const FeatureGateOCI = gates.Gate("HELM_EXPERIMENTAL_OCI")

var settings = cli.New()

func init() {
	log.SetFlags(log.Lshortfile)
}

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func initKubeLogs() {
	pflag.CommandLine.SetNormalizeFunc(wordSepNormalizeFunc)
	gofs := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(gofs)
	pflag.CommandLine.AddGoFlagSet(gofs)
	pflag.CommandLine.Set("logtostderr", "true")
}

func main() {
	initKubeLogs()

	actionConfig := new(action.Configuration)
	cmd := newRootCmd(actionConfig, os.Stdout, os.Args[1:])

	if err := cmd.Execute(); err != nil {
		debug("%+v", err)
		switch e := err.(type) {
		case pluginError:
			os.Exit(e.code)
		default:
			os.Exit(1)
		}
	}
}

func initActionConfig(ac *action.Configuration, allNamespaces bool) error {
	if ac.Log == nil {
		ac.Log = debug
	}
	if ac.RESTClientGetter == nil {
		ac.RESTClientGetter = settings.KubeConfig
	}

	if ac.KubeClient == nil {
		kc := kube.New(ac.RESTClientGetter)
		kc.Log = debug
		ac.KubeClient = kc
	}

	if ac.Releases == nil {
		kc := kube.New(ac.RESTClientGetter)
		kc.Log = debug
		ac.KubeClient = kc
		clientset, err := kc.Factory.KubernetesClientSet()
		if err != nil {
			return err
		}
		var namespace string
		if !allNamespaces {
			namespace = settings.Namespace()
		}

		var store *storage.Storage
		switch os.Getenv("HELM_DRIVER") {
		case "secret", "secrets", "":
			d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
			d.Log = debug
			store = storage.Init(d)
		case "configmap", "configmaps":
			d := driver.NewConfigMaps(clientset.CoreV1().ConfigMaps(namespace))
			d.Log = debug
			store = storage.Init(d)
		case "memory":
			d := driver.NewMemory()
			store = storage.Init(d)
		default:
			return errors.New("Unknown driver in HELM_DRIVER: " + os.Getenv("HELM_DRIVER"))
		}
		ac.Releases = store
	}
	return nil
}

// wordSepNormalizeFunc changes all flags that contain "_" separators
func wordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
}

func checkOCIFeatureGate() func(_ *cobra.Command, _ []string) error {
	return func(_ *cobra.Command, _ []string) error {
		if !FeatureGateOCI.IsEnabled() {
			return FeatureGateOCI.Error()
		}
		return nil
	}
}
