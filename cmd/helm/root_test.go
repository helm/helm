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
	"k8s.io/helm/pkg/helm/environment"
	"os"
	"testing"
)

func TestRootCmd(t *testing.T) {
	defer resetEnv()()

	tests := []struct {
		name, args, home string
		envars           map[string]string
	}{
		{
			name: "defaults",
			args: "home",
			home: environment.GetDefaultConfigHome(),
		},
		{
			name: "with --home set",
			args: "--home /foo",
			home: "/foo",
		},
		{
			name: "subcommands with --home set",
			args: "home --home /foo",
			home: "/foo",
		},
		{
			name:   "with $HELM_HOME set",
			args:   "home",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/bar",
		},
		{
			name:   "subcommands with $HELM_HOME set",
			args:   "home",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/bar",
		},
		{
			name:   "with $HELM_HOME and --home set",
			args:   "home --home /foo",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/foo",
		},
	}

	// ensure not set locally
	os.Unsetenv("HELM_HOME")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Unsetenv("HELM_HOME")

			for k, v := range tt.envars {
				os.Setenv(k, v)
			}

			cmd, _, err := executeCommandC(nil, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if settings.Home.String() != tt.home {
				t.Errorf("expected home %q, got %q", tt.home, settings.Home)
			}
			homeFlag := cmd.Flag("home").Value.String()
			homeFlag = os.ExpandEnv(homeFlag)
			if homeFlag != tt.home {
				t.Errorf("expected home %q, got %q", tt.home, homeFlag)
			}
		})
	}
}
