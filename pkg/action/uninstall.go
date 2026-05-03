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

	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	renderutil "helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	releasei "helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
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

	deletedResources, kept, errs := u.deleteRelease(rel, waiter)
	if errs != nil {
		u.cfg.Logger().Debug("uninstall: Failed to delete release", slog.Any("error", errs))
		return nil, fmt.Errorf("failed to delete release: %s", name)
	}

	if kept != "" {
		kept = "These resources were kept due to the resource policy:\n" + kept
	}
	res.Info = kept

	if !isSequencedRelease(rel) {
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
	if len(u.Description) > 0 {
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

func isSequencedRelease(rel *release.Release) bool {
	return rel.SequencingInfo != nil && rel.SequencingInfo.Enabled
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
	for _, f := range filesToKeep {
		fmt.Fprintf(&kept, "[%s] %s\n", f.Head.Kind, f.Head.Metadata.Name)
	}

	if isSequencedRelease(rel) {
		filesToDelete = u.recoverManifestPathsForSequencedUninstall(rel, filesToDelete)
		deleted, errs := u.sequencedDeleteManifests(rel.Chart, filesToDelete, waiter)
		return deleted, kept.String(), errs
	}

	resources, err := u.buildDeleteResources(filesToDelete)
	if err != nil {
		return nil, "", []error{err}
	}
	if len(resources) > 0 {
		_, errs := u.cfg.KubeClient.Delete(resources, parseCascadingFlag(u.DeletionPropagation))
		return resources, kept.String(), errs
	}
	return resources, kept.String(), nil
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

func (u *Uninstall) deleteManifestBatch(manifests []releaseutil.Manifest, waiter kube.Waiter, deadline time.Time) (kube.ResourceList, []error) {
	resources, err := u.buildDeleteResources(manifests)
	if err != nil || len(resources) == 0 {
		if err != nil {
			return nil, []error{err}
		}
		return nil, nil
	}

	_, errs := u.cfg.KubeClient.Delete(resources, parseCascadingFlag(u.DeletionPropagation))
	if len(errs) > 0 {
		return resources, errs
	}

	timeout := u.Timeout
	if !deadline.IsZero() {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if err := waiter.WaitForDelete(resources, timeout); err != nil {
		errs = append(errs, err)
	}
	return resources, errs
}

func (u *Uninstall) sequencedDeleteManifests(chrt *chart.Chart, manifests []releaseutil.Manifest, waiter kube.Waiter) (kube.ResourceList, []error) {
	deadline := computeDeadline(u.Timeout)
	if chrt == nil {
		return u.deleteResourceGroupBatchesReverse(manifests, waiter, deadline)
	}
	return u.deleteChartLevelReverse(chrt, manifests, waiter, deadline)
}

func (u *Uninstall) deleteChartLevelReverse(chrt *chart.Chart, manifests []releaseutil.Manifest, waiter kube.Waiter, deadline time.Time) (kube.ResourceList, []error) {
	grouped := GroupManifestsByDirectSubchart(manifests, chrt.Name())

	allDeleted, errs := u.deleteResourceGroupBatchesReverse(grouped[""], waiter, deadline)
	if len(errs) > 0 {
		return allDeleted, errs
	}

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return allDeleted, []error{fmt.Errorf("building subchart DAG for sequenced uninstall: %w", err)}
	}

	batches, err := dag.GetBatches()
	if err != nil {
		return allDeleted, []error{fmt.Errorf("getting subchart batches for sequenced uninstall: %w", err)}
	}

	for i, j := 0, len(batches)-1; i < j; i, j = i+1, j-1 {
		batches[i], batches[j] = batches[j], batches[i]
	}

	for _, batch := range batches {
		for _, subchartName := range batch {
			subchartManifests := grouped[subchartName]
			if len(subchartManifests) == 0 {
				continue
			}

			subchart := findSubchart(chrt, subchartName)
			var deleted kube.ResourceList
			if subchart == nil {
				u.cfg.Logger().Warn("subchart not found in chart dependencies during sequenced uninstall; falling back to flat resource-group deletion", "subchart", subchartName)
				deleted, errs = u.deleteResourceGroupBatchesReverse(subchartManifests, waiter, deadline)
			} else {
				deleted, errs = u.deleteChartLevelReverse(subchart, subchartManifests, waiter, deadline)
			}
			allDeleted = append(allDeleted, deleted...)
			if len(errs) > 0 {
				return allDeleted, errs
			}
		}
	}

	return allDeleted, nil
}

func (u *Uninstall) deleteResourceGroupBatchesReverse(manifests []releaseutil.Manifest, waiter kube.Waiter, deadline time.Time) (kube.ResourceList, []error) {
	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return nil, []error{fmt.Errorf("parsing resource-group annotations for sequenced uninstall: %w", err)}
	}
	for _, warning := range warnings {
		u.cfg.Logger().Warn("resource-group annotation warning during uninstall", "warning", warning)
	}

	var allDeleted kube.ResourceList

	if len(result.Unsequenced) > 0 {
		deleted, errs := u.deleteManifestBatch(result.Unsequenced, waiter, deadline)
		allDeleted = append(allDeleted, deleted...)
		if len(errs) > 0 {
			return allDeleted, errs
		}
	}

	if len(result.Groups) == 0 {
		return allDeleted, nil
	}

	dag, err := releaseutil.BuildResourceGroupDAG(result)
	if err != nil {
		return allDeleted, []error{fmt.Errorf("building resource-group DAG for sequenced uninstall: %w", err)}
	}
	batches, err := dag.GetBatches()
	if err != nil {
		return allDeleted, []error{fmt.Errorf("getting resource-group batches for sequenced uninstall: %w", err)}
	}

	for i, j := 0, len(batches)-1; i < j; i, j = i+1, j-1 {
		batches[i], batches[j] = batches[j], batches[i]
	}

	for _, batch := range batches {
		var batchManifests []releaseutil.Manifest
		for _, groupName := range batch {
			batchManifests = append(batchManifests, result.Groups[groupName]...)
		}
		deleted, errs := u.deleteManifestBatch(batchManifests, waiter, deadline)
		allDeleted = append(allDeleted, deleted...)
		if len(errs) > 0 {
			return allDeleted, errs
		}
	}

	return allDeleted, nil
}

func (u *Uninstall) recoverManifestPathsForSequencedUninstall(rel *release.Release, manifests []releaseutil.Manifest) []releaseutil.Manifest {
	rendered, err := renderReleaseManifestsWithPaths(u.cfg, rel)
	if err != nil {
		u.cfg.Logger().Warn("unable to recover rendered chart paths for sequenced uninstall; falling back to stored manifest order", slog.Any("error", err))
		return manifests
	}
	return applyRenderedManifestPaths(manifests, rendered)
}

// renderReleaseManifestsWithPaths re-renders the release chart+config to produce
// manifests keyed by source template paths (e.g., "mychart/templates/deploy.yaml").
func renderReleaseManifestsWithPaths(cfg *Configuration, rel *release.Release) ([]releaseutil.Manifest, error) {
	if rel.Chart == nil {
		return nil, errors.New("release chart is missing")
	}
	if err := chartutil.ProcessDependencies(rel.Chart, rel.Config); err != nil {
		return nil, fmt.Errorf("processing chart dependencies: %w", err)
	}

	caps, err := cfg.getCapabilities()
	if err != nil {
		return nil, fmt.Errorf("getting capabilities: %w", err)
	}

	valuesToRender, err := renderutil.ToRenderValuesWithSchemaValidation(
		rel.Chart,
		rel.Config,
		chartcommon.ReleaseOptions{
			Name:      rel.Name,
			Namespace: rel.Namespace,
			Revision:  rel.Version,
			IsInstall: rel.Version <= 1,
			IsUpgrade: rel.Version > 1,
		},
		caps,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("building render values: %w", err)
	}

	_, _, _, manifests, err := cfg.renderResourcesWithFiles(rel.Chart, valuesToRender, rel.Name, "", false, false, false, nil, false, false, false, PostRenderStrategyCombined)
	if err != nil {
		return nil, fmt.Errorf("rendering chart manifests: %w", err)
	}
	return manifests, nil
}

func applyRenderedManifestPaths(stored, rendered []releaseutil.Manifest) []releaseutil.Manifest {
	byContent := make(map[string][]string, len(rendered))
	byIdentity := make(map[string][]string, len(rendered))
	for _, manifest := range rendered {
		contentKey := normalizedManifestContent(manifest.Content)
		byContent[contentKey] = append(byContent[contentKey], manifest.Name)

		identityKey := manifestIdentity(manifest)
		byIdentity[identityKey] = append(byIdentity[identityKey], manifest.Name)
	}

	out := make([]releaseutil.Manifest, len(stored))
	copy(out, stored)

	for i, manifest := range out {
		contentKey := normalizedManifestContent(manifest.Content)
		if names := byContent[contentKey]; len(names) > 0 {
			out[i].Name = names[0]
			byContent[contentKey] = names[1:]
			continue
		}

		identityKey := manifestIdentity(manifest)
		if names := byIdentity[identityKey]; len(names) > 0 {
			out[i].Name = names[0]
			byIdentity[identityKey] = names[1:]
		}
	}

	return out
}

func manifestIdentity(manifest releaseutil.Manifest) string {
	if manifest.Head == nil || manifest.Head.Metadata == nil {
		return normalizedManifestContent(manifest.Content)
	}
	return fmt.Sprintf("%s/%s/%s/%s", manifest.Head.Version, manifest.Head.Kind, manifest.Head.Metadata.Namespace, manifest.Head.Metadata.Name)
}

func normalizedManifestContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	start := 0
	for start < len(lines) {
		line := strings.TrimSpace(lines[start])
		if line == "" || strings.HasPrefix(line, "#") {
			start++
			continue
		}
		break
	}

	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

func parseCascadingFlag(cascadingFlag string) v1.DeletionPropagation {
	switch cascadingFlag {
	case "orphan":
		return v1.DeletePropagationOrphan
	case "foreground":
		return v1.DeletePropagationForeground
	case "background":
		return v1.DeletePropagationBackground
	default:
		slog.Debug("uninstall: given cascade value, defaulting to delete propagation background", "value", cascadingFlag)
		return v1.DeletePropagationBackground
	}
}
