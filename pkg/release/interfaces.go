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

package release

import (
	"time"

	"helm.sh/helm/v4/pkg/chart"
)

type Releaser interface{}

type Hook interface{}

type Accessor interface {
	Name() string
	Namespace() string
	Version() int
	Hooks() []Hook
	Manifest() string
	Notes() string
	Labels() map[string]string
	Chart() chart.Charter
	Status() string
	ApplyMethod() string
	DeployedAt() time.Time
}

type HookAccessor interface {
	Path() string
	Manifest() string
}
