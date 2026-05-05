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

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	ri "helm.sh/helm/v4/pkg/release"
	release "helm.sh/helm/v4/pkg/release/v1"
)

const (
	ExcludeNameFilter = "!name"
	IncludeNameFilter = "name"
)

// ReleaseTesting is the action for testing a release.
//
// It provides the implementation of 'helm test'.
type ReleaseTesting struct {
	cfg         *Configuration
	Timeout     time.Duration
	WaitOptions []kube.WaitOption
	// Used for fetching logs from test pods
	Namespace string
	Filters   map[string][]string
}

// NewReleaseTesting creates a new ReleaseTesting object with the given configuration.
func NewReleaseTesting(cfg *Configuration) *ReleaseTesting {
	return &ReleaseTesting{
		cfg:     cfg,
		Filters: map[string][]string{},
	}
}

// Run executes 'helm test' against the given release.
func (r *ReleaseTesting) Run(name string) (ri.Releaser, ExecuteShutdownFunc, error) {
	if err := r.cfg.KubeClient.IsReachable(); err != nil {
		return nil, shutdownNoOp, err
	}

	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, shutdownNoOp, fmt.Errorf("releaseTest: Release name is invalid: %s", name)
	}

	// finds the non-deleted release with the given name
	reli, err := r.cfg.Releases.Last(name)
	if err != nil {
		return reli, shutdownNoOp, err
	}

	rel, err := releaserToV1Release(reli)
	if err != nil {
		return reli, shutdownNoOp, err
	}

	skippedHooks := []*release.Hook{}
	executingHooks := []*release.Hook{}
	if len(r.Filters[ExcludeNameFilter]) != 0 {
		for _, h := range rel.Hooks {
			if slices.Contains(r.Filters[ExcludeNameFilter], h.Name) {
				skippedHooks = append(skippedHooks, h)
			} else {
				executingHooks = append(executingHooks, h)
			}
		}
		rel.Hooks = executingHooks
	}
	if len(r.Filters[IncludeNameFilter]) != 0 {
		executingHooks = nil
		for _, h := range rel.Hooks {
			if slices.Contains(r.Filters[IncludeNameFilter], h.Name) {
				executingHooks = append(executingHooks, h)
			} else {
				skippedHooks = append(skippedHooks, h)
			}
		}
		rel.Hooks = executingHooks
	}

	serverSideApply := rel.ApplyMethod == string(release.ApplyMethodServerSideApply)
	shutdown, err := r.cfg.execHookWithDelayedShutdown(rel, release.HookTest, kube.StatusWatcherStrategy, r.WaitOptions, r.Timeout, serverSideApply)

	if err != nil {
		rel.Hooks = append(skippedHooks, rel.Hooks...)
		r.cfg.Releases.Update(reli)
		return reli, shutdown, err
	}

	rel.Hooks = append(skippedHooks, rel.Hooks...)
	return reli, shutdown, r.cfg.Releases.Update(reli)
}

// GetPodLogs will write the logs for all test pods in the given release into
// the given writer. These can be immediately output to the user or captured for
// other uses
func (r *ReleaseTesting) GetPodLogs(out io.Writer, rel *release.Release) error {
	client, err := r.cfg.KubernetesClientSet()
	if err != nil {
		return fmt.Errorf("unable to get kubernetes client to fetch pod logs: %w", err)
	}

	hooksByWight := append([]*release.Hook{}, rel.Hooks...)
	sort.Stable(hookByWeight(hooksByWight))
	for _, h := range hooksByWight {
		for _, e := range h.Events {
			if e == release.HookTest {
				if slices.Contains(r.Filters[ExcludeNameFilter], h.Name) {
					continue
				}
				if len(r.Filters[IncludeNameFilter]) > 0 && !slices.Contains(r.Filters[IncludeNameFilter], h.Name) {
					continue
				}
				req := client.CoreV1().Pods(r.Namespace).GetLogs(h.Name, &v1.PodLogOptions{})
				logReader, err := req.Stream(context.Background())
				if err != nil {
					return fmt.Errorf("unable to get pod logs for %s: %w", h.Name, err)
				}

				fmt.Fprintf(out, "POD LOGS: %s\n", h.Name)
				_, err = io.Copy(out, logReader)
				fmt.Fprintln(out)
				if err != nil {
					return fmt.Errorf("unable to write pod logs for %s: %w", h.Name, err)
				}
			}
		}
	}
	return nil
}
