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
		ns, kcontext string
		debug        bool
		maxhistory   int
		kAsUser      string
		kAsGroups    []string
		kCaFile      string
	}{
		{
			name:       "defaults",
			ns:         "default",
			maxhistory: defaultMaxHistory,
		},
		{
			name:       "with flags set",
			args:       "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/tmp/ca.crt",
			ns:         "myns",
			debug:      true,
			maxhistory: defaultMaxHistory,
			kAsUser:    "poro",
			kAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kCaFile:    "/tmp/ca.crt",
		},
		{
			name:       "with envvars set",
			envvars:    map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt"},
			ns:         "yourns",
			maxhistory: 5,
			debug:      true,
			kAsUser:    "pikachu",
			kAsGroups:  []string{"operators", "snackeaters", "partyanimals"},
			kCaFile:    "/tmp/ca.crt",
		},
		{
			name:       "with flags and envvars set",
			args:       "--debug --namespace=myns --kube-as-user=poro --kube-as-group=admins --kube-as-group=teatime --kube-as-group=snackeaters --kube-ca-file=/my/ca.crt",
			envvars:    map[string]string{"HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_KUBEASUSER": "pikachu", "HELM_KUBEASGROUPS": ",,,operators,snackeaters,partyanimals", "HELM_MAX_HISTORY": "5", "HELM_KUBECAFILE": "/tmp/ca.crt"},
			ns:         "myns",
			debug:      true,
			maxhistory: 5,
			kAsUser:    "poro",
			kAsGroups:  []string{"admins", "teatime", "snackeaters"},
			kCaFile:    "/my/ca.crt",
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
			if tt.kAsUser != settings.KubeAsUser {
				t.Errorf("expected kAsUser %q, got %q", tt.kAsUser, settings.KubeAsUser)
			}
			if !reflect.DeepEqual(tt.kAsGroups, settings.KubeAsGroups) {
				t.Errorf("expected kAsGroups %+v, got %+v", len(tt.kAsGroups), len(settings.KubeAsGroups))
			}
			if tt.kCaFile != settings.KubeCaFile {
				t.Errorf("expected kCaFile %q, got %q", tt.kCaFile, settings.KubeCaFile)
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
