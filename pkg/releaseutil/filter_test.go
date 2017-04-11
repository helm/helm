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

package releaseutil // import "k8s.io/helm/pkg/releaseutil"

import (
	"testing"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestFilterAny(t *testing.T) {
	ls := Any(StatusFilter(rspb.Status_DELETED)).Filter(releases)
	if len(ls) != 2 {
		t.Fatalf("expected 2 results, got '%d'", len(ls))
	}

	r0, r1 := ls[0], ls[1]
	switch {
	case r0.Info.Status.Code != rspb.Status_DELETED:
		t.Fatalf("expected DELETED result, got '%s'", r1.Info.Status.Code)
	case r1.Info.Status.Code != rspb.Status_DELETED:
		t.Fatalf("expected DELETED result, got '%s'", r1.Info.Status.Code)
	}
}

func TestFilterAll(t *testing.T) {
	fn := FilterFunc(func(rls *rspb.Release) bool {
		// true if not deleted and version < 4
		v0 := !StatusFilter(rspb.Status_DELETED).Check(rls)
		v1 := rls.Version < 4
		return v0 && v1
	})

	ls := All(fn).Filter(releases)
	if len(ls) != 1 {
		t.Fatalf("expected 1 result, got '%d'", len(ls))
	}

	switch r0 := ls[0]; {
	case r0.Version == 4:
		t.Fatal("got release with status revision 4")
	case r0.Info.Status.Code == rspb.Status_DELETED:
		t.Fatal("got release with status DELTED")
	}
}
