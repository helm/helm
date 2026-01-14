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
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// Rollback is the action for rolling back to a given release.
//
// It provides the implementation of 'helm rollback'.
type Rollback struct {
	cfg *Configuration

	Version      int
	Timeout      time.Duration
	WaitStrategy kube.WaitStrategy
	WaitOptions  []kube.WaitOption
	WaitForJobs  bool
	DisableHooks bool
	// DryRunStrategy can be set to prepare, but not execute the operation and whether or not to interact with the remote cluster
	DryRunStrategy DryRunStrategy
	// ForceReplace will, if set to `true`, ignore certain warnings and perform the rollback anyway.
	//
	// This should be used with caution.
	ForceReplace bool
	// ForceConflicts causes server-side apply to force conflicts ("Overwrite value, become sole manager")
	// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
	ForceConflicts bool
	// ServerSideApply enables changes to be applied via Kubernetes server-side apply
	// Can be the string: "true", "false" or "auto"
	// When "auto", sever-side usage will be based upon the releases previous usage
	// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/
	ServerSideApply string
	CleanupOnFail   bool
	MaxHistory      int // MaxHistory limits the maximum number of revisions saved per release
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

	r.cfg.Logger().Debug("preparing rollback", "name", name)
	currentRelease, targetRelease, serverSideApply, err := r.prepareRollback(name)
	if err != nil {
		return err
	}

	if !isDryRun(r.DryRunStrategy) {
		r.cfg.Logger().Debug("creating rolled back release", "name", name)
		if err := r.cfg.Releases.Create(targetRelease); err != nil {
			return err
		}
	}

	r.cfg.Logger().Debug("performing rollback", "name", name)
	if _, err := r.performRollback(currentRelease, targetRelease, serverSideApply); err != nil {
		return err
	}

	if !isDryRun(r.DryRunStrategy) {
		r.cfg.Logger().Debug("updating status for rolled back release", "name", name)
		if err := r.cfg.Releases.Update(targetRelease); err != nil {
			return err
		}
	}
	return nil
}

// prepareRollback finds the previous release and prepares a new release object with
// the previous release's configuration
func (r *Rollback) prepareRollback(name string) (*release.Release, *release.Release, bool, error) {
	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, nil, false, fmt.Errorf("prepareRollback: Release name is invalid: %s", name)
	}

	if r.Version < 0 {
		return nil, nil, false, errInvalidRevision
	}

	currentReleasei, err := r.cfg.Releases.Last(name)
	if err != nil {
		return nil, nil, false, err
	}

	currentRelease, err := releaserToV1Release(currentReleasei)
	if err != nil {
		return nil, nil, false, err
	}

	previousVersion := r.Version
	if r.Version == 0 {
		previousVersion = currentRelease.Version - 1
	}

	historyReleases, err := r.cfg.Releases.History(name)
	if err != nil {
		return nil, nil, false, err
	}

	// Check if the history version to be rolled back exists
	previousVersionExist := false
	for _, historyReleasei := range historyReleases {
		historyRelease, err := releaserToV1Release(historyReleasei)
		if err != nil {
			return nil, nil, false, err
		}
		version := historyRelease.Version
		if previousVersion == version {
			previousVersionExist = true
			break
		}
	}
	if !previousVersionExist {
		return nil, nil, false, fmt.Errorf("release has no %d version", previousVersion)
	}

	r.cfg.Logger().Debug("rolling back", "name", name, "currentVersion", currentRelease.Version, "targetVersion", previousVersion)

	previousReleasei, err := r.cfg.Releases.Get(name, previousVersion)
	if err != nil {
		return nil, nil, false, err
	}
	previousRelease, err := releaserToV1Release(previousReleasei)
	if err != nil {
		return nil, nil, false, err
	}

	serverSideApply, err := getUpgradeServerSideValue(r.ServerSideApply, previousRelease.ApplyMethod)
	if err != nil {
		return nil, nil, false, err
	}

	// Store a new release object with previous release's configuration
	targetRelease := &release.Release{
		Name:      name,
		Namespace: currentRelease.Namespace,
		Chart:     previousRelease.Chart,
		Config:    previousRelease.Config,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  time.Now(),
			Status:        common.StatusPendingRollback,
			Notes:         previousRelease.Info.Notes,
			// Because we lose the reference to previous version elsewhere, we set the
			// message here, and only override it later if we experience failure.
			Description: fmt.Sprintf("Rollback to %d", previousVersion),
		},
		Version:     currentRelease.Version + 1,
		Labels:      previousRelease.Labels,
		Manifest:    previousRelease.Manifest,
		Hooks:       previousRelease.Hooks,
		ApplyMethod: string(determineReleaseSSApplyMethod(serverSideApply)),
	}

	return currentRelease, targetRelease, serverSideApply, nil
}

func (r *Rollback) performRollback(currentRelease, targetRelease *release.Release, serverSideApply bool) (*release.Release, error) {
	if isDryRun(r.DryRunStrategy) {
		r.cfg.Logger().Debug("dry run", "name", targetRelease.Name)
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
		if err := r.cfg.execHook(targetRelease, release.HookPreRollback, r.WaitStrategy, r.WaitOptions, r.Timeout, serverSideApply); err != nil {
			return targetRelease, err
		}
	} else {
		r.cfg.Logger().Debug("rollback hooks disabled", "name", targetRelease.Name)
	}

	// It is safe to use "forceOwnership" here because these are resources currently rendered by the chart.
	err = target.Visit(setMetadataVisitor(targetRelease.Name, targetRelease.Namespace, true))
	if err != nil {
		return targetRelease, fmt.Errorf("unable to set metadata visitor from target release: %w", err)
	}
	results, err := r.cfg.KubeClient.Update(
		current,
		target,
		kube.ClientUpdateOptionForceReplace(r.ForceReplace),
		kube.ClientUpdateOptionServerSideApply(serverSideApply, r.ForceConflicts),
		kube.ClientUpdateOptionThreeWayMergeForUnstructured(false),
		kube.ClientUpdateOptionUpgradeClientSideFieldManager(true))

	if err != nil {
		msg := fmt.Sprintf("Rollback %q failed: %s", targetRelease.Name, err)
		r.cfg.Logger().Warn(msg)
		currentRelease.Info.Status = common.StatusSuperseded
		targetRelease.Info.Status = common.StatusFailed
		targetRelease.Info.Description = msg
		r.cfg.recordRelease(currentRelease)
		r.cfg.recordRelease(targetRelease)
		if r.CleanupOnFail {
			r.cfg.Logger().Debug("cleanup on fail set, cleaning up resources", "count", len(results.Created))
			_, errs := r.cfg.KubeClient.Delete(results.Created, metav1.DeletePropagationBackground)
			if errs != nil {
				return targetRelease, fmt.Errorf(
					"an error occurred while cleaning up resources. original rollback error: %w",
					fmt.Errorf("unable to cleanup resources: %w", joinErrors(errs, ", ")))
			}
			r.cfg.Logger().Debug("resource cleanup complete")
		}
		return targetRelease, err
	}

	var waiter kube.Waiter
	if c, supportsOptions := r.cfg.KubeClient.(kube.InterfaceWaitOptions); supportsOptions {
		waiter, err = c.GetWaiterWithOptions(r.WaitStrategy, r.WaitOptions...)
	} else {
		waiter, err = r.cfg.KubeClient.GetWaiter(r.WaitStrategy)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to get waiter: %w", err)
	}
	if r.WaitForJobs {
		if err := waiter.WaitWithJobs(target, r.Timeout); err != nil {
			targetRelease.SetStatus(common.StatusFailed, fmt.Sprintf("Release %q failed: %s", targetRelease.Name, err.Error()))
			r.cfg.recordRelease(currentRelease)
			r.cfg.recordRelease(targetRelease)
			return targetRelease, fmt.Errorf("release %s failed: %w", targetRelease.Name, err)
		}
	} else {
		if err := waiter.Wait(target, r.Timeout); err != nil {
			targetRelease.SetStatus(common.StatusFailed, fmt.Sprintf("Release %q failed: %s", targetRelease.Name, err.Error()))
			r.cfg.recordRelease(currentRelease)
			r.cfg.recordRelease(targetRelease)
			return targetRelease, fmt.Errorf("release %s failed: %w", targetRelease.Name, err)
		}
	}

	// post-rollback hooks
	if !r.DisableHooks {
		if err := r.cfg.execHook(targetRelease, release.HookPostRollback, r.WaitStrategy, r.WaitOptions, r.Timeout, serverSideApply); err != nil {
			return targetRelease, err
		}
	}

	deployed, err := r.cfg.Releases.DeployedAll(currentRelease.Name)
	if err != nil && !errors.Is(err, driver.ErrNoDeployedReleases) {
		return nil, err
	}
	// Supersede all previous deployments, see issue #2941.
	for _, reli := range deployed {
		rel, err := releaserToV1Release(reli)
		if err != nil {
			return nil, err
		}
		r.cfg.Logger().Debug("superseding previous deployment", "version", rel.Version)
		rel.Info.Status = common.StatusSuperseded
		r.cfg.recordRelease(rel)
	}

	targetRelease.Info.Status = common.StatusDeployed

	return targetRelease, nil
}
