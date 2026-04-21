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
// If unset, context.Background() will be used.
func WithWaitContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.ctx = ctx
	}
}

// WithWatchUntilReadyMethodContext sets the context specifically for the WatchUntilReady method.
// If unset, the context set by `WithWaitContext` will be used (falling back to `context.Background()`).
func WithWatchUntilReadyMethodContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.watchUntilReadyCtx = ctx
	}
}

// WithWaitMethodContext sets the context specifically for the Wait method.
// If unset, the context set by `WithWaitContext` will be used (falling back to `context.Background()`).
func WithWaitMethodContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.waitCtx = ctx
	}
}

// WithWaitWithJobsMethodContext sets the context specifically for the WaitWithJobs method.
// If unset, the context set by `WithWaitContext` will be used (falling back to `context.Background()`).
func WithWaitWithJobsMethodContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.waitWithJobsCtx = ctx
	}
}

// WithWaitForDeleteMethodContext sets the context specifically for the WaitForDelete method.
// If unset, the context set by `WithWaitContext` will be used (falling back to `context.Background()`).
func WithWaitForDeleteMethodContext(ctx context.Context) WaitOption {
	return func(wo *waitOptions) {
		wo.waitForDeleteCtx = ctx
	}
}

// WithKStatusReaders sets the status readers to be used while waiting on resources.
func WithKStatusReaders(readers ...engine.StatusReader) WaitOption {
	return func(wo *waitOptions) {
		wo.statusReaders = readers
	}
}

// WithStatusComputeWorkers sets the number of concurrent goroutines used to
// compute object status per informer. This prevents the informer notification
// pipeline from being blocked by slow API calls (e.g., LIST ReplicaSets/Pods
// for Deployments) when many resources are updated simultaneously.
//
// A value of 0 (the default) keeps the underlying cli-utils behavior, where
// status is computed synchronously on the informer goroutine. SDK consumers
// (for example helm-controller) inherit this conservative default and can
// opt in explicitly. The Helm CLI passes a non-zero value so that
// `helm install/upgrade/rollback` users get the fix for multi-minute waits
// out of the box. See https://github.com/fluxcd/cli-utils/pull/20.
func WithStatusComputeWorkers(n int) WaitOption {
	return func(wo *waitOptions) {
		wo.statusComputeWorkers = n
	}
}

type waitOptions struct {
	ctx                  context.Context
	watchUntilReadyCtx   context.Context
	waitCtx              context.Context
	waitWithJobsCtx      context.Context
	waitForDeleteCtx     context.Context
	statusReaders        []engine.StatusReader
	statusComputeWorkers int
}
