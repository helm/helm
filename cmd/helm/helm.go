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

package main // import "helm.sh/helm/v4/cmd/helm"

import (
	"log"
	"os"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	logadapter "helm.sh/helm/v4/internal/log"
	helmcmd "helm.sh/helm/v4/pkg/cmd"
	"helm.sh/helm/v4/pkg/kube"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	// Setting the name of the app for managedFields in the Kubernetes client.
	// It is set here to the full name of "helm" so that renaming of helm to
	// another name (e.g., helm2 or helm3) does not change the name of the
	// manager as picked up by the automated name detection.
	kube.ManagedFieldsManager = "helm"
	logger := logadapter.NewReadableTextLogger(os.Stderr, false)

	cmd, err := helmcmd.NewRootCmd(os.Stdout, os.Args[1:])
	if err != nil {
		logger.Warn("%+v", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		logger.Debug("error", err)
		switch e := err.(type) {
		case helmcmd.PluginError:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}
