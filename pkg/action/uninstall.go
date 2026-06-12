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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	releasei "helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// Uninstall is the action for uninstalling releases.
//
// It provides the implementation of 'helm uninstall'.
type Uninstall struct {
	cfg *Configuration

	DisableHooks        bool
	DryRun              bool
	IgnoreNotFound      bool
	KeepHistory         bool
	WaitStrategy        kube.WaitStrategy
	WaitOptions         []kube.WaitOption
	DeletionPropagation string
	Timeout             time.Duration
	Description         string
}

// NewUninstall creates a new Uninstall object with the given configuration.
func NewUninstall(cfg *Configuration) *Uninstall {
	return &Uninstall{
		cfg: cfg,
	}
}

// Run uninstalls the given release.
func (u *Uninstall) Run(name string) (*releasei.UninstallReleaseResponse, error) {
	if err := u.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	var waiter kube.Waiter
	var err error
	if c, supportsOptions := u.cfg.KubeClient.(kube.InterfaceWaitOptions); supportsOptions {
		waiter, err = c.GetWaiterWithOptions(u.WaitStrategy, u.WaitOptions...)
	} else {
		waiter, err = u.cfg.KubeClient.GetWaiter(u.WaitStrategy)
	}
	if err != nil {
		return nil, err
	}

	if u.DryRun {
		ri, err := u.cfg.releaseContent(name, 0)

		if err != nil {
			if u.IgnoreNotFound && errors.Is(err, driver.ErrReleaseNotFound) {
				return nil, nil
			}
			return &releasei.UninstallReleaseResponse{}, err
		}
		r, err := releaserToV1Release(ri)
		if err != nil {
			return nil, err
		}

		// Verify ownership in dry-run mode to show what would actually be deleted
		manifests := releaseutil.SplitManifests(r.Manifest)
		_, files, err := releaseutil.SortManifests(manifests, nil, releaseutil.UninstallOrder)
		if err == nil {
			filesToKeep, filesToDelete := filterManifestsToKeep(files)

			var builder strings.Builder
			for _, file := range filesToDelete {
				builder.WriteString("\n---\n" + file.Content)
			}

			resources, err := u.cfg.KubeClient.Build(strings.NewReader(builder.String()), false)
			if err == nil && len(resources) > 0 {
				ownedResources, unownedResources, unverifiableResources, err := verifyOwnershipBeforeDelete(resources, r.Name, r.Namespace)
				if err == nil {
					if len(unownedResources) > 0 {
						u.cfg.Logger().Warn("dry-run: resources would be skipped because they are not owned by this release",
							"release", r.Name,
							"count", len(unownedResources))
						for _, info := range unownedResources {
							u.cfg.Logger().Warn("dry-run: would skip resource",
								"kind", info.Mapping.GroupVersionKind.Kind,
								"name", info.Name,
								"namespace", info.Namespace)
						}
					}

					if len(unverifiableResources) > 0 {
						u.cfg.Logger().Warn("dry-run: resources would be skipped because their ownership could not be verified",
							"release", r.Name,
							"count", len(unverifiableResources))
						for _, ur := range unverifiableResources {
							u.cfg.Logger().Warn("dry-run: would skip resource (ownership could not be verified)",
								"kind", ur.Info.Mapping.GroupVersionKind.Kind,
								"name", ur.Info.Name,
								"namespace", ur.Info.Namespace,
								"error", ur.Err)
						}
					}

					if len(ownedResources) > 0 {
						u.cfg.Logger().Debug("dry-run: resources would be deleted",
							"release", r.Name,
							"count", len(ownedResources))
						for _, info := range ownedResources {
							u.cfg.Logger().Debug("dry-run: would delete resource",
								"kind", info.Mapping.GroupVersionKind.Kind,
								"name", info.Name,
								"namespace", info.Namespace)
						}
					}
				}
			}

			// Include kept resources in dry-run info
			if len(filesToKeep) > 0 {
				var kept strings.Builder
				kept.WriteString("These resources were kept due to the resource policy:\n")
				for _, f := range filesToKeep {
					fmt.Fprintf(&kept, "[%s] %s\n", f.Head.Kind, f.Head.Metadata.Name)
				}
				res := &releasei.UninstallReleaseResponse{Release: r, Info: kept.String()}
				return res, nil
			}
		}

		return &releasei.UninstallReleaseResponse{Release: r}, nil
	}

	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, fmt.Errorf("uninstall: Release name is invalid: %s", name)
	}

	relsi, err := u.cfg.Releases.History(name)
	if err != nil {
		if u.IgnoreNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("uninstall: Release not loaded: %s: %w", name, err)
	}
	if len(relsi) < 1 {
		return nil, errMissingRelease
	}

	rels, err := releaseListToV1List(relsi)
	if err != nil {
		return nil, err
	}

	releaseutil.SortByRevision(rels)
	rel := rels[len(rels)-1]

	// TODO: Are there any cases where we want to force a delete even if it's
	// already marked deleted?
	if rel.Info.Status == releasecommon.StatusUninstalled {
		if !u.KeepHistory {
			if err := u.purgeReleases(rels...); err != nil {
				return nil, fmt.Errorf("uninstall: Failed to purge the release: %w", err)
			}
			return &releasei.UninstallReleaseResponse{Release: rel}, nil
		}
		return nil, fmt.Errorf("the release named %q is already deleted", name)
	}

	gatingWaiter := waiter
	if rel.IsSequenced() && u.WaitStrategy == kube.HookOnlyStrategy {
		u.cfg.Logger().Info("release was installed with ordered sequencing; using the status watcher to gate deletion batches (hooks keep the hook-only strategy)", "release", rel.Name)
		gatingWaiter, err = getWaiterFor(u.cfg.KubeClient, kube.StatusWatcherStrategy, u.WaitOptions...)
		if err != nil {
			return nil, err
		}
	}

	u.cfg.Logger().Debug("uninstall: deleting release", "name", name)
	rel.Info.Status = releasecommon.StatusUninstalling
	rel.Info.Deleted = time.Now()
	rel.Info.Description = "Deletion in progress (or silently failed)"
	res := &releasei.UninstallReleaseResponse{Release: rel}

	if !u.DisableHooks {
		serverSideApply := true
		if err := u.cfg.execHook(rel, release.HookPreDelete, u.WaitStrategy, u.WaitOptions, u.Timeout, serverSideApply); err != nil {
			return res, err
		}
	} else {
		u.cfg.Logger().Debug("delete hooks disabled", "release", name)
	}

	// From here on out, the release is currently considered to be in StatusUninstalling
	// state.
	if err := u.cfg.Releases.Update(rel); err != nil {
		u.cfg.Logger().Debug("uninstall: Failed to store updated release", slog.Any("error", err))
	}

	deletedResources, kept, errs := u.deleteRelease(rel, gatingWaiter)
	if errs != nil {
		u.cfg.Logger().Debug("uninstall: Failed to delete release", slog.Any("error", errs))
		return nil, fmt.Errorf("failed to delete release: %s", name)
	}

	res.Info = kept

	if !rel.IsSequenced() {
		if err := waiter.WaitForDelete(deletedResources, u.Timeout); err != nil {
			errs = append(errs, err)
		}
	}

	if !u.DisableHooks {
		serverSideApply := true
		if err := u.cfg.execHook(rel, release.HookPostDelete, u.WaitStrategy, u.WaitOptions, u.Timeout, serverSideApply); err != nil {
			errs = append(errs, err)
		}
	}

	rel.Info.Status = releasecommon.StatusUninstalled
	if u.Description != "" {
		rel.Info.Description = u.Description
	} else {
		rel.Info.Description = "Uninstallation complete"
	}

	if !u.KeepHistory {
		u.cfg.Logger().Debug("purge requested", "release", name)
		err := u.purgeReleases(rels...)
		if err != nil {
			errs = append(errs, fmt.Errorf("uninstall: Failed to purge the release: %w", err))
		}

		// Return the errors that occurred while deleting the release, if any
		if len(errs) > 0 {
			return res, fmt.Errorf("uninstallation completed with %d error(s): %w", len(errs), joinErrors(errs, "; "))
		}

		return res, nil
	}

	if err := u.cfg.Releases.Update(rel); err != nil {
		u.cfg.Logger().Debug("uninstall: Failed to store updated release", slog.Any("error", err))
	}

	// Supersede all previous deployments, see issue #12556 (which is a
	// variation on #2941).
	deployed, err := u.cfg.Releases.DeployedAll(name)
	if err != nil && !errors.Is(err, driver.ErrNoDeployedReleases) {
		return nil, err
	}
	for _, reli := range deployed {
		rel, err := releaserToV1Release(reli)
		if err != nil {
			return nil, err
		}

		u.cfg.Logger().Debug("superseding previous deployment", "version", rel.Version)
		rel.Info.Status = releasecommon.StatusSuperseded
		if err := u.cfg.Releases.Update(rel); err != nil {
			u.cfg.Logger().Debug("uninstall: Failed to store updated release", slog.Any("error", err))
		}
	}

	if len(errs) > 0 {
		return res, fmt.Errorf("uninstallation completed with %d error(s): %w", len(errs), joinErrors(errs, "; "))
	}
	return res, nil
}

func (u *Uninstall) purgeReleases(rels ...*release.Release) error {
	for _, rel := range rels {
		if _, err := u.cfg.Releases.Delete(rel.Name, rel.Version); err != nil {
			return err
		}
	}
	return nil
}

type joinedErrors struct {
	errs []error
	sep  string
}

func joinErrors(errs []error, sep string) error {
	return &joinedErrors{
		errs: errs,
		sep:  sep,
	}
}

func (e *joinedErrors) Error() string {
	errs := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, e.sep)
}

func (e *joinedErrors) Unwrap() []error {
	return e.errs
}

// deleteRelease deletes the release and returns list of delete resources and manifests that were kept in the deletion process.
func (u *Uninstall) deleteRelease(rel *release.Release, waiter kube.Waiter) (kube.ResourceList, string, []error) {
	if rel.IsSequenced() {
		return u.deleteReleaseSequenced(rel, waiter)
	}

	var errs []error

	manifests := releaseutil.SplitManifests(rel.Manifest)
	_, files, err := releaseutil.SortManifests(manifests, nil, releaseutil.UninstallOrder)
	if err != nil {
		// We could instead just delete everything in no particular order.
		// FIXME: One way to delete at this point would be to try a label-based
		// deletion. The problem with this is that we could get a false positive
		// and delete something that was not legitimately part of this release.
		return nil, rel.Manifest, []error{fmt.Errorf("corrupted release record. You must manually delete the resources: %w", err)}
	}

	filesToKeep, filesToDelete := filterManifestsToKeep(files)
	var kept strings.Builder
	if len(filesToKeep) > 0 {
		kept.WriteString("These resources were kept due to the resource policy:\n")
		for _, f := range filesToKeep {
			fmt.Fprintf(&kept, "[%s] %s\n", f.Head.Kind, f.Head.Metadata.Name)
		}
	}

	resources, err := u.buildDeleteResources(filesToDelete)
	if err != nil {
		return nil, "", []error{err}
	}

	ownedResources, skipped, err := u.verifyOwnedForDelete(resources, rel.Name, rel.Namespace)
	if err != nil {
		return nil, "", []error{err}
	}
	if skipped != "" {
		if kept.Len() > 0 {
			kept.WriteString("\n")
		}
		kept.WriteString(skipped)
	}

	// Delete only owned resources
	if len(ownedResources) > 0 {
		_, errs = u.cfg.KubeClient.Delete(ownedResources, parseCascadingFlag(u.DeletionPropagation, u.cfg.Logger()))
	}
	return ownedResources, kept.String(), errs
}

// deleteReleaseSequenced deletes a sequenced release's resources in exact
// reverse deployment order. The forward plan is rebuilt from the STORED
// manifest via its "# Source:" comments (no chart re-render), so it covers
// every stored resource, including vendored subcharts absent from Chart.yaml
// dependencies, which the DAG-walking deleter used to leak (bead i42).
// Keep-policy filtering applies to the parsed stream before the plan is built,
// so kept resources are never part of the plan.
func (u *Uninstall) deleteReleaseSequenced(rel *release.Release, waiter kube.Waiter) (kube.ResourceList, string, []error) {
	manifests, err := sequence.ParseStoredManifests(rel.Manifest)
	if err != nil {
		return nil, rel.Manifest, []error{fmt.Errorf("corrupted release record. You must manually delete the resources: %w", err)}
	}

	filesToKeep, filesToDelete := filterManifestsToKeep(manifests)
	var kept strings.Builder
	if len(filesToKeep) > 0 {
		kept.WriteString("These resources were kept due to the resource policy:\n")
		for _, f := range filesToKeep {
			fmt.Fprintf(&kept, "[%s] %s\n", f.Head.Kind, f.Head.Metadata.Name)
		}
	}

	plan, err := sequence.Build(rel.Chart, filesToDelete)
	if err != nil {
		return nil, kept.String(), []error{fmt.Errorf("building sequencing plan for uninstall: %w", err)}
	}
	logPlanWarnings(u.cfg.Logger(), plan)

	deadline := computeDeadline(u.Timeout)
	var allDeleted kube.ResourceList
	for _, batch := range plan.Reverse().Batches {
		deleted, errs := u.deleteManifestBatch(batch.Manifests(), waiter, deadline, rel.Name, rel.Namespace)
		allDeleted = append(allDeleted, deleted...)
		if len(errs) > 0 {
			return allDeleted, kept.String(), errs
		}
	}
	return allDeleted, kept.String(), nil
}

// verifyOwnedForDelete verifies which of the built resources are owned by the
// release. It logs (and thereby skips) resources that are not owned by the
// release or whose ownership cannot be verified, and returns the owned subset
// together with a human-readable summary of what was skipped (empty when nothing
// was skipped). It performs no deletion; callers delete the returned owned
// resources. Both the sequenced and non-sequenced uninstall paths route through
// it so neither deletes resources that do not belong to the release.
func (u *Uninstall) verifyOwnedForDelete(resources kube.ResourceList, releaseName, releaseNamespace string) (kube.ResourceList, string, error) {
	if len(resources) == 0 {
		return nil, "", nil
	}

	ownedResources, unownedResources, unverifiableResources, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
	if err != nil {
		return nil, "", fmt.Errorf("unable to verify resource ownership: %w", err)
	}

	var skipped strings.Builder

	// Log and report resources that are not owned by this release.
	if len(unownedResources) > 0 {
		for _, info := range unownedResources {
			u.cfg.Logger().Warn("skipping delete of resource not owned by this release",
				"kind", info.Mapping.GroupVersionKind.Kind,
				"name", info.Name,
				"namespace", info.Namespace,
				"release", releaseName)
		}
		fmt.Fprintf(&skipped, "%d resource(s) were not deleted because they are not owned by this release:\n", len(unownedResources))
		for _, info := range unownedResources {
			fmt.Fprintf(&skipped, "[%s] %s\n", info.Mapping.GroupVersionKind.Kind, info.Name)
		}
	}

	// Log and report resources whose ownership could not be verified.
	if len(unverifiableResources) > 0 {
		for _, ur := range unverifiableResources {
			u.cfg.Logger().Warn("skipping delete of resource because ownership could not be verified",
				"kind", ur.Info.Mapping.GroupVersionKind.Kind,
				"name", ur.Info.Name,
				"namespace", ur.Info.Namespace,
				"release", releaseName,
				"error", ur.Err)
		}
		if skipped.Len() > 0 {
			skipped.WriteString("\n")
		}
		fmt.Fprintf(&skipped, "%d resource(s) were not deleted because their ownership could not be verified:\n", len(unverifiableResources))
		for _, ur := range unverifiableResources {
			fmt.Fprintf(&skipped, "[%s] %s: %s\n", ur.Info.Mapping.GroupVersionKind.Kind, ur.Info.Name, ur.Err)
		}
	}

	for _, info := range ownedResources {
		u.cfg.Logger().Debug("deleting resource owned by this release",
			"kind", info.Mapping.GroupVersionKind.Kind,
			"name", info.Name,
			"namespace", info.Namespace,
			"release", releaseName)
	}

	return ownedResources, skipped.String(), nil
}

func (u *Uninstall) buildDeleteResources(manifests []releaseutil.Manifest) (kube.ResourceList, error) {
	var builder strings.Builder
	for _, file := range manifests {
		builder.WriteString("\n---\n" + file.Content)
	}

	resources, err := u.cfg.KubeClient.Build(strings.NewReader(builder.String()), false)
	if err != nil {
		return nil, fmt.Errorf("unable to build kubernetes objects for delete: %w", err)
	}
	return resources, nil
}

func (u *Uninstall) deleteManifestBatch(manifests []releaseutil.Manifest, waiter kube.Waiter, deadline time.Time, releaseName, releaseNamespace string) (kube.ResourceList, []error) {
	resources, err := u.buildDeleteResources(manifests)
	if err != nil || len(resources) == 0 {
		if err != nil {
			return nil, []error{err}
		}
		return nil, nil
	}

	// Verify ownership before deleting so a sequenced uninstall, like the
	// non-sequenced path, never deletes resources it does not own.
	ownedResources, _, err := u.verifyOwnedForDelete(resources, releaseName, releaseNamespace)
	if err != nil {
		return nil, []error{err}
	}
	if len(ownedResources) == 0 {
		return nil, nil
	}

	_, errs := u.cfg.KubeClient.Delete(ownedResources, parseCascadingFlag(u.DeletionPropagation, u.cfg.Logger()))
	if len(errs) > 0 {
		return ownedResources, errs
	}

	timeout := u.Timeout
	if !deadline.IsZero() {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if err := waiter.WaitForDelete(ownedResources, timeout); err != nil {
		errs = append(errs, err)
	}
	return ownedResources, errs
}

func parseCascadingFlag(cascadingFlag string, logger *slog.Logger) v1.DeletionPropagation {
	switch cascadingFlag {
	case "orphan":
		return v1.DeletePropagationOrphan
	case "foreground":
		return v1.DeletePropagationForeground
	case "background":
		return v1.DeletePropagationBackground
	default:
		logger.Debug("uninstall: given cascade value, defaulting to delete propagation background", "value", cascadingFlag)
		return v1.DeletePropagationBackground
	}
}
