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

package action

// Dependency is the action for building a given chart's dependency tree.
//
// It provides the implementation of 'helm dependency' and its respective subcommands.
type Dependency struct {
	Verify      bool
	Keyring     string
	SkipRefresh bool
}

// NewDependency creates a new Dependency object with the given configuration.
func NewDependency() *Dependency {
	return &Dependency{}
}
