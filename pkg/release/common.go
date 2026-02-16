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
	"errors"
	"fmt"
	"time"

	v2release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart"
	v1release "helm.sh/helm/v4/pkg/release/v1"
)

var NewAccessor func(rel Releaser) (Accessor, error) = newDefaultAccessor //nolint:revive

var NewHookAccessor func(rel Hook) (HookAccessor, error) = newDefaultHookAccessor //nolint:revive

func newDefaultAccessor(rel Releaser) (Accessor, error) {
	switch v := rel.(type) {
	case v1release.Release:
		return &v1Accessor{&v}, nil
	case *v1release.Release:
		return &v1Accessor{v}, nil
	case v2release.Release:
		return &v2Accessor{&v}, nil
	case *v2release.Release:
		return &v2Accessor{v}, nil
	default:
		return nil, fmt.Errorf("unsupported release type: %T", rel)
	}
}

func newDefaultHookAccessor(hook Hook) (HookAccessor, error) {
	switch h := hook.(type) {
	case v1release.Hook:
		return &v1HookAccessor{&h}, nil
	case *v1release.Hook:
		return &v1HookAccessor{h}, nil
	case v2release.Hook:
		return &v2HookAccessor{&h}, nil
	case *v2release.Hook:
		return &v2HookAccessor{h}, nil
	default:
		return nil, errors.New("unsupported release hook type")
	}
}

type v1Accessor struct {
	rel *v1release.Release
}

func (a *v1Accessor) Name() string {
	return a.rel.Name
}

func (a *v1Accessor) Namespace() string {
	return a.rel.Namespace
}

func (a *v1Accessor) Version() int {
	return a.rel.Version
}

func (a *v1Accessor) Hooks() []Hook {
	var hooks = make([]Hook, len(a.rel.Hooks))
	for i, h := range a.rel.Hooks {
		hooks[i] = h
	}
	return hooks
}

func (a *v1Accessor) Manifest() string {
	return a.rel.Manifest
}

func (a *v1Accessor) Notes() string {
	return a.rel.Info.Notes
}

func (a *v1Accessor) Labels() map[string]string {
	return a.rel.Labels
}

func (a *v1Accessor) Chart() chart.Charter {
	return a.rel.Chart
}

func (a *v1Accessor) Status() string {
	return a.rel.Info.Status.String()
}

func (a *v1Accessor) ApplyMethod() string {
	return a.rel.ApplyMethod
}

func (a *v1Accessor) DeployedAt() time.Time {
	return a.rel.Info.LastDeployed
}

type v1HookAccessor struct {
	hook *v1release.Hook
}

func (a *v1HookAccessor) Path() string {
	return a.hook.Path
}

func (a *v1HookAccessor) Manifest() string {
	return a.hook.Manifest
}

type v2Accessor struct {
	rel *v2release.Release
}

func (a *v2Accessor) Name() string {
	return a.rel.Name
}

func (a *v2Accessor) Namespace() string {
	return a.rel.Namespace
}

func (a *v2Accessor) Version() int {
	return a.rel.Version
}

func (a *v2Accessor) Hooks() []Hook {
	var hooks = make([]Hook, len(a.rel.Hooks))
	for i, h := range a.rel.Hooks {
		hooks[i] = h
	}
	return hooks
}

func (a *v2Accessor) Manifest() string {
	return a.rel.Manifest
}

func (a *v2Accessor) Notes() string {
	return a.rel.Info.Notes
}

func (a *v2Accessor) Labels() map[string]string {
	return a.rel.Labels
}

func (a *v2Accessor) Chart() chart.Charter {
	return a.rel.Chart
}

func (a *v2Accessor) Status() string {
	return a.rel.Info.Status.String()
}

func (a *v2Accessor) ApplyMethod() string {
	return a.rel.ApplyMethod
}

func (a *v2Accessor) DeployedAt() time.Time {
	return a.rel.Info.LastDeployed
}

type v2HookAccessor struct {
	hook *v2release.Hook
}

func (a *v2HookAccessor) Path() string {
	return a.hook.Path
}

func (a *v2HookAccessor) Manifest() string {
	return a.hook.Manifest
}
