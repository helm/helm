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

	"k8s.io/helm/pkg/helm/helmpath"

	"github.com/spf13/pflag"
)

func TestEnvSettings(t *testing.T) {
	tests := []struct {
		name string

		// input
		args   []string
		envars map[string]string

		// expected values
		home, host, ns, kcontext, kconfig, plugins string
		debug, tlsverify                           bool
	}{
		{
			name:      "defaults",
			args:      []string{},
			home:      DefaultHelmHome,
			plugins:   helmpath.Home(DefaultHelmHome).Plugins(),
			ns:        "kube-system",
			tlsverify: false,
		},
		{
			name:      "with flags set",
			args:      []string{"--home", "/foo", "--host=here", "--debug", "--tiller-namespace=myns", "--kubeconfig", "/bar"},
			home:      "/foo",
			plugins:   helmpath.Home("/foo").Plugins(),
			host:      "here",
			ns:        "myns",
			kconfig:   "/bar",
			debug:     true,
			tlsverify: false,
		},
		{
			name:      "with envvars set",
			args:      []string{},
			envars:    map[string]string{"HELM_HOME": "/bar", "HELM_HOST": "there", "HELM_DEBUG": "1", "TILLER_NAMESPACE": "yourns"},
			home:      "/bar",
			plugins:   helmpath.Home("/bar").Plugins(),
			host:      "there",
			ns:        "yourns",
			debug:     true,
			tlsverify: false,
		},
		{
			name:      "with TLS envvars set",
			args:      []string{},
			envars:    map[string]string{"HELM_HOME": "/bar", "HELM_HOST": "there", "HELM_DEBUG": "1", "TILLER_NAMESPACE": "yourns", "HELM_TLS_VERIFY": "1"},
			home:      "/bar",
			plugins:   helmpath.Home("/bar").Plugins(),
			host:      "there",
			ns:        "yourns",
			debug:     true,
			tlsverify: true,
		},
		{
			name:      "with flags and envvars set",
			args:      []string{"--home", "/foo", "--host=here", "--debug", "--tiller-namespace=myns"},
			envars:    map[string]string{"HELM_HOME": "/bar", "HELM_HOST": "there", "HELM_DEBUG": "1", "TILLER_NAMESPACE": "yourns", "HELM_PLUGIN": "glade"},
			home:      "/foo",
			plugins:   "glade",
			host:      "here",
			ns:        "myns",
			debug:     true,
			tlsverify: false,
		},
	}

	allEnvvars := map[string]string{
		"HELM_DEBUG":        "",
		"HELM_HOME":         "",
		"HELM_HOST":         "",
		"TILLER_NAMESPACE":  "",
		"HELM_PLUGIN":       "",
		"HELM_TLS_HOSTNAME": "",
		"HELM_TLS_CA_CERT":  "",
		"HELM_TLS_CERT":     "",
		"HELM_TLS_KEY":      "",
		"HELM_TLS_VERIFY":   "",
		"HELM_TLS_ENABLE":   "",
	}

	resetEnv(allEnvvars)
	defer resetEnv(allEnvvars)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envars {
				os.Setenv(k, v)
			}

			flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)

			settings := &EnvSettings{}
			settings.AddFlags(flags)
			settings.AddFlagsTLS(flags)
			flags.Parse(tt.args)

			settings.Init(flags)
			settings.InitTLS(flags)

			if settings.Home != helmpath.Home(tt.home) {
				t.Errorf("expected home %q, got %q", tt.home, settings.Home)
			}
			if settings.PluginDirs() != tt.plugins {
				t.Errorf("expected plugins %q, got %q", tt.plugins, settings.PluginDirs())
			}
			if settings.TillerHost != tt.host {
				t.Errorf("expected host %q, got %q", tt.host, settings.TillerHost)
			}
			if settings.Debug != tt.debug {
				t.Errorf("expected debug %t, got %t", tt.debug, settings.Debug)
			}
			if settings.TillerNamespace != tt.ns {
				t.Errorf("expected tiller-namespace %q, got %q", tt.ns, settings.TillerNamespace)
			}
			if settings.KubeContext != tt.kcontext {
				t.Errorf("expected kube-context %q, got %q", tt.kcontext, settings.KubeContext)
			}
			if settings.KubeConfig != tt.kconfig {
				t.Errorf("expected kubeconfig %q, got %q", tt.kconfig, settings.KubeConfig)
			}
			if settings.TLSVerify != tt.tlsverify {
				t.Errorf("expected tls-verify %t, got %t", tt.tlsverify, settings.TLSVerify)
			}

			resetEnv(tt.envars)
		})
	}
}

func resetEnv(envars map[string]string) func() {
	origEnv := os.Environ()

	// ensure any local envvars do not hose us
	for e := range envars {
		os.Unsetenv(e)
	}

	return func() {
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}
