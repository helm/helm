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

package environment

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"k8s.io/helm/pkg/helm/helmpath"
)

func TestEnvSettings(t *testing.T) {
	tests := []struct {
		name string

		// input
		args   string
		envars map[string]string

		// expected values
		home, ns, kcontext, plugins string
		debug                       bool
	}{
		{
			name:    "defaults",
			home:    defaultHelmHome,
			plugins: helmpath.Home(defaultHelmHome).Plugins(),
			ns:      "",
		},
		{
			name:    "with flags set",
			args:    "--home /foo --debug --namespace=myns",
			home:    "/foo",
			plugins: helmpath.Home("/foo").Plugins(),
			ns:      "myns",
			debug:   true,
		},
		{
			name:    "with envvars set",
			envars:  map[string]string{"HELM_HOME": "/bar", "HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns"},
			home:    "/bar",
			plugins: helmpath.Home("/bar").Plugins(),
			ns:      "yourns",
			debug:   true,
		},
		{
			name:    "with flags and envvars set",
			args:    "--home /foo --debug --namespace=myns",
			envars:  map[string]string{"HELM_HOME": "/bar", "HELM_DEBUG": "1", "HELM_NAMESPACE": "yourns", "HELM_PLUGIN": "glade"},
			home:    "/foo",
			plugins: "glade",
			ns:      "myns",
			debug:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			for k, v := range tt.envars {
				os.Setenv(k, v)
			}

			flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)

			settings := &EnvSettings{}
			settings.AddFlags(flags)
			flags.Parse(strings.Split(tt.args, " "))

			settings.Init(flags)

			if settings.Home != helmpath.Home(tt.home) {
				t.Errorf("expected home %q, got %q", tt.home, settings.Home)
			}
			if settings.PluginDirs() != tt.plugins {
				t.Errorf("expected plugins %q, got %q", tt.plugins, settings.PluginDirs())
			}
			if settings.Debug != tt.debug {
				t.Errorf("expected debug %t, got %t", tt.debug, settings.Debug)
			}
			if settings.Namespace != tt.ns {
				t.Errorf("expected namespace %q, got %q", tt.ns, settings.Namespace)
			}
			if settings.KubeContext != tt.kcontext {
				t.Errorf("expected kube-context %q, got %q", tt.kcontext, settings.KubeContext)
			}
		})
	}
}

func resetEnv() func() {
	origEnv := os.Environ()

	// ensure any local envvars do not hose us
	for _, e := range envMap {
		os.Unsetenv(e)
	}

	return func() {
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}
