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
	"os"
	"testing"
)

func TestEnv(t *testing.T) {
	defer resetEnv()()
	envFixture := map[string]string{
		"HELM_BIN":                          "./bin/helm",
		"HELM_BURST_LIMIT":                  "100",
		"HELM_CACHE_HOME":                   "/home/user/.cache/helm",
		"HELM_CONFIG_HOME":                  "/home/user/.config/helm",
		"HELM_DATA_HOME":                    "/home/user/.local/share/helm",
		"HELM_DEBUG":                        "false",
		"HELM_KUBEAPISERVER":                "",
		"HELM_KUBEASGROUPS":                 "",
		"HELM_KUBEASUSER":                   "",
		"HELM_KUBECAFILE":                   "",
		"HELM_KUBECONTEXT":                  "",
		"HELM_KUBEINSECURE_SKIP_TLS_VERIFY": "false",
		"HELM_KUBETLS_SERVER_NAME":          "",
		"HELM_KUBETOKEN":                    "",
		"HELM_MAX_HISTORY":                  "10",
		"HELM_NAMESPACE":                    "default",
		"HELM_PLUGINS":                      "/home/user/.local/share/helm/plugins",
		"HELM_REGISTRY_CONFIG":              "/home/user/.config/helm/registry/config.json",
		"HELM_REPOSITORY_CACHE":             "/home/user/.cache/helm/repository",
		"HELM_REPOSITORY_CONFIG":            "/home/user/.config/helm/repositories.yaml",
	}

	for k, v := range envFixture {
		os.Setenv(k, v)
	}

	tests := []cmdTestCase{
		{
			name:   "completion for env",
			cmd:    "__complete env ''",
			golden: "output/env-comp.txt",
		},
		{
			name:   "completion for env output flag",
			cmd:    "__complete env --output ''",
			golden: "output/env-output-comp.txt",
		},
		{
			name:   "no args",
			cmd:    "env",
			golden: "output/env-no-args.txt",
		},
		{
			name:   "no args in json format",
			cmd:    "env --output json",
			golden: "output/env-no-args-json.txt",
		},
		{
			name:   "no args in yaml format",
			cmd:    "env --output yaml",
			golden: "output/env-no-args-yaml.txt",
		},
		{
			name:      "no args in invalid format",
			cmd:       "env --output table",
			golden:    "output/env-no-args-invalid-format.txt",
			wantError: true,
		},
		{
			name:   "with args",
			cmd:    "env HELM_BIN",
			golden: "output/env-with-args.txt",
		},
		{
			name:   "with args in json format",
			cmd:    "env HELM_BIN --output json",
			golden: "output/env-with-args-json.txt",
		},
		{
			name:   "with args in yaml format",
			cmd:    "env HELM_BIN --output yaml",
			golden: "output/env-with-args-yaml.txt",
		},
	}
	runTestCmd(t, tests)
}

func TestEnvFileCompletion(t *testing.T) {
	checkFileCompletion(t, "env", false)
	checkFileCompletion(t, "env HELM_BIN", false)
}
