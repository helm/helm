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

import "fmt"

type MissingConfigError struct {
	string
}

func (e MissingConfigError) Error() string {
	return fmt.Sprintf("missing config error: %s param missing from client configuration", e.string)
}

// Checks whether the error yielded by the Validate function contains MissingConfigErrors.
func HasMissingConfigErrors(errs []error) bool {
	return len(errs) > 0
}

// Ensures that the required fields are set on the Settings struct and returns a list of MissingConfigError
// for all missing fields.
func (s *Settings) Validate() []error {
	errs := make([]error, 0)

	if s.Namespace == "" {
		appendError(&errs, "Namespace")
	}
	if s.HelmDriver == "" {
		appendError(&errs, "HelmDriver")
	}
	if s.KubeConfig == "" {
		appendError(&errs, "KubeConfig")
	}
	if s.KubeContext == "" {
		appendError(&errs, "KubeContext")
	}
	if s.KubeToken == "" {
		appendError(&errs, "KubeToken")
	}
	if s.KubeAPIServer == "" {
		appendError(&errs, "KubeAPIServer")
	}
	if s.RegistryConfig == "" {
		appendError(&errs, "RegistryConfig")
	}
	if s.RepositoryConfig == "" {
		appendError(&errs, "RepositoryConfig")
	}
	if s.RepositoryCache == "" {
		appendError(&errs, "RepositoryCache")
	}
	if s.PluginsDirectory == "" {
		appendError(&errs, "RepositoryCache")
	}

	return errs
}

func appendError(errs *[]error, field string) {
	*errs = append(*errs, MissingConfigError{field})
}
