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

package release

// ReleaseStatus is the status of a release
type ReleaseStatus string

// Describe the status of a release
const (
	// StatusUnknown indicates that a release is in an uncertain state.
	StatusUnknown ReleaseStatus = "unknown"
	// StatusDeployed indicates that the release has been pushed to Kubernetes.
	StatusDeployed ReleaseStatus = "deployed"
	// StatusUninstalled indicates that a release has been uninstalled from Kubermetes.
	StatusUninstalled ReleaseStatus = "uninstalled"
	// StatusSuperseded indicates that this release object is outdated and a newer one exists.
	StatusSuperseded ReleaseStatus = "superseded"
	// StatusFailed indicates that the release was not successfully deployed.
	StatusFailed ReleaseStatus = "failed"
	// StatusUninstalling indicates that a uninstall operation is underway.
	StatusUninstalling ReleaseStatus = "uninstalling"
	// StatusPendingInstall indicates that an install operation is underway.
	StatusPendingInstall ReleaseStatus = "pending-install"
	// StatusPendingUpgrade indicates that an upgrade operation is underway.
	StatusPendingUpgrade ReleaseStatus = "pending-upgrade"
	// StatusPendingRollback indicates that an rollback operation is underway.
	StatusPendingRollback ReleaseStatus = "pending-rollback"
)

func (x ReleaseStatus) String() string { return string(x) }
