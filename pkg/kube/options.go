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

package kube

import (
	"context"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
)

// WaitOption is a function that configures an option for waiting on resources.
type WaitOption func(*waitOptions)

// WithWaitContext sets the context for waiting on resources.
func WithWaitContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.ctx = ctx
	}
}

// WithKStatusReaders sets the status readers to be used while waiting on resources.
func WithKStatusReaders(readers ...engine.StatusReader) WaitOption {
	return func(wo *waitOptions) {
		wo.statusReaders = readers
	}
}

type waitOptions struct {
	ctx           context.Context
	statusReaders []engine.StatusReader
}
