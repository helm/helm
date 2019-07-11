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
	"sort"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/hooks"
	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/releaseutil"
)

// Upgrade is the action for upgrading releases.
//
// It provides the implementation of 'helm upgrade'.
type Upgrade struct {
	cfg *Configuration

	ChartPathOptions
	ValueOptions

	Install      bool
	Devel        bool
	Namespace    string
	Timeout      time.Duration
	Wait         bool
	DisableHooks bool
	DryRun       bool
	Force        bool
	ResetValues  bool
	ReuseValues  bool
	// Recreate will (if true) recreate pods after a rollback.
	Recreate bool
	// MaxHistory limits the maximum number of revisions saved per release
	MaxHistory int
	Atomic     bool
}

// NewUpgrade creates a new Upgrade object with the given configuration.
func NewUpgrade(cfg *Configuration) *Upgrade {
	return &Upgrade{
		cfg: cfg,
	}
}

// Run executes the upgrade on the given release.
func (u *Upgrade) Run(name string, chart *chart.Chart) (*release.Release, error) {
	if err := chartutil.ProcessDependencies(chart, u.rawValues); err != nil {
		return nil, err
	}

	// Make sure if Atomic is set, that wait is set as well. This makes it so
	// the user doesn't have to specify both
	u.Wait = u.Wait || u.Atomic

	if err := validateReleaseName(name); err != nil {
		return nil, errors.Errorf("upgradeRelease: Release name is invalid: %s", name)
	}
	u.cfg.Log("preparing upgrade for %s", name)
	currentRelease, upgradedRelease, err := u.prepareUpgrade(name, chart)
	if err != nil {
		return nil, err
	}

	u.cfg.Releases.MaxHistory = u.MaxHistory

	if !u.DryRun {
		u.cfg.Log("creating upgraded release for %s", name)
		if err := u.cfg.Releases.Create(upgradedRelease); err != nil {
			return nil, err
		}
	}

	u.cfg.Log("performing update for %s", name)
	res, err := u.performUpgrade(currentRelease, upgradedRelease)
	if err != nil {
		return res, err
	}

	if !u.DryRun {
		u.cfg.Log("updating status for upgraded release for %s", name)
		if err := u.cfg.Releases.Update(upgradedRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

func validateReleaseName(releaseName string) error {
	if releaseName == "" {
		return errMissingRelease
	}

	if !ValidName.MatchString(releaseName) || (len(releaseName) > releaseNameMaxLen) {
		return errInvalidName
	}

	return nil
}

// prepareUpgrade builds an upgraded release for an upgrade operation.
func (u *Upgrade) prepareUpgrade(name string, chart *chart.Chart) (*release.Release, *release.Release, error) {
	if chart == nil {
		return nil, nil, errMissingChart
	}

	// finds the deployed release with the given name
	currentRelease, err := u.cfg.Releases.Deployed(name)
	if err != nil {
		return nil, nil, err
	}

	// determine if values will be reused
	if err := u.reuseValues(chart, currentRelease); err != nil {
		return nil, nil, err
	}

	// finds the non-deleted release with the given name
	lastRelease, err := u.cfg.Releases.Last(name)
	if err != nil {
		return nil, nil, err
	}

	// Increment revision count. This is passed to templates, and also stored on
	// the release object.
	revision := lastRelease.Version + 1

	options := chartutil.ReleaseOptions{
		Name:      name,
		Namespace: currentRelease.Namespace,
		IsUpgrade: true,
	}

	caps, err := u.cfg.getCapabilities()
	if err != nil {
		return nil, nil, err
	}
	valuesToRender, err := chartutil.ToRenderValues(chart, u.rawValues, options, caps)
	if err != nil {
		return nil, nil, err
	}

	hooks, manifestDoc, notesTxt, err := u.cfg.renderResources(chart, valuesToRender, "")
	if err != nil {
		return nil, nil, err
	}

	// Store an upgraded release.
	upgradedRelease := &release.Release{
		Name:      name,
		Namespace: currentRelease.Namespace,
		Chart:     chart,
		Config:    u.rawValues,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  Timestamper(),
			Status:        release.StatusPendingUpgrade,
			Description:   "Preparing upgrade", // This should be overwritten later.
		},
		Version:  revision,
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
	}

	if len(notesTxt) > 0 {
		upgradedRelease.Info.Notes = notesTxt
	}
	err = validateManifest(u.cfg.KubeClient, manifestDoc.Bytes())
	return currentRelease, upgradedRelease, err
}

func (u *Upgrade) performUpgrade(originalRelease, upgradedRelease *release.Release) (*release.Release, error) {
	if u.DryRun {
		u.cfg.Log("dry run for %s", upgradedRelease.Name)
		upgradedRelease.Info.Description = "Dry run complete"
		return upgradedRelease, nil
	}

	// pre-upgrade hooks
	if !u.DisableHooks {
		if err := u.execHook(upgradedRelease.Hooks, hooks.PreUpgrade); err != nil {
			return u.failRelease(upgradedRelease, fmt.Errorf("pre-upgrade hooks failed: %s", err))
		}
	} else {
		u.cfg.Log("upgrade hooks disabled for %s", upgradedRelease.Name)
	}
	if err := u.upgradeRelease(originalRelease, upgradedRelease); err != nil {
		u.cfg.recordRelease(originalRelease)
		return u.failRelease(upgradedRelease, err)
	}

	if u.Wait {
		buf := bytes.NewBufferString(upgradedRelease.Manifest)
		if err := u.cfg.KubeClient.Wait(buf, u.Timeout); err != nil {
			u.cfg.recordRelease(originalRelease)
			return u.failRelease(upgradedRelease, err)
		}
	}

	// post-upgrade hooks
	if !u.DisableHooks {
		if err := u.execHook(upgradedRelease.Hooks, hooks.PostUpgrade); err != nil {
			return u.failRelease(upgradedRelease, fmt.Errorf("post-upgrade hooks failed: %s", err))
		}
	}

	originalRelease.Info.Status = release.StatusSuperseded
	u.cfg.recordRelease(originalRelease)

	upgradedRelease.Info.Status = release.StatusDeployed
	upgradedRelease.Info.Description = "Upgrade complete"

	return upgradedRelease, nil
}

func (u *Upgrade) failRelease(rel *release.Release, err error) (*release.Release, error) {
	msg := fmt.Sprintf("Upgrade %q failed: %s", rel.Name, err)
	u.cfg.Log("warning: %s", msg)

	rel.Info.Status = release.StatusFailed
	rel.Info.Description = msg
	u.cfg.recordRelease(rel)
	if u.Atomic {
		u.cfg.Log("Upgrade failed and atomic is set, rolling back to last successful release")

		// As a protection, get the last successful release before rollback.
		// If there are no successful releases, bail out
		hist := NewHistory(u.cfg)
		fullHistory, herr := hist.Run(rel.Name)
		if herr != nil {
			return rel, errors.Wrapf(herr, "an error occurred while finding last successful release. original upgrade error: %s", err)
		}

		// There isn't a way to tell if a previous release was successful, but
		// generally failed releases do not get superseded unless the next
		// release is successful, so this should be relatively safe
		filteredHistory := releaseutil.FilterFunc(func(r *release.Release) bool {
			return r.Info.Status == release.StatusSuperseded || r.Info.Status == release.StatusDeployed
		}).Filter(fullHistory)
		if len(filteredHistory) == 0 {
			return rel, errors.Wrap(err, "unable to find a previously successful release when attempting to rollback. original upgrade error")
		}

		releaseutil.Reverse(filteredHistory, releaseutil.SortByRevision)

		rollin := NewRollback(u.cfg)
		rollin.Version = filteredHistory[0].Version
		rollin.Wait = true
		rollin.DisableHooks = u.DisableHooks
		rollin.Recreate = u.Recreate
		rollin.Force = u.Force
		rollin.Timeout = u.Timeout
		if _, rollErr := rollin.Run(rel.Name); rollErr != nil {
			return rel, errors.Wrapf(rollErr, "an error occurred while rolling back the release. original upgrade error: %s", err)
		}
		return rel, errors.Wrapf(err, "release %s failed, and has been rolled back due to atomic being set", rel.Name)
	}

	return rel, err
}

// upgradeRelease performs an upgrade from current to target release
func (u *Upgrade) upgradeRelease(current, target *release.Release) error {
	cm := bytes.NewBufferString(current.Manifest)
	tm := bytes.NewBufferString(target.Manifest)
	// TODO add wait
	return u.cfg.KubeClient.Update(cm, tm, u.Force, u.Recreate)
}

// reuseValues copies values from the current release to a new release if the
// new release does not have any values.
//
// If the request already has values, or if there are no values in the current
// release, this does nothing.
//
// This is skipped if the u.ResetValues flag is set, in which case the
// request values are not altered.
func (u *Upgrade) reuseValues(chart *chart.Chart, current *release.Release) error {
	if u.ResetValues {
		// If ResetValues is set, we comletely ignore current.Config.
		u.cfg.Log("resetting values to the chart's original version")
		return nil
	}

	// If the ReuseValues flag is set, we always copy the old values over the new config's values.
	if u.ReuseValues {
		u.cfg.Log("reusing the old release's values")

		// We have to regenerate the old coalesced values:
		oldVals, err := chartutil.CoalesceValues(current.Chart, current.Config)
		if err != nil {
			return errors.Wrap(err, "failed to rebuild old values")
		}

		u.rawValues = chartutil.CoalesceTables(current.Config, u.rawValues)

		chart.Values = oldVals

		return nil
	}

	if len(u.rawValues) == 0 && len(current.Config) > 0 {
		u.cfg.Log("copying values from %s (v%d) to new release.", current.Name, current.Version)
		u.rawValues = current.Config
	}
	return nil
}

func validateManifest(c kube.Interface, manifest []byte) error {
	_, err := c.Build(bytes.NewReader(manifest))
	return err
}

// execHook executes all of the hooks for the given hook event.
func (u *Upgrade) execHook(hs []*release.Hook, hook string) error {
	timeout := u.Timeout
	executingHooks := []*release.Hook{}

	for _, h := range hs {
		for _, e := range h.Events {
			if string(e) == hook {
				executingHooks = append(executingHooks, h)
			}
		}
	}

	sort.Sort(hookByWeight(executingHooks))
	for _, h := range executingHooks {
		if err := deleteHookByPolicy(u.cfg, h, hooks.BeforeHookCreation); err != nil {
			return err
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := u.cfg.KubeClient.Create(b); err != nil {
			return errors.Wrapf(err, "warning: Hook %s %s failed", hook, h.Path)
		}
		b.Reset()
		b.WriteString(h.Manifest)

		if err := u.cfg.KubeClient.WatchUntilReady(b, timeout); err != nil {
			// If a hook is failed, checkout the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := deleteHookByPolicy(u.cfg, h, hooks.HookFailed); err != nil {
				return err
			}
			return err
		}
	}

	// If all hooks are succeeded, checkout the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := deleteHookByPolicy(u.cfg, h, hooks.HookSucceeded); err != nil {
			return err
		}
		h.LastRun = time.Now()
	}

	return nil
}
