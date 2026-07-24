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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestDefaultVersionSetMatchesScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1beta1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	groups := scheme.PrioritizedVersionsAllGroups()
	schemeVersions := make(VersionSet, 0, len(groups))
	for _, group := range groups {
		schemeVersions = append(schemeVersions, group.String())
	}

	require.ElementsMatch(t, DefaultVersionSet, schemeVersions, "DefaultVersionSet must stay in sync with the Kubernetes scheme")
}
