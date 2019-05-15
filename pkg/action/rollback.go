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

	"helm.sh/helm/pkg/hooks"
	"helm.sh/helm/pkg/release"
)

// Rollback is the action for rolling back to a given release.
//
// It provides the implementation of 'helm rollback'.
type Rollback struct {
	cfg *Configuration

	Version      int
	Timeout      time.Duration
	Wait         bool
	DisableHooks bool
	DryRun       bool
	Recreate     bool // will (if true) recreate pods after a rollback.
	Force        bool // will (if true) force resource upgrade through uninstall/recreate if needed
}

// NewRollback creates a new Rollback object with the given configuration.
func NewRollback(cfg *Configuration) *Rollback {
	return &Rollback{
		cfg: cfg,
	}
}

// Run executes 'helm rollback' against the given release.
func (r *Rollback) Run(name string) (*release.Release, error) {
	r.cfg.Log("preparing rollback of %s", name)
	currentRelease, targetRelease, err := r.prepareRollback(name)
	if err != nil {
		return nil, err
	}

	if !r.DryRun {
		r.cfg.Log("creating rolled back release for %s", name)
		if err := r.cfg.Releases.Create(targetRelease); err != nil {
			return nil, err
		}
	}
	r.cfg.Log("performing rollback of %s", name)
	res, err := r.performRollback(currentRelease, targetRelease)
	if err != nil {
		return res, err
	}

	if !r.DryRun {
		r.cfg.Log("updating status for rolled back release for %s", name)
		if err := r.cfg.Releases.Update(targetRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

// prepareRollback finds the previous release and prepares a new release object with
// the previous release's configuration
func (r *Rollback) prepareRollback(name string) (*release.Release, *release.Release, error) {
	if err := validateReleaseName(name); err != nil {
		return nil, nil, errors.Errorf("prepareRollback: Release name is invalid: %s", name)
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

	r.cfg.Log("rolling back %s (current: v%d, target: v%d)", name, currentRelease.Version, previousVersion)

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
			LastDeployed:  time.Now(),
			Status:        release.StatusPendingRollback,
			Notes:         previousRelease.Info.Notes,
			// Because we lose the reference to previous version elsewhere, we set the
			// message here, and only override it later if we experience failure.
			Description: fmt.Sprintf("Rollback to %d", previousVersion),
		},
		Version:  currentRelease.Version + 1,
		Manifest: previousRelease.Manifest,
		Hooks:    previousRelease.Hooks,
	}

	return currentRelease, targetRelease, nil
}

func (r *Rollback) performRollback(currentRelease, targetRelease *release.Release) (*release.Release, error) {

	if r.DryRun {
		r.cfg.Log("dry run for %s", targetRelease.Name)
		return targetRelease, nil
	}

	// pre-rollback hooks
	if !r.DisableHooks {
		if err := r.execHook(targetRelease.Hooks, hooks.PreRollback); err != nil {
			return targetRelease, err
		}
	} else {
		r.cfg.Log("rollback hooks disabled for %s", targetRelease.Name)
	}

	cr := bytes.NewBufferString(currentRelease.Manifest)
	tr := bytes.NewBufferString(targetRelease.Manifest)
	// TODO add wait
	if err := r.cfg.KubeClient.Update(cr, tr, r.Force, r.Recreate); err != nil {
		msg := fmt.Sprintf("Rollback %q failed: %s", targetRelease.Name, err)
		r.cfg.Log("warning: %s", msg)
		currentRelease.Info.Status = release.StatusSuperseded
		targetRelease.Info.Status = release.StatusFailed
		targetRelease.Info.Description = msg
		r.cfg.recordRelease(currentRelease)
		r.cfg.recordRelease(targetRelease)
		return targetRelease, err
	}

	// post-rollback hooks
	if !r.DisableHooks {
		if err := r.execHook(targetRelease.Hooks, hooks.PostRollback); err != nil {
			return targetRelease, err
		}
	}

	deployed, err := r.cfg.Releases.DeployedAll(currentRelease.Name)
	if err != nil {
		return nil, err
	}
	// Supersede all previous deployments, see issue #2941.
	for _, rel := range deployed {
		r.cfg.Log("superseding previous deployment %d", rel.Version)
		rel.Info.Status = release.StatusSuperseded
		r.cfg.recordRelease(rel)
	}

	targetRelease.Info.Status = release.StatusDeployed

	return targetRelease, nil
}

// execHook executes all of the hooks for the given hook event.
func (r *Rollback) execHook(hs []*release.Hook, hook string) error {
	timeout := r.Timeout
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
		if err := deleteHookByPolicy(r.cfg, h, hooks.BeforeHookCreation); err != nil {
			return err
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := r.cfg.KubeClient.Create(b); err != nil {
			return errors.Wrapf(err, "warning: Hook %s %s failed", hook, h.Path)
		}
		b.Reset()
		b.WriteString(h.Manifest)

		if err := r.cfg.KubeClient.WatchUntilReady(b, timeout); err != nil {
			// If a hook is failed, checkout the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := deleteHookByPolicy(r.cfg, h, hooks.HookFailed); err != nil {
				return err
			}
			return err
		}
	}

	// If all hooks are succeeded, checkout the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := deleteHookByPolicy(r.cfg, h, hooks.HookSucceeded); err != nil {
			return err
		}
		h.LastRun = time.Now()
	}

	return nil
}

// deleteHookByPolicy deletes a hook if the hook policy instructs it to
func deleteHookByPolicy(cfg *Configuration, h *release.Hook, policy string) error {
	if hookHasDeletePolicy(h, policy) {
		b := bytes.NewBufferString(h.Manifest)
		return cfg.KubeClient.Delete(b)
	}
	return nil
}
