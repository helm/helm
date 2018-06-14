/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package helm

import (
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/chart"
	"k8s.io/helm/pkg/hapi/release"
)

// Interface for helm client for mocking in tests
type Interface interface {
	ListReleases(opts ...ReleaseListOption) ([]*release.Release, error)
	InstallRelease(chStr, namespace string, opts ...InstallOption) (*release.Release, error)
	InstallReleaseFromChart(chart *chart.Chart, namespace string, opts ...InstallOption) (*release.Release, error)
	UninstallRelease(rlsName string, opts ...UninstallOption) (*hapi.UninstallReleaseResponse, error)
	ReleaseStatus(rlsName string, version int) (*hapi.GetReleaseStatusResponse, error)
	UpdateRelease(rlsName, chStr string, opts ...UpdateOption) (*release.Release, error)
	UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...UpdateOption) (*release.Release, error)
	RollbackRelease(rlsName string, opts ...RollbackOption) (*release.Release, error)
	ReleaseContent(rlsName string, version int) (*release.Release, error)
	ReleaseHistory(rlsName string, max int) ([]*release.Release, error)
	RunReleaseTest(rlsName string, opts ...ReleaseTestOption) (<-chan *hapi.TestReleaseResponse, <-chan error)
}
