/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

package tiller

import (
	"bytes"

	"k8s.io/helm/pkg/proto/hapi/release"
	rudderAPI "k8s.io/helm/pkg/proto/hapi/rudder"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/rudder"
	"k8s.io/helm/pkg/tiller/environment"
)

// ReleaseModule is an interface that allows ReleaseServer to run operations on release via either local implementation or Rudder service
type ReleaseModule interface {
	Create(r *release.Release, req *services.InstallReleaseRequest, env *environment.Environment) error
	Update(current, target *release.Release, req *services.UpdateReleaseRequest, env *environment.Environment) error
	Rollback(current, target *release.Release, req *services.RollbackReleaseRequest, env *environment.Environment) error
}

// LocalReleaseModule is a local implementation of ReleaseModule
type LocalReleaseModule struct{}

// Create creates a release via kubeclient from provided environment
func (m *LocalReleaseModule) Create(r *release.Release, req *services.InstallReleaseRequest, env *environment.Environment) error {
	b := bytes.NewBufferString(r.Manifest)
	return env.KubeClient.Create(r.Namespace, b, req.Timeout, req.Wait)
}

func (m *LocalReleaseModule) Update(current, target *release.Release, req *services.UpdateReleaseRequest, env *environment.Environment) error {
	c := bytes.NewBufferString(current.Manifest)
	t := bytes.NewBufferString(target.Manifest)
	return env.KubeClient.Update(target.Namespace, c, t, req.Recreate, req.Timeout, req.Wait)
}

func (m *LocalReleaseModule) Rollback(current, target *release.Release, req *services.RollbackReleaseRequest, env *environment.Environment) error {
	c := bytes.NewBufferString(current.Manifest)
	t := bytes.NewBufferString(target.Manifest)
	return env.KubeClient.Update(target.Namespace, c, t, req.Recreate, req.Timeout, req.Wait)
}

// RemoteReleaseModule is a ReleaseModule which calls Rudder service to operate on a release
type RemoteReleaseModule struct{}

// Create calls rudder.InstallRelease
func (m *RemoteReleaseModule) Create(r *release.Release, req *services.InstallReleaseRequest, env *environment.Environment) error {
	request := &rudderAPI.InstallReleaseRequest{Release: r}
	_, err := rudder.InstallRelease(request)
	return err
}

// Update calls rudder.UpgradeRelease
func (m *RemoteReleaseModule) Update(current, target *release.Release, req *services.UpdateReleaseRequest, env *environment.Environment) error {
	upgrade := &rudderAPI.UpgradeReleaseRequest{
		Current:  current,
		Target:   target,
		Recreate: req.Recreate,
		Timeout:  req.Timeout,
		Wait:     req.Wait,
	}
	_, err := rudder.UpgradeRelease(upgrade)
	return err
}

// Rollback calls rudder.Rollback
func (m *RemoteReleaseModule) Rollback(current, target *release.Release, req *services.RollbackReleaseRequest, env *environment.Environment) error {
	rollback := &rudderAPI.RollbackReleaseRequest{
		Current:  current,
		Target:   target,
		Recreate: req.Recreate,
		Timeout:  req.Timeout,
		Wait:     req.Wait,
	}
	_, err := rudder.RollbackRelease(rollback)
	return err
}
