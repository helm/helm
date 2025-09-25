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

	v1release "helm.sh/helm/v4/pkg/release/v1"
)

var NewAccessor func(rel Releaser) (Accessor, error) = NewDefaultAccessor //nolint:revive

var NewHookAccessor func(rel Hook) (HookAccessor, error) = NewDefaultHookAccessor //nolint:revive

func NewDefaultAccessor(rel Releaser) (Accessor, error) {
	switch v := rel.(type) {
	case v1release.Release:
		return &v1Accessor{&v}, nil
	case *v1release.Release:
		return &v1Accessor{v}, nil
	default:
		return nil, errors.New("unsupported release type")
	}
}

func NewDefaultHookAccessor(hook Hook) (HookAccessor, error) {
	switch h := hook.(type) {
	case v1release.Hook:
		return &v1HookAccessor{&h}, nil
	case *v1release.Hook:
		return &v1HookAccessor{h}, nil
	default:
		return nil, errors.New("unsupported release hook type")
	}
}

type v1Accessor struct {
	rel *v1release.Release
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

type v1HookAccessor struct {
	hook *v1release.Hook
}

func (a *v1HookAccessor) Path() string {
	return a.hook.Path
}

func (a *v1HookAccessor) Manifest() string {
	return a.hook.Manifest
}
