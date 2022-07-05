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
)

func TestSetNamespace(t *testing.T) {
	settings := New()

	if settings.namespace != "" {
		t.Errorf("Expected empty namespace, got %s", settings.namespace)
	}

	settings.SetNamespace("testns")
	if settings.namespace != "testns" {
		t.Errorf("Expected namespace testns, got %s", settings.namespace)
	}

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
	}{
		{
			name:       "defaults",
			ns:         "default",
			maxhistory: defaultMaxHistory,
			burstLimit: defaultBurstLimit,
		},
		{
			name:          "with flags set",
			args:          "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/tmp/ca.crt --burst-limit 100 --kube-insecure-skip-tls-verify=true --kube-tls-server-name=example.org",
			ns:            "myns",
			debug:         true,
			maxhistory:    defaultMaxHistory,
			burstLimit:    100,
			kubeAsUser:    "poro",
			kubeAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kubeCaFile:    "/tmp/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
		{
			name:          "with envvars set",
			envvars:       map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt", "HELM_BURST_LIMIT": "150", "HELM_KUBEINSECURE_SKIP_TLS_VERIFY": "true", "HELM_KUBETLS_SERVER_NAME": "example.org"},
			ns:            "yourns",
			maxhistory:    5,
			burstLimit:    150,
			debug:         true,
			kubeAsUser:    "pikachu",
			kubeAsGroups:  []string{"operators", "snackeaters", "partyanimals"},
			kubeCaFile:    "/tmp/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
		{
			name:          "with flags and envvars set",
			args:          "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/my/ca.crt --burst-limit 175 --kube-insecure-skip-tls-verify=true --kube-tls-server-name=example.org",
			envvars:       map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt", "HELM_BURST_LIMIT": "200", "HELM_KUBEINSECURE_SKIP_TLS_VERIFY": "true", "HELM_KUBETLS_SERVER_NAME": "example.org"},
			ns:            "myns",
			debug:         true,
			maxhistory:    5,
			burstLimit:    175,
			kubeAsUser:    "poro",
			kubeAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kubeCaFile:    "/my/ca.crt",
			kubeTLSServer: "example.org",
			kubeInsecure:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			for k, v := range tt.envvars {
				os.Setenv(k, v)
			}

			flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)

			settings := New()
			settings.AddFlags(flags)
			flags.Parse(strings.Split(tt.args, " "))

			if settings.Debug != tt.debug {
				t.Errorf("expected debug %t, got %t", tt.debug, settings.Debug)
			}
			if settings.Namespace() != tt.ns {
				t.Errorf("expected namespace %q, got %q", tt.ns, settings.Namespace())
			}
			if settings.KubeContext != tt.kcontext {
				t.Errorf("expected kube-context %q, got %q", tt.kcontext, settings.KubeContext)
			}
			if settings.MaxHistory != tt.maxhistory {
				t.Errorf("expected maxHistory %d, got %d", tt.maxhistory, settings.MaxHistory)
			}
			if tt.kubeAsUser != settings.KubeAsUser {
				t.Errorf("expected kAsUser %q, got %q", tt.kubeAsUser, settings.KubeAsUser)
			}
			if !reflect.DeepEqual(tt.kubeAsGroups, settings.KubeAsGroups) {
				t.Errorf("expected kAsGroups %+v, got %+v", len(tt.kubeAsGroups), len(settings.KubeAsGroups))
			}
			if tt.kubeCaFile != settings.KubeCaFile {
				t.Errorf("expected kCaFile %q, got %q", tt.kubeCaFile, settings.KubeCaFile)
			}
			if tt.burstLimit != settings.BurstLimit {
				t.Errorf("expected BurstLimit %d, got %d", tt.burstLimit, settings.BurstLimit)
			}
			if tt.kubeInsecure != settings.KubeInsecureSkipTLSVerify {
				t.Errorf("expected kubeInsecure %t, got %t", tt.kubeInsecure, settings.KubeInsecureSkipTLSVerify)
			}
			if tt.kubeTLSServer != settings.KubeTLSServerName {
				t.Errorf("expected kubeTLSServer %q, got %q", tt.kubeTLSServer, settings.KubeTLSServerName)
			}
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
				t.Cleanup(func() {
					os.Unsetenv(tt.env)
				})
				os.Setenv(tt.env, tt.val)
			}
			actual := envBoolOr(tt.env, tt.def)
			if actual != tt.expected {
				t.Errorf("expected result %t, got %t", tt.expected, actual)
			}
		})
	}
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
