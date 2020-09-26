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

package engine

import (
	"k8s.io/client-go/rest"
)

// Option allows specifying various settings configurable by the user within Engine
type Option func(*Engine) error

// WithConfig specifies a rest.Config for the Engine during rendering
func WithConfig(config *rest.Config) Option {
	return func(e *Engine) error {
		e.config = config
		return nil
	}
}

// WithLintMode allows missing required funcs to not fail on render
func WithLintMode(lintMode bool) Option {
	return func(e *Engine) error {
		e.LintMode = lintMode
		return nil
	}
}

// WithStrict will cause rendering to fail if a referenced value is not passed
// into the render
func WithStrict(strict bool) Option {
	return func(e *Engine) error {
		e.Strict = strict
		return nil
	}
}
