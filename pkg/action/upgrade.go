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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/postrender"
	"helm.sh/helm/v4/pkg/registry"
	releaseutil "helm.sh/helm/v4/pkg/release/util"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// Upgrade is the action for upgrading releases.
//
// It provides the implementation of 'helm upgrade'.
type Upgrade struct {
	cfg *Configuration

	ChartPathOptions

	// Install is a purely informative flag that indicates whether this upgrade was done in "install" mode.
	//
	// Applications may use this to determine whether this Upgrade operation was done as part of a
	// pure upgrade (Upgrade.Install == false) or as part of an install-or-upgrade operation
	// (Upgrade.Install == true).
	//
	// Setting this to `true` will NOT cause `Upgrade` to perform an install if the release does not exist.
	// That process must be handled by creating an Install action directly. See cmd/upgrade.go for an
	// example of how this flag is used.
	Install bool
	// Devel indicates that the operation is done in devel mode.
	Devel bool
	// Namespace is the namespace in which this operation should be performed.
	Namespace string
	// SkipCRDs skips installing CRDs when install flag is enabled during upgrade
	SkipCRDs bool
	// Timeout is the timeout for this operation
	Timeout time.Duration
	// WaitStrategy determines what type of waiting should be done
	WaitStrategy kube.WaitStrategy
	// WaitForJobs determines whether the wait operation for the Jobs should be performed after the upgrade is requested.
	WaitForJobs bool
	// DisableHooks disables hook processing if set to true.
	DisableHooks bool
	// DryRun controls whether the operation is prepared, but not executed.
	DryRun bool
	// DryRunOption controls whether the operation is prepared, but not executed with options on whether or not to interact with the remote cluster.
	DryRunOption string
	// HideSecret can be set to true when DryRun is enabled in order to hide
	// Kubernetes Secrets in the output. It cannot be used outside of DryRun.
	HideSecret bool
	// Force will, if set to `true`, ignore certain warnings and perform the upgrade anyway.
	//
	// This should be used with caution.
	Force bool
	// ResetValues will reset the values to the chart's built-ins rather than merging with existing.
	ResetValues bool
	// ReuseValues will reuse the user's last supplied values.
	ReuseValues bool
	// ResetThenReuseValues will reset the values to the chart's built-ins then merge with user's last supplied values.
	ResetThenReuseValues bool
	// Recreate will (if true) recreate pods after a rollback.
	Recreate bool
	// MaxHistory limits the maximum number of revisions saved per release
	MaxHistory int
	// Atomic, if true, will roll back on failure.
	Atomic bool
	// CleanupOnFail will, if true, cause the upgrade to delete newly-created resources on a failed update.
	CleanupOnFail bool
	// SubNotes determines whether sub-notes are rendered in the chart.
	SubNotes bool
	// HideNotes determines whether notes are output during upgrade
	HideNotes bool
	// SkipSchemaValidation determines if JSON schema validation is disabled.
	SkipSchemaValidation bool
	// Description is the description of this operation
	Description string
	Labels      map[string]string
	// PostRenderer is an optional post-renderer
	//
	// If this is non-nil, then after templates are rendered, they will be sent to the
	// post renderer before sending to the Kubernetes API server.
	PostRenderer postrender.PostRenderer
	// DisableOpenAPIValidation controls whether OpenAPI validation is enforced.
	DisableOpenAPIValidation bool
	// Get missing dependencies
	DependencyUpdate bool
	// Lock to control raceconditions when the process receives a SIGTERM
	Lock sync.Mutex
	// Enable DNS lookups when rendering templates
	EnableDNS bool
	// TakeOwnership will skip the check for helm annotations and adopt all existing resources.
	TakeOwnership bool
}

type resultMessage struct {
	r *release.Release
	e error
}

// NewUpgrade creates a new Upgrade object with the given configuration.
func NewUpgrade(cfg *Configuration) *Upgrade {
	up := &Upgrade{
		cfg: cfg,
	}
	up.registryClient = cfg.RegistryClient

	return up
}

// SetRegistryClient sets the registry client to use when fetching charts.
func (u *Upgrade) SetRegistryClient(client *registry.Client) {
	u.registryClient = client
}

// Run executes the upgrade on the given release.
func (u *Upgrade) Run(name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	ctx := context.Background()
	return u.RunWithContext(ctx, name, chart, vals)
}

// RunWithContext executes the upgrade on the given release with context.
func (u *Upgrade) RunWithContext(ctx context.Context, name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	if err := u.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	// Make sure if Atomic is set, that wait is set as well. This makes it so
	// the user doesn't have to specify both
	if u.WaitStrategy == kube.HookOnlyStrategy && u.Atomic {
		u.WaitStrategy = kube.StatusWatcherStrategy
	}

	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, fmt.Errorf("release name is invalid: %s", name)
	}

	slog.Debug("preparing upgrade", "name", name)
	currentRelease, upgradedRelease, err := u.prepareUpgrade(name, chart, vals)
	if err != nil {
		return nil, err
	}

	u.cfg.Releases.MaxHistory = u.MaxHistory

	slog.Debug("performing update", "name", name)
	res, err := u.performUpgrade(ctx, currentRelease, upgradedRelease)
	if err != nil {
		return res, err
	}

	// Do not update for dry runs
	if !u.isDryRun() {
		slog.Debug("updating status for upgraded release", "name", name)
		if err := u.cfg.Releases.Update(upgradedRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

// isDryRun returns true if Upgrade is set to run as a DryRun
func (u *Upgrade) isDryRun() bool {
	if u.DryRun || u.DryRunOption == "client" || u.DryRunOption == "server" || u.DryRunOption == "true" {
		return true
	}
	return false
}

// prepareUpgrade builds an upgraded release for an upgrade operation.
func (u *Upgrade) prepareUpgrade(name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, *release.Release, error) {
	if chart == nil {
		return nil, nil, errMissingChart
	}

	// HideSecret must be used with dry run. Otherwise, return an error.
	if !u.isDryRun() && u.HideSecret {
		return nil, nil, errors.New("hiding Kubernetes secrets requires a dry-run mode")
	}

	// finds the last non-deleted release with the given name
	lastRelease, err := u.cfg.Releases.Last(name)
	if err != nil {
		// to keep existing behavior of returning the "%q has no deployed releases" error when an existing release does not exist
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, nil, driver.NewErrNoDeployedReleases(name)
		}
		return nil, nil, err
	}

	// Concurrent `helm upgrade`s will either fail here with `errPending` or when creating the release with "already exists". This should act as a pessimistic lock.
	if lastRelease.Info.Status.IsPending() {
		return nil, nil, errPending
	}

	var currentRelease *release.Release
	if lastRelease.Info.Status == release.StatusDeployed {
		// no need to retrieve the last deployed release from storage as the last release is deployed
		currentRelease = lastRelease
	} else {
		// finds the deployed release with the given name
		currentRelease, err = u.cfg.Releases.Deployed(name)
		if err != nil {
			if errors.Is(err, driver.ErrNoDeployedReleases) &&
				(lastRelease.Info.Status == release.StatusFailed || lastRelease.Info.Status == release.StatusSuperseded) {
				currentRelease = lastRelease
			} else {
				return nil, nil, err
			}
		}
	}

	// determine if values will be reused
	vals, err = u.reuseValues(chart, currentRelease, vals)
	if err != nil {
		return nil, nil, err
	}

	if err := chartutil.ProcessDependencies(chart, vals); err != nil {
		return nil, nil, err
	}

	// Increment revision count. This is passed to templates, and also stored on
	// the release object.
	revision := lastRelease.Version + 1

	options := chartutil.ReleaseOptions{
		Name:      name,
		Namespace: currentRelease.Namespace,
		Revision:  revision,
		IsUpgrade: true,
	}

	caps, err := u.cfg.getCapabilities()
	if err != nil {
		return nil, nil, err
	}
	valuesToRender, err := chartutil.ToRenderValuesWithSchemaValidation(chart, vals, options, caps, u.SkipSchemaValidation)
	if err != nil {
		return nil, nil, err
	}

	// Determine whether or not to interact with remote
	var interactWithRemote bool
	if !u.isDryRun() || u.DryRunOption == "server" || u.DryRunOption == "none" || u.DryRunOption == "false" {
		interactWithRemote = true
	}

	hooks, manifestDoc, notesTxt, err := u.cfg.renderResources(chart, valuesToRender, "", "", u.SubNotes, false, false, u.PostRenderer, interactWithRemote, u.EnableDNS, u.HideSecret)
	if err != nil {
		return nil, nil, err
	}

	if driver.ContainsSystemLabels(u.Labels) {
		return nil, nil, fmt.Errorf("user supplied labels contains system reserved label name. System labels: %+v", driver.GetSystemLabels())
	}

	// Store an upgraded release.
	upgradedRelease := &release.Release{
		Name:      name,
		Namespace: currentRelease.Namespace,
		Chart:     chart,
		Config:    vals,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  Timestamper(),
			Status:        release.StatusPendingUpgrade,
			Description:   "Preparing upgrade", // This should be overwritten later.
		},
		Version:  revision,
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
		Labels:   mergeCustomLabels(lastRelease.Labels, u.Labels),
	}

	if len(notesTxt) > 0 {
		upgradedRelease.Info.Notes = notesTxt
	}
	err = validateManifest(u.cfg.KubeClient, manifestDoc.Bytes(), !u.DisableOpenAPIValidation)
	return currentRelease, upgradedRelease, err
}

func (u *Upgrade) performUpgrade(ctx context.Context, originalRelease, upgradedRelease *release.Release) (*release.Release, error) {
	current, err := u.cfg.KubeClient.Build(bytes.NewBufferString(originalRelease.Manifest), false)
	if err != nil {
		// Checking for removed Kubernetes API error so can provide a more informative error message to the user
		// Ref: https://github.com/helm/helm/issues/7219
		if strings.Contains(err.Error(), "unable to recognize \"\": no matches for kind") {
			return upgradedRelease, fmt.Errorf("current release manifest contains removed kubernetes api(s) for this "+
				"kubernetes version and it is therefore unable to build the kubernetes "+
				"objects for performing the diff. error from kubernetes: %w", err)
		}
		return upgradedRelease, fmt.Errorf("unable to build kubernetes objects from current release manifest: %w", err)
	}
	target, err := u.cfg.KubeClient.Build(bytes.NewBufferString(upgradedRelease.Manifest), !u.DisableOpenAPIValidation)
	if err != nil {
		return upgradedRelease, fmt.Errorf("unable to build kubernetes objects from new release manifest: %w", err)
	}

	// It is safe to use force only on target because these are resources currently rendered by the chart.
	err = target.Visit(setMetadataVisitor(upgradedRelease.Name, upgradedRelease.Namespace, true))
	if err != nil {
		return upgradedRelease, err
	}

	// Do a basic diff using gvk + name to figure out what new resources are being created so we can validate they don't already exist
	existingResources := make(map[string]bool)
	for _, r := range current {
		existingResources[objectKey(r)] = true
	}

	var toBeCreated kube.ResourceList
	for _, r := range target {
		if !existingResources[objectKey(r)] {
			toBeCreated = append(toBeCreated, r)
		}
	}

	var toBeUpdated kube.ResourceList
	if u.TakeOwnership {
		toBeUpdated, err = requireAdoption(toBeCreated)
	} else {
		toBeUpdated, err = existingResourceConflict(toBeCreated, upgradedRelease.Name, upgradedRelease.Namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to continue with update: %w", err)
	}

	toBeUpdated.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		current.Append(r)
		return nil
	})

	// Run if it is a dry run
	if u.isDryRun() {
		slog.Debug("dry run for release", "name", upgradedRelease.Name)
		if len(u.Description) > 0 {
			upgradedRelease.Info.Description = u.Description
		} else {
			upgradedRelease.Info.Description = "Dry run complete"
		}
		return upgradedRelease, nil
	}

	slog.Debug("creating upgraded release", "name", upgradedRelease.Name)
	if err := u.cfg.Releases.Create(upgradedRelease); err != nil {
		return nil, err
	}
	rChan := make(chan resultMessage)
	ctxChan := make(chan resultMessage)
	doneChan := make(chan interface{})
	defer close(doneChan)
	go u.releasingUpgrade(rChan, upgradedRelease, current, target, originalRelease)
	go u.handleContext(ctx, doneChan, ctxChan, upgradedRelease)
	select {
	case result := <-rChan:
		return result.r, result.e
	case result := <-ctxChan:
		return result.r, result.e
	}
}

// Function used to lock the Mutex, this is important for the case when the atomic flag is set.
// In that case the upgrade will finish before the rollback is finished so it is necessary to wait for the rollback to finish.
// The rollback will be trigger by the function failRelease
func (u *Upgrade) reportToPerformUpgrade(c chan<- resultMessage, rel *release.Release, created kube.ResourceList, err error) {
	u.Lock.Lock()
	if err != nil {
		rel, err = u.failRelease(rel, created, err)
	}
	c <- resultMessage{r: rel, e: err}
	u.Lock.Unlock()
}

// Setup listener for SIGINT and SIGTERM
func (u *Upgrade) handleContext(ctx context.Context, done chan interface{}, c chan<- resultMessage, upgradedRelease *release.Release) {
	select {
	case <-ctx.Done():
		err := ctx.Err()

		// when the atomic flag is set the ongoing release finish first and doesn't give time for the rollback happens.
		u.reportToPerformUpgrade(c, upgradedRelease, kube.ResourceList{}, err)
	case <-done:
		return
	}
}
func (u *Upgrade) releasingUpgrade(c chan<- resultMessage, upgradedRelease *release.Release, current kube.ResourceList, target kube.ResourceList, originalRelease *release.Release) {
	// pre-upgrade hooks

	if !u.DisableHooks {
		if err := u.cfg.execHook(upgradedRelease, release.HookPreUpgrade, u.WaitStrategy, u.Timeout); err != nil {
			u.reportToPerformUpgrade(c, upgradedRelease, kube.ResourceList{}, fmt.Errorf("pre-upgrade hooks failed: %s", err))
			return
		}
	} else {
		slog.Debug("upgrade hooks disabled", "name", upgradedRelease.Name)
	}

	results, err := u.cfg.KubeClient.Update(current, target, u.Force)
	if err != nil {
		u.cfg.recordRelease(originalRelease)
		u.reportToPerformUpgrade(c, upgradedRelease, results.Created, err)
		return
	}

	if u.Recreate {
		// NOTE: Because this is not critical for a release to succeed, we just
		// log if an error occurs and continue onward. If we ever introduce log
		// levels, we should make these error level logs so users are notified
		// that they'll need to go do the cleanup on their own
		if err := recreate(u.cfg, results.Updated); err != nil {
			slog.Error(err.Error())
		}
	}
	waiter, err := u.cfg.KubeClient.GetWaiter(u.WaitStrategy)
	if err != nil {
		u.cfg.recordRelease(originalRelease)
		u.reportToPerformUpgrade(c, upgradedRelease, results.Created, err)
		return
	}
	if u.WaitForJobs {
		if err := waiter.WaitWithJobs(target, u.Timeout); err != nil {
			u.cfg.recordRelease(originalRelease)
			u.reportToPerformUpgrade(c, upgradedRelease, results.Created, err)
			return
		}
	} else {
		if err := waiter.Wait(target, u.Timeout); err != nil {
			u.cfg.recordRelease(originalRelease)
			u.reportToPerformUpgrade(c, upgradedRelease, results.Created, err)
			return
		}
	}

	// post-upgrade hooks
	if !u.DisableHooks {
		if err := u.cfg.execHook(upgradedRelease, release.HookPostUpgrade, u.WaitStrategy, u.Timeout); err != nil {
			u.reportToPerformUpgrade(c, upgradedRelease, results.Created, fmt.Errorf("post-upgrade hooks failed: %s", err))
			return
		}
	}

	originalRelease.Info.Status = release.StatusSuperseded
	u.cfg.recordRelease(originalRelease)

	upgradedRelease.Info.Status = release.StatusDeployed
	if len(u.Description) > 0 {
		upgradedRelease.Info.Description = u.Description
	} else {
		upgradedRelease.Info.Description = "Upgrade complete"
	}
	u.reportToPerformUpgrade(c, upgradedRelease, nil, nil)
}

func (u *Upgrade) failRelease(rel *release.Release, created kube.ResourceList, err error) (*release.Release, error) {
	msg := fmt.Sprintf("Upgrade %q failed: %s", rel.Name, err)
	slog.Warn("upgrade failed", "name", rel.Name, slog.Any("error", err))

	rel.Info.Status = release.StatusFailed
	rel.Info.Description = msg
	u.cfg.recordRelease(rel)
	if u.CleanupOnFail && len(created) > 0 {
		slog.Debug("cleanup on fail set", "cleaning_resources", len(created))
		_, errs := u.cfg.KubeClient.Delete(created)
		if errs != nil {
			return rel, fmt.Errorf(
				"an error occurred while cleaning up resources. original upgrade error: %w: %w",
				err,
				fmt.Errorf(
					"unable to cleanup resources: %w",
					joinErrors(errs, ", "),
				),
			)
		}
		slog.Debug("resource cleanup complete")
	}
	if u.Atomic {
		slog.Debug("upgrade failed and atomic is set, rolling back to last successful release")

		// As a protection, get the last successful release before rollback.
		// If there are no successful releases, bail out
		hist := NewHistory(u.cfg)
		fullHistory, herr := hist.Run(rel.Name)
		if herr != nil {
			return rel, fmt.Errorf("an error occurred while finding last successful release. original upgrade error: %w: %w", err, herr)
		}

		// There isn't a way to tell if a previous release was successful, but
		// generally failed releases do not get superseded unless the next
		// release is successful, so this should be relatively safe
		filteredHistory := releaseutil.FilterFunc(func(r *release.Release) bool {
			return r.Info.Status == release.StatusSuperseded || r.Info.Status == release.StatusDeployed
		}).Filter(fullHistory)
		if len(filteredHistory) == 0 {
			return rel, fmt.Errorf("unable to find a previously successful release when attempting to rollback. original upgrade error: %w", err)
		}

		releaseutil.Reverse(filteredHistory, releaseutil.SortByRevision)

		rollin := NewRollback(u.cfg)
		rollin.Version = filteredHistory[0].Version
		if u.WaitStrategy == kube.HookOnlyStrategy {
			rollin.WaitStrategy = kube.StatusWatcherStrategy
		}
		rollin.WaitForJobs = u.WaitForJobs
		rollin.DisableHooks = u.DisableHooks
		rollin.Recreate = u.Recreate
		rollin.Force = u.Force
		rollin.Timeout = u.Timeout
		if rollErr := rollin.Run(rel.Name); rollErr != nil {
			return rel, fmt.Errorf("an error occurred while rolling back the release. original upgrade error: %w: %w", err, rollErr)
		}
		return rel, fmt.Errorf("release %s failed, and has been rolled back due to atomic being set: %w", rel.Name, err)
	}

	return rel, err
}

// reuseValues copies values from the current release to a new release if the
// new release does not have any values.
//
// If the request already has values, or if there are no values in the current
// release, this does nothing.
//
// This is skipped if the u.ResetValues flag is set, in which case the
// request values are not altered.
func (u *Upgrade) reuseValues(chart *chart.Chart, current *release.Release, newVals map[string]interface{}) (map[string]interface{}, error) {
	if u.ResetValues {
		// If ResetValues is set, we completely ignore current.Config.
		slog.Debug("resetting values to the chart's original version")
		return newVals, nil
	}

	// If the ReuseValues flag is set, we always copy the old values over the new config's values.
	if u.ReuseValues {
		slog.Debug("reusing the old release's values")

		// We have to regenerate the old coalesced values:
		oldVals, err := chartutil.CoalesceValues(current.Chart, current.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to rebuild old values: %w", err)
		}

		newVals = chartutil.CoalesceTables(newVals, current.Config)

		chart.Values = oldVals

		return newVals, nil
	}

	// If the ResetThenReuseValues flag is set, we use the new chart's values, but we copy the old config's values over the new config's values.
	if u.ResetThenReuseValues {
		slog.Debug("merging values from old release to new values")

		newVals = chartutil.CoalesceTables(newVals, current.Config)

		return newVals, nil
	}

	if len(newVals) == 0 && len(current.Config) > 0 {
		slog.Debug("copying values from old release", "name", current.Name, "version", current.Version)
		newVals = current.Config
	}
	return newVals, nil
}

func validateManifest(c kube.Interface, manifest []byte, openAPIValidation bool) error {
	_, err := c.Build(bytes.NewReader(manifest), openAPIValidation)
	return err
}

// recreate captures all the logic for recreating pods for both upgrade and
// rollback. If we end up refactoring rollback to use upgrade, this can just be
// made an unexported method on the upgrade action.
func recreate(cfg *Configuration, resources kube.ResourceList) error {
	for _, res := range resources {
		versioned := kube.AsVersioned(res)
		selector, err := kube.SelectorsForObject(versioned)
		if err != nil {
			// If no selector is returned, it means this object is
			// definitely not a pod, so continue onward
			continue
		}

		client, err := cfg.KubernetesClientSet()
		if err != nil {
			return fmt.Errorf("unable to recreate pods for object %s/%s because an error occurred: %w", res.Namespace, res.Name, err)
		}

		pods, err := client.CoreV1().Pods(res.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return fmt.Errorf("unable to recreate pods for object %s/%s because an error occurred: %w", res.Namespace, res.Name, err)
		}

		// Restart pods
		for _, pod := range pods.Items {
			// Delete each pod for get them restarted with changed spec.
			if err := client.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, *metav1.NewPreconditionDeleteOptions(string(pod.UID))); err != nil {
				return fmt.Errorf("unable to recreate pods for object %s/%s because an error occurred: %w", res.Namespace, res.Name, err)
			}
		}
	}
	return nil
}

func objectKey(r *resource.Info) string {
	gvk := r.Object.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s/%s/%s/%s", gvk.GroupVersion().String(), gvk.Kind, r.Namespace, r.Name)
}

func mergeCustomLabels(current, desired map[string]string) map[string]string {
	labels := mergeStrStrMaps(current, desired)
	for k, v := range labels {
		if v == "null" {
			delete(labels, k)
		}
	}
	return labels
}
