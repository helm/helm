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
	"strings"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	helmtime "helm.sh/helm/v3/pkg/time"
)

// Uninstall is the action for uninstalling releases.
//
// It provides the implementation of 'helm uninstall'.
type Uninstall struct {
	cfg *Configuration

	DisableHooks bool
	DryRun       bool
	KeepHistory  bool
	Timeout      time.Duration
	Description  string
}

// NewUninstall creates a new Uninstall object with the given configuration.
func NewUninstall(cfg *Configuration) *Uninstall {
	return &Uninstall{
		cfg: cfg,
	}
}

// Run uninstalls the given release.
func (u *Uninstall) Run(name string) (*release.UninstallReleaseResponse, error) {
	if err := u.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	if u.DryRun {
		// In the dry run case, just see if the release exists
		r, err := u.cfg.releaseContent(name, 0)
		if err != nil {
			return &release.UninstallReleaseResponse{}, err
		}
		return &release.UninstallReleaseResponse{Release: r}, nil
	}

	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, errors.Errorf("uninstall: Release name is invalid: %s", name)
	}

	rels, err := u.cfg.Releases.History(name)
	if err != nil {
		return nil, errors.Wrapf(err, "uninstall: Release not loaded: %s", name)
	}
	if len(rels) < 1 {
		return nil, errMissingRelease
	}

	releaseutil.SortByRevision(rels)
	rel := rels[len(rels)-1]

	// TODO: Are there any cases where we want to force a delete even if it's
	// already marked deleted?
	if rel.Info.Status == release.StatusUninstalled {
		if !u.KeepHistory {
			if err := u.purgeReleases(rels...); err != nil {
				return nil, errors.Wrap(err, "uninstall: Failed to purge the release")
			}
			return &release.UninstallReleaseResponse{Release: rel}, nil
		}
		return nil, errors.Errorf("the release named %q is already deleted", name)
	}

	u.cfg.Log("uninstall: Deleting %s", name)
	rel.Info.Status = release.StatusUninstalling
	rel.Info.Deleted = helmtime.Now()
	rel.Info.Description = "Deletion in progress (or silently failed)"
	res := &release.UninstallReleaseResponse{Release: rel}

	if !u.DisableHooks {
		if err := u.cfg.execHook(rel, release.HookPreDelete, u.Timeout); err != nil {
			return res, err
		}
	} else {
		u.cfg.Log("delete hooks disabled for %s", name)
	}

	// From here on out, the release is currently considered to be in StatusUninstalling
	// state.
	if err := u.cfg.Releases.Update(rel); err != nil {
		u.cfg.Log("uninstall: Failed to store updated release: %s", err)
	}

	kept, errs := u.deleteRelease(rel)

	if kept != "" {
		kept = "These resources were kept due to the resource policy:\n" + kept
	}
	res.Info = kept

	if !u.DisableHooks {
		if err := u.cfg.execHook(rel, release.HookPostDelete, u.Timeout); err != nil {
			errs = append(errs, err)
		}
	}

	rel.Info.Status = release.StatusUninstalled
	if len(u.Description) > 0 {
		rel.Info.Description = u.Description
	} else {
		rel.Info.Description = "Uninstallation complete"
	}

	if !u.KeepHistory {
		u.cfg.Log("purge requested for %s", name)
		err := u.purgeReleases(rels...)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "uninstall: Failed to purge the release"))
		}

		// Return the errors that occurred while deleting the release, if any
		if len(errs) > 0 {
			return res, errors.Errorf("uninstallation completed with %d error(s): %s", len(errs), joinErrors(errs))
		}

		return res, nil
	}

	if err := u.cfg.Releases.Update(rel); err != nil {
		u.cfg.Log("uninstall: Failed to store updated release: %s", err)
	}

	if len(errs) > 0 {
		return res, errors.Errorf("uninstallation completed with %d error(s): %s", len(errs), joinErrors(errs))
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

func joinErrors(errs []error) string {
	es := make([]string, 0, len(errs))
	for _, e := range errs {
		es = append(es, e.Error())
	}
	return strings.Join(es, "; ")
}

// deleteRelease deletes the release and returns manifests that were kept in the deletion process
func (u *Uninstall) deleteRelease(rel *release.Release) (string, []error) {
	var errs []error
	caps, err := u.cfg.getCapabilities()
	if err != nil {
		return rel.Manifest, []error{errors.Wrap(err, "could not get apiVersions from Kubernetes")}
	}

	manifests := releaseutil.SplitManifests(rel.Manifest)
	_, files, err := releaseutil.SortManifests(manifests, caps.APIVersions, releaseutil.UninstallOrder)
	if err != nil {
		// We could instead just delete everything in no particular order.
		// FIXME: One way to delete at this point would be to try a label-based
		// deletion. The problem with this is that we could get a false positive
		// and delete something that was not legitimately part of this release.
		return rel.Manifest, []error{errors.Wrap(err, "corrupted release record. You must manually delete the resources")}
	}

	filesToKeep, filesToDelete := filterManifestsToKeep(files)
	var kept string
	for _, f := range filesToKeep {
		kept += "[" + f.Head.Kind + "] " + f.Head.Metadata.Name + "\n"
	}

	var builder strings.Builder
	for _, file := range filesToDelete {
		builder.WriteString("\n---\n" + file.Content)
	}

	resources, err := u.cfg.KubeClient.Build(strings.NewReader(builder.String()), false)
	if err != nil {
		return "", []error{errors.Wrap(err, "unable to build kubernetes objects for delete")}
	}
	if len(resources) > 0 {
		_, errs = u.cfg.KubeClient.Delete(resources)
	}
	return kept, errs
}
