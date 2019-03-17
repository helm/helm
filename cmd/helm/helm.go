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
	"fmt"
	"log"
	"os"
	"sync"

	// Import to initialize client auth plugins.
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"
)

var (
	settings   cli.EnvSettings
	config     genericclioptions.RESTClientGetter
	configOnce sync.Once
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func logf(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func main() {
	cmd := newRootCmd(newActionConfig(false), os.Stdout, os.Args[1:])
	if err := cmd.Execute(); err != nil {
		logf("%+v", err)
		os.Exit(1)
	}
}

func newActionConfig(allNamespaces bool) *action.Configuration {
	kc := kube.New(kubeConfig())
	kc.Log = logf

	clientset, err := kc.KubernetesClientSet()
	if err != nil {
		// TODO return error
		log.Fatal(err)
	}
	var namespace string
	if !allNamespaces {
		namespace = getNamespace()
	}

	var store *storage.Storage
	switch os.Getenv("HELM_DRIVER") {
	case "secret", "secrets", "":
		d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
		d.Log = logf
		store = storage.Init(d)
	case "configmap", "configmaps":
		d := driver.NewConfigMaps(clientset.CoreV1().ConfigMaps(namespace))
		d.Log = logf
		store = storage.Init(d)
	case "memory":
		d := driver.NewMemory()
		store = storage.Init(d)
	default:
		// Not sure what to do here.
		panic("Unknown driver in HELM_DRIVER: " + os.Getenv("HELM_DRIVER"))
	}

	return &action.Configuration{
		KubeClient: kc,
		Releases:   store,
		Discovery:  clientset.Discovery(),
		Log:        logf,
	}
}

func kubeConfig() genericclioptions.RESTClientGetter {
	configOnce.Do(func() {
		config = kube.GetConfig(settings.KubeConfig, settings.KubeContext, settings.Namespace)
	})
	return config
}

func getNamespace() string {
	if ns, _, err := kubeConfig().ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}
