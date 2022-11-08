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

package kube // import "helm.sh/helm/v3/pkg/kube"

import (
	"errors"
	"testing"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
)

func Test_isServiceUnavailable(t *testing.T) {
	tests := []struct {
		err    error
		expect bool
	}{
		{err: nil, expect: false},
		{err: errors.New("random error from somewhere"), expect: false},
		{err: rpctypes.ErrGRPCLeaderChanged, expect: true},
		{err: errors.New("etcdserver: leader changed"), expect: true},
	}

	for _, tt := range tests {
		if isServiceUnavailable(tt.err) != tt.expect {
			t.Errorf("failed test for %q (expect equal: %t)", tt.err, tt.expect)
		}
	}
}
