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

package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// setListPageSize overrides listPageSize for the duration of the test.
func setListPageSize(t *testing.T, size int64) {
	t.Helper()
	old := listPageSize
	listPageSize = size
	t.Cleanup(func() { listPageSize = old })
}

func TestGetSystemLabel(t *testing.T) {
	output := GetSystemLabels()
	assert.Equalf(t, systemLabels, output, "Expected {%v}, got {%v}", systemLabels, output)
}

func TestIsSystemLabel(t *testing.T) {
	tests := map[string]bool{
		"name":  true,
		"owner": true,
		"test":  false,
		"NaMe":  false,
	}
	for label, result := range tests {
		output := isSystemLabel(label)
		assert.Equalf(t, output, result, "Output %t not equal to expected %t", output, result)
	}
}

func TestFilterSystemLabels(t *testing.T) {
	tests := [][2]map[string]string{
		{nil, map[string]string{}},
		{map[string]string{}, map[string]string{}},
		{map[string]string{
			"name":       "name",
			"owner":      "owner",
			"status":     "status",
			"version":    "version",
			"createdAt":  "createdAt",
			"modifiedAt": "modifiedAt",
		}, map[string]string{}},
		{map[string]string{
			"StaTus": "status",
			"name":   "name",
			"owner":  "owner",
			"key":    "value",
		}, map[string]string{
			"StaTus": "status",
			"key":    "value",
		}},
		{map[string]string{
			"key1": "value1",
			"key2": "value2",
		}, map[string]string{
			"key1": "value1",
			"key2": "value2",
		}},
	}
	for _, test := range tests {
		output := filterSystemLabels(test[0])
		assert.Equalf(t, test[1], output, "Expected {%v}, got {%v}", test[1], output)
	}
}

func TestContainsSystemLabels(t *testing.T) {
	tests := []struct {
		input  map[string]string
		output bool
	}{
		{nil, false},
		{map[string]string{}, false},
		{map[string]string{
			"name":       "name",
			"owner":      "owner",
			"status":     "status",
			"version":    "version",
			"createdAt":  "createdAt",
			"modifiedAt": "modifiedAt",
		}, true},
		{map[string]string{
			"StaTus": "status",
			"name":   "name",
			"owner":  "owner",
			"key":    "value",
		}, true},
		{map[string]string{
			"key1": "value1",
			"key2": "value2",
		}, false},
	}
	for _, test := range tests {
		output := ContainsSystemLabels(test.input)
		assert.Equalf(t, test.output, output, "Expected {%v}, got {%v}", test.output, output)
	}
}

func TestListAllPaged(t *testing.T) {
	setListPageSize(t, 2)

	all := []v1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c"}},
	}

	tests := []struct {
		name      string
		pageFn    func(calls *[]metav1.ListOptions) func(context.Context, metav1.ListOptions) (runtime.Object, error)
		wantNames []string
		wantCalls int
		wantErr   bool
	}{
		{
			name: "walks every page",
			pageFn: func(calls *[]metav1.ListOptions) func(context.Context, metav1.ListOptions) (runtime.Object, error) {
				return func(_ context.Context, opts metav1.ListOptions) (runtime.Object, error) {
					*calls = append(*calls, opts)
					items, next, err := paginate(all, opts)
					if err != nil {
						return nil, err
					}
					return &v1.SecretList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
				}
			},
			wantNames: []string{"a", "b", "c"},
			wantCalls: 2,
		},
		{
			name: "falls back to a full list when the continue token expires",
			pageFn: func(calls *[]metav1.ListOptions) func(context.Context, metav1.ListOptions) (runtime.Object, error) {
				return func(_ context.Context, opts metav1.ListOptions) (runtime.Object, error) {
					*calls = append(*calls, opts)
					if opts.Continue != "" {
						return nil, apierrors.NewResourceExpired("continue token expired")
					}
					items, next, err := paginate(all, opts)
					if err != nil {
						return nil, err
					}
					return &v1.SecretList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
				}
			},
			wantNames: []string{"a", "b", "c"},
			wantCalls: 3,
		},
		{
			name: "returns list errors",
			pageFn: func(calls *[]metav1.ListOptions) func(context.Context, metav1.ListOptions) (runtime.Object, error) {
				return func(_ context.Context, opts metav1.ListOptions) (runtime.Object, error) {
					*calls = append(*calls, opts)
					return nil, errors.New("boom")
				}
			},
			wantCalls: 1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []metav1.ListOptions
			var names []string

			count, err := listAllPaged(t.Context(), metav1.ListOptions{}, tt.pageFn(&calls), func(s *v1.Secret) {
				names = append(names, s.Name)
			})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantNames, names)
			assert.Len(t, names, count)
			assert.Len(t, calls, tt.wantCalls)
		})
	}
}
