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
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	release "helm.sh/helm/v4/pkg/release/v1"
	helmtime "helm.sh/helm/v4/pkg/time"
)

// Rollback is the action for rolling back to a given release.
//
// It provides the implementation of 'helm rollback'.
type Rollback struct {
	cfg *Configuration

	Version      int
	Timeout      time.Duration
	WaitStrategy kube.WaitStrategy
	WaitForJobs  bool
	DisableHooks bool
	// DryRunStrategy can be set to prepare, but not execute the operation and whether or not to interact with the remote cluster
	DryRunStrategy DryRunStrategy
	Force          bool // will (if true) force resource upgrade through uninstall/recreate if needed
	CleanupOnFail  bool
	MaxHistory     int // MaxHistory limits the maximum number of revisions saved per release
}

// NewRollback creates a new Rollback object with the given configuration.
func NewRollback(cfg *Configuration) *Rollback {
	return &Rollback{
		cfg:            cfg,
		DryRunStrategy: DryRunNone,
	}
}

// Run executes 'helm rollback' against the given release.
func (r *Rollback) Run(name string) error {
	if err := r.cfg.KubeClient.IsReachable(); err != nil {
		return err
	}

	r.cfg.Releases.MaxHistory = r.MaxHistory

	slog.Debug("preparing rollback", "name", name)
	currentRelease, targetRelease, err := r.prepareRollback(name)
	if err != nil {
		return err
	}

	if !isDryRun(r.DryRunStrategy) {
		slog.Debug("creating rolled back release", "name", name)
		if err := r.cfg.Releases.Create(targetRelease); err != nil {
			return err
		}
	}

	slog.Debug("performing rollback", "name", name)
	if _, err := r.performRollback(currentRelease, targetRelease); err != nil {
		return err
	}

	if !isDryRun(r.DryRunStrategy) {
		slog.Debug("updating status for rolled back release", "name", name)
		if err := r.cfg.Releases.Update(targetRelease); err != nil {
			return err
		}
	}
	return nil
}

// prepareRollback finds the previous release and prepares a new release object with
// the previous release's configuration
func (r *Rollback) prepareRollback(name string) (*release.Release, *release.Release, error) {
	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, nil, fmt.Errorf("prepareRollback: Release name is invalid: %s", name)
	}

	if r.Version < 0 {
		return nil, nil, errInvalidRevision
	}

	currentRelease, err := r.cfg.Releases.Last(name)
	if err != nil {
		return nil, nil, err
	}

	previousVersion := r.Version
	if r.Version == 0 {
		previousVersion = currentRelease.Version - 1
	}

	historyReleases, err := r.cfg.Releases.History(name)
	if err != nil {
		return nil, nil, err
	}

	// Check if the history version to be rolled back exists
	previousVersionExist := false
	for _, historyRelease := range historyReleases {
		version := historyRelease.Version
		if previousVersion == version {
			previousVersionExist = true
			break
		}
	}
	if !previousVersionExist {
		return nil, nil, fmt.Errorf("release has no %d version", previousVersion)
	}

	slog.Debug("rolling back", "name", name, "currentVersion", currentRelease.Version, "targetVersion", previousVersion)

	previousRelease, err := r.cfg.Releases.Get(name, previousVersion)
	if err != nil {
		return nil, nil, err
	}

	// Store a new release object with previous release's configuration
	targetRelease := &release.Release{
		Name:      name,
		Namespace: currentRelease.Namespace,
		Chart:     previousRelease.Chart,
		Config:    previousRelease.Config,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  helmtime.Now(),
			Status:        release.StatusPendingRollback,
			Notes:         previousRelease.Info.Notes,
			// Because we lose the reference to previous version elsewhere, we set the
			// message here, and only override it later if we experience failure.
			Description: fmt.Sprintf("Rollback to %d", previousVersion),
		},
		Version:  currentRelease.Version + 1,
		Labels:   previousRelease.Labels,
		Manifest: previousRelease.Manifest,
		Hooks:    previousRelease.Hooks,
	}

	return currentRelease, targetRelease, nil
}

func (r *Rollback) performRollback(currentRelease, targetRelease *release.Release) (*release.Release, error) {
	if isDryRun(r.DryRunStrategy) {
		slog.Debug("dry run", "name", targetRelease.Name)
		return targetRelease, nil
	}

	current, err := r.cfg.KubeClient.Build(bytes.NewBufferString(currentRelease.Manifest), false)
	if err != nil {
		return targetRelease, fmt.Errorf("unable to build kubernetes objects from current release manifest: %w", err)
	}
	target, err := r.cfg.KubeClient.Build(bytes.NewBufferString(targetRelease.Manifest), false)
	if err != nil {
		return targetRelease, fmt.Errorf("unable to build kubernetes objects from new release manifest: %w", err)
	}

	// pre-rollback hooks
	if !r.DisableHooks {
		if err := r.cfg.execHook(targetRelease, release.HookPreRollback, r.WaitStrategy, r.Timeout); err != nil {
			return targetRelease, err
		}
	} else {
		slog.Debug("rollback hooks disabled", "name", targetRelease.Name)
	}

	// It is safe to use "force" here because these are resources currently rendered by the chart.
	err = target.Visit(setMetadataVisitor(targetRelease.Name, targetRelease.Namespace, true))
	if err != nil {
		return targetRelease, fmt.Errorf("unable to set metadata visitor from target release: %w", err)
	}
	results, err := r.cfg.KubeClient.Update(current, target, r.Force)

	if err != nil {
		msg := fmt.Sprintf("Rollback %q failed: %s", targetRelease.Name, err)
		slog.Warn(msg)
		currentRelease.Info.Status = release.StatusSuperseded
		targetRelease.Info.Status = release.StatusFailed
		targetRelease.Info.Description = msg
		r.cfg.recordRelease(currentRelease)
		r.cfg.recordRelease(targetRelease)
		if r.CleanupOnFail {
			slog.Debug("cleanup on fail set, cleaning up resources", "count", len(results.Created))
			_, errs := r.cfg.KubeClient.Delete(results.Created)
			if errs != nil {
				return targetRelease, fmt.Errorf(
					"an error occurred while cleaning up resources. original rollback error: %w",
					fmt.Errorf("unable to cleanup resources: %w", joinErrors(errs, ", ")))
			}
			slog.Debug("resource cleanup complete")
		}
		return targetRelease, err
	}

	waiter, err := r.cfg.KubeClient.GetWaiter(r.WaitStrategy)
	if err != nil {
		return nil, fmt.Errorf("unable to set metadata visitor from target release: %w", err)
	}
	if r.WaitForJobs {
		if err := waiter.WaitWithJobs(target, r.Timeout); err != nil {
			targetRelease.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", targetRelease.Name, err.Error()))
			r.cfg.recordRelease(currentRelease)
			r.cfg.recordRelease(targetRelease)
			return targetRelease, fmt.Errorf("release %s failed: %w", targetRelease.Name, err)
		}
	} else {
		if err := waiter.Wait(target, r.Timeout); err != nil {
			targetRelease.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", targetRelease.Name, err.Error()))
			r.cfg.recordRelease(currentRelease)
			r.cfg.recordRelease(targetRelease)
			return targetRelease, fmt.Errorf("release %s failed: %w", targetRelease.Name, err)
		}
	}

	// post-rollback hooks
	if !r.DisableHooks {
		if err := r.cfg.execHook(targetRelease, release.HookPostRollback, r.WaitStrategy, r.Timeout); err != nil {
			return targetRelease, err
		}
	}

	deployed, err := r.cfg.Releases.DeployedAll(currentRelease.Name)
	if err != nil && !strings.Contains(err.Error(), "has no deployed releases") {
		return nil, err
	}
	// Supersede all previous deployments, see issue #2941.
	for _, rel := range deployed {
		slog.Debug("superseding previous deployment", "version", rel.Version)
		rel.Info.Status = release.StatusSuperseded
		r.cfg.recordRelease(rel)
	}

	targetRelease.Info.Status = release.StatusDeployed

	return targetRelease, nil
}
