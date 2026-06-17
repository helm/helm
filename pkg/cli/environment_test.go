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

package cli

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/version"
)

func TestSetNamespace(t *testing.T) {
	settings := New()

	assert.Empty(t, settings.namespace)

	settings.SetNamespace("testns")
	assert.Equal(t, "testns", settings.namespace)
}

func TestEnvSettings(t *testing.T) {
	tests := []struct {
		name string

		// input
		args    string
		envvars map[string]string

		// expected values
		ns, kcontext  string
		debug         bool
		maxhistory    int
		kubeAsUser    string
		kubeAsGroups  []string
		kubeCaFile    string
		kubeInsecure  bool
		kubeTLSServer string
		burstLimit    int
		qps           float32
	}{
		{
			name:       "defaults",
			ns:         "default",
			maxhistory: defaultMaxHistory,
			burstLimit: defaultBurstLimit,
			qps:        defaultQPS,
		},
		{
			name:          "with flags set",
			args:          "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/tmp/ca.crt --burst-limit 100  --qps 50.12 --kube-insecure-skip-tls-verify=true --kube-tls-server-name=example.org",
			ns:            "myns",
			debug:         true,
			maxhistory:    defaultMaxHistory,
			burstLimit:    100,
			qps:           50.12,
			kubeAsUser:    "poro",
			kubeAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kubeCaFile:    "/tmp/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
		{
			name:          "with envvars set",
			envvars:       map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt", "HELM_BURST_LIMIT": "150", "HELM_KUBEINSECURE_SKIP_TLS_VERIFY": "true", "HELM_KUBETLS_SERVER_NAME": "example.org", "HELM_QPS": "60.34"},
			ns:            "yourns",
			maxhistory:    5,
			burstLimit:    150,
			qps:           60.34,
			debug:         true,
			kubeAsUser:    "pikachu",
			kubeAsGroups:  []string{"operators", "snackeaters", "partyanimals"},
			kubeCaFile:    "/tmp/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
		{
			name:          "with flags and envvars set",
			args:          "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/my/ca.crt --burst-limit 175 --qps 70 --kube-insecure-skip-tls-verify=true --kube-tls-server-name=example.org",
			envvars:       map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt", "HELM_BURST_LIMIT": "200", "HELM_KUBEINSECURE_SKIP_TLS_VERIFY": "true", "HELM_KUBETLS_SERVER_NAME": "example.org", "HELM_QPS": "40"},
			ns:            "myns",
			debug:         true,
			maxhistory:    5,
			burstLimit:    175,
			qps:           70,
			kubeAsUser:    "poro",
			kubeAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kubeCaFile:    "/my/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
		{
			name:       "invalid kubeconfig",
			ns:         "testns",
			args:       "--namespace=testns --kubeconfig=/path/to/fake/file",
			maxhistory: defaultMaxHistory,
			burstLimit: defaultBurstLimit,
			qps:        defaultQPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			for k, v := range tt.envvars {
				t.Setenv(k, v)
			}

			flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)

			settings := New()
			settings.AddFlags(flags)
			flags.Parse(strings.Split(tt.args, " "))

			assert.Equal(t, tt.debug, settings.Debug, "debug")
			assert.Equal(t, tt.ns, settings.Namespace(), "namespace")
			assert.Equal(t, tt.kcontext, settings.KubeContext, "kube-context")
			assert.Equal(t, tt.maxhistory, settings.MaxHistory, "maxHistory")
			assert.Equal(t, tt.kubeAsUser, settings.KubeAsUser, "kubeAsUser")
			assert.True(t, reflect.DeepEqual(tt.kubeAsGroups, settings.KubeAsGroups), "kubeAsGroups")
			assert.Equal(t, tt.kubeCaFile, settings.KubeCaFile, "kubeCaFile")
			assert.Equal(t, tt.burstLimit, settings.BurstLimit, "burstLimit")
			assert.Equal(t, tt.kubeInsecure, settings.KubeInsecureSkipTLSVerify, "kubeInsecure")
			assert.Equal(t, tt.kubeTLSServer, settings.KubeTLSServerName, "kubeTLSServer")
		})
	}
}

func TestEnvOrBool(t *testing.T) {
	const envName = "TEST_ENV_OR_BOOL"
	tests := []struct {
		name     string
		env      string
		val      string
		def      bool
		expected bool
	}{
		{
			name:     "unset with default false",
			def:      false,
			expected: false,
		},
		{
			name:     "unset with default true",
			def:      true,
			expected: true,
		},
		{
			name:     "blank env with default false",
			env:      envName,
			def:      false,
			expected: false,
		},
		{
			name:     "blank env with default true",
			env:      envName,
			def:      true,
			expected: true,
		},
		{
			name:     "env true with default false",
			env:      envName,
			val:      "true",
			def:      false,
			expected: true,
		},
		{
			name:     "env false with default true",
			env:      envName,
			val:      "false",
			def:      true,
			expected: false,
		},
		{
			name:     "env fails parsing with default true",
			env:      envName,
			val:      "NOT_A_BOOL",
			def:      true,
			expected: true,
		},
		{
			name:     "env fails parsing with default false",
			env:      envName,
			val:      "NOT_A_BOOL",
			def:      false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(tt.env, tt.val)
			}
			actual := envBoolOr(tt.env, tt.def)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUserAgentHeaderInK8sRESTClientConfig(t *testing.T) {
	defer resetEnv()()

	settings := New()
	restConfig, err := settings.RESTClientGetter().ToRESTConfig()
	require.NoError(t, err)

	expectedUserAgent := version.GetUserAgent()
	assert.Equal(t, expectedUserAgent, restConfig.UserAgent)
}

func resetEnv() func() {
	origEnv := os.Environ()

	// ensure any local envvars do not hose us
	for e := range New().EnvVars() {
		os.Unsetenv(e)
	}

	return func() {
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}
