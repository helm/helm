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

	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
)

// Status is the action for checking the deployment status of releases.
//
// It provides the implementation of 'helm status'.
type Status struct {
	cfg *Configuration

	Version int

	// If true, display description to output format,
	// only affect print type table.
	// TODO Helm 4: Remove this flag and output the description by default.
	ShowDescription bool

	// ShowResources sets if the resources should be retrieved with the status.
	// TODO Helm 4: Remove this flag and output the resources by default.
	ShowResources bool

	// ShowResourcesTable is used with ShowResources. When true this will cause
	// the resulting objects to be retrieved as a kind=table.
	ShowResourcesTable bool
}

// NewStatus creates a new Status object with the given configuration.
func NewStatus(cfg *Configuration) *Status {
	return &Status{
		cfg: cfg,
	}
}

// Run executes 'helm status' against the given release.
func (s *Status) Run(name string) (*release.Release, error) {
	if err := s.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	if !s.ShowResources {
		return s.cfg.releaseContent(name, s.Version)
	}

	rel, err := s.cfg.releaseContent(name, s.Version)
	if err != nil {
		return nil, err
	}

	if kubeClient, ok := s.cfg.KubeClient.(kube.InterfaceResources); ok {
		var resources kube.ResourceList
		if s.ShowResourcesTable {
			resources, err = kubeClient.BuildTable(bytes.NewBufferString(rel.Manifest), false)
			if err != nil {
				return nil, err
			}
		} else {
			resources, err = s.cfg.KubeClient.Build(bytes.NewBufferString(rel.Manifest), false)
			if err != nil {
				return nil, err
			}
		}

		resp, err := kubeClient.Get(resources, true)
		if err != nil {
			return nil, err
		}

		rel.Info.Resources = resp

		return rel, nil
	}
	return nil, errors.New("unable to get kubeClient with interface InterfaceResources")
}
