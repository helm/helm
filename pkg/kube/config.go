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

package kube // import "k8s.io/helm/pkg/kube"

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
)

// GetConfig returns a Kubernetes client config for a given context.
func GetConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	flags.StringVar(&rules.ExplicitPath, "kubeconfig", "", "path to the kubeconfig file to use for CLI requests")

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	flags.StringVarP(&overrides.Context.Namespace, "namespace", "n", "", "if present, the namespace scope for this CLI request")
	flags.StringVar(&overrides.CurrentContext, "context", "", "the name of the kubeconfig context to use")

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}
