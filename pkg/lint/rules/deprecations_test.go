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

package rules // import "helm.sh/helm/v3/pkg/lint/rules"

import "testing"

func TestValidateNoDeprecations(t *testing.T) {
	deprecated := &K8sYamlStruct{
		APIVersion: "extensions/v1beta1",
		Kind:       "Deployment",
	}
	err := validateNoDeprecations(deprecated)
	if err == nil {
		t.Fatal("Expected deprecated extension to be flagged")
	}

	depErr := err.(deprecatedAPIError)
	if depErr.Alternative != "apps/v1 Deployment" {
		t.Errorf("Expected %q to be replaced by %q", depErr.Deprecated, depErr.Alternative)
	}

	if err := validateNoDeprecations(&K8sYamlStruct{
		APIVersion: "v1",
		Kind:       "Pod",
	}); err != nil {
		t.Errorf("Expected a v1 Pod to not be deprecated")
	}
}
