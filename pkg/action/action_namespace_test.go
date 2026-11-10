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

package action

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestConfigurationInit_ResourceNamespace(t *testing.T) {
	tests := []struct {
		name              string
		configNamespace   string
		contextNamespace  string
		manifestNamespace string
		clusterScoped     bool
		reinitialize      bool
		changeContext     bool
		wantNamespace     string
	}{
		{name: "explicit configuration namespace", configNamespace: "release-ns", contextNamespace: "context-ns", wantNamespace: "release-ns"},
		{name: "explicit manifest namespace", configNamespace: "release-ns", contextNamespace: "context-ns", manifestNamespace: "manifest-ns", wantNamespace: "manifest-ns"},
		{name: "kubeconfig fallback", contextNamespace: "context-ns", wantNamespace: "context-ns"},
		{name: "default fallback", wantNamespace: v1.NamespaceDefault},
		{name: "cluster-scoped resource", configNamespace: "release-ns", contextNamespace: "context-ns", clusterScoped: true},
		{name: "reinitialized configuration", configNamespace: "release-ns", contextNamespace: "context-ns", reinitialize: true, wantNamespace: "release-ns"},
		{name: "kubeconfig changed after initialization", configNamespace: "release-ns", contextNamespace: "context-ns", changeContext: true, wantNamespace: "release-ns"},
	}
	for _, tt := range tests {
		for _, asTable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/table=%t", tt.name, asTable), func(t *testing.T) {
				context := &clientcmdapi.Context{Cluster: "cluster", Namespace: tt.contextNamespace}
				kubeconfig := clientcmdapi.Config{
					CurrentContext: "current",
					Contexts:       map[string]*clientcmdapi.Context{"current": context},
					Clusters:       map[string]*clientcmdapi.Cluster{"cluster": {Server: "https://kubernetes.example"}},
				}
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{v1.SchemeGroupVersion})
				mapper.Add(v1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
				mapper.Add(v1.SchemeGroupVersion.WithKind("Namespace"), meta.RESTScopeRoot)
				getter := genericclioptions.NewTestConfigFlags().
					WithClientConfig(clientcmd.NewNonInteractiveClientConfig(kubeconfig, "current", &clientcmd.ConfigOverrides{}, nil)).
					WithRESTMapper(mapper)

				cfg := NewConfiguration()
				if tt.reinitialize {
					require.NoError(t, cfg.Init(getter, "previous-ns", "memory"))
				}
				require.NoError(t, cfg.Init(getter, tt.configNamespace, "memory"))
				if tt.changeContext {
					context.Namespace = "changed-context-ns"
				}

				kind := "ConfigMap"
				if tt.clusterScoped {
					kind = "Namespace"
				}
				manifest := fmt.Sprintf("apiVersion: v1\nkind: %s\nmetadata:\n  name: example\n", kind)
				if tt.manifestNamespace != "" {
					manifest += "  namespace: " + tt.manifestNamespace + "\n"
				}
				build := cfg.KubeClient.Build
				if asTable {
					build = cfg.KubeClient.BuildTable
				}
				resources, err := build(strings.NewReader(manifest), false)
				require.NoError(t, err)
				require.Len(t, resources, 1)
				assert.Equal(t, tt.wantNamespace, resources[0].Namespace)
				assert.Equal(t, tt.wantNamespace, resources[0].Object.(*unstructured.Unstructured).GetNamespace())
			})
		}
	}
}
