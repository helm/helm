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

import "testing"

func TestListStates(t *testing.T) {
	for input, expect := range map[string]ListStates{
		"deployed":            ListDeployed,
		"uninstalled":         ListUninstalled,
		"uninstalling":        ListUninstalling,
		"superseded":          ListSuperseded,
		"failed":              ListFailed,
		"pending-install":     ListPendingInstall,
		"pending-rollback":    ListPendingRollback,
		"pending-upgrade":     ListPendingUpgrade,
		"unknown":             ListUnknown,
		"totally made up key": ListUnknown,
	} {
		if expect != expect.FromName(input) {
			t.Errorf("Expected %d for %s", expect, input)
		}
		// This is a cheap way to verify that ListAll actually allows everything but Unknown
		if got := expect.FromName(input); got != ListUnknown && got&ListAll == 0 {
			t.Errorf("Expected %s to match the ListAll filter", input)
		}
	}

	filter := ListDeployed | ListPendingRollback
	if status := filter.FromName("deployed"); filter&status == 0 {
		t.Errorf("Expected %d to match mask %d", status, filter)
	}
	if status := filter.FromName("failed"); filter&status != 0 {
		t.Errorf("Expected %d to fail to match mask %d", status, filter)
	}
}
