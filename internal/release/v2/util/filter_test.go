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

package util // import "helm.sh/helm/v4/internal/release/v2/util"

import (
	"testing"

	"github.com/stretchr/testify/require"

	rspb "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/release/common"
)

func TestFilterAny(t *testing.T) {
	ls := Any(StatusFilter(common.StatusUninstalled)).Filter(releases)
	require.Len(t, ls, 2)

	r0, r1 := ls[0], ls[1]
	require.Equal(t, common.StatusUninstalled, r0.Info.Status)
	require.Equal(t, common.StatusUninstalled, r1.Info.Status)
}

func TestFilterAll(t *testing.T) {
	fn := FilterFunc(func(rls *rspb.Release) bool {
		// true if not uninstalled and version < 4
		v0 := !StatusFilter(common.StatusUninstalled).Check(rls)
		v1 := rls.Version < 4
		return v0 && v1
	})

	ls := All(fn).Filter(releases)
	require.Len(t, ls, 1)

	r0 := ls[0]
	require.NotEqual(t, 4, r0.Version, "got release with status revision 4")
	require.NotEqual(t, common.StatusUninstalled, r0.Info.Status, "got release with status UNINSTALLED")
}
