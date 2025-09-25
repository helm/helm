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

package chart

import (
	"errors"

	v3chart "helm.sh/helm/v4/internal/chart/v3"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

var NewDependencyAccessor func(dep Dependency) (DependencyAccessor, error) = NewDefaultDependencyAccessor //nolint:revive

func NewDefaultDependencyAccessor(dep Dependency) (DependencyAccessor, error) {
	switch v := dep.(type) {
	case v2chart.Dependency:
		return &v2DependencyAccessor{&v}, nil
	case *v2chart.Dependency:
		return &v2DependencyAccessor{v}, nil
	case v3chart.Dependency:
		return &v3DependencyAccessor{&v}, nil
	case *v3chart.Dependency:
		return &v3DependencyAccessor{v}, nil
	default:
		return nil, errors.New("unsupported chart dependency type")
	}
}

type v2DependencyAccessor struct {
	dep *v2chart.Dependency
}

func (r *v2DependencyAccessor) Name() string {
	return r.dep.Name
}

func (r *v2DependencyAccessor) Alias() string {
	return r.dep.Alias
}

type v3DependencyAccessor struct {
	dep *v3chart.Dependency
}

func (r *v3DependencyAccessor) Name() string {
	return r.dep.Name
}

func (r *v3DependencyAccessor) Alias() string {
	return r.dep.Alias
}
