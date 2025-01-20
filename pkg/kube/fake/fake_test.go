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

package fake

import (
	"errors" 
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/pkg/kube"
)

func TestFailingKubeClient_UpdateWithTakeOwnership(t *testing.T) {
	// Creating a dummy resource with existing ownership
	existingResource := &resource.Info{
		Name:      "existing-resource",
		Namespace: "default",
		Object: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing-resource",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ReplicaSet",
						Name:       "original-owner",
						UID:        "123",
					},
				},
			},
		},
	}

	// Creating modified resource with new ownership
	modifiedResource := &resource.Info{
		Name:      "existing-resource",
		Namespace: "default",
		Object: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing-resource",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Helm",
						Name:       "new-owner",
						UID:        "456",
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		takeOwnership bool
		updateError   error
		wantError     bool
	}{
		{
			name:          "successful take ownership",
			takeOwnership: true,
			updateError:   nil,
			wantError:     false,
		},
		{
			name:          "ownership not taken when flag is false",
			takeOwnership: false,
			updateError:   nil,
			wantError:     false,
		},
		{
			name:          "update error with take ownership",
			takeOwnership: true,
			updateError:   errors.New("update failed"), 
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &FailingKubeClient{
				PrintingKubeClient: PrintingKubeClient{
					Out: os.Stdout,
				},
				UpdateError: tt.updateError,
			}

			existingList := kube.ResourceList{existingResource}
			modifiedList := kube.ResourceList{modifiedResource}
			result, err := client.Update(existingList, modifiedList, tt.takeOwnership)

			if tt.wantError && err == nil {
				t.Error("expected error, got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil {
				if tt.takeOwnership {
					modifiedPod := modifiedList[0].Object.(*v1.Pod)
					if len(modifiedPod.OwnerReferences) != 1 {
						t.Error("expected exactly one owner reference")
					}
					if ownerRef := modifiedPod.OwnerReferences[0]; ownerRef.Kind != "Helm" {
						t.Errorf("expected owner kind to be Helm, got %s", ownerRef.Kind)
					}
				}
				if !tt.takeOwnership {
					originalPod := existingList[0].Object.(*v1.Pod)
					if len(originalPod.OwnerReferences) != 1 {
						t.Error("expected exactly one owner reference")
					}
					if ownerRef := originalPod.OwnerReferences[0]; ownerRef.Kind != "ReplicaSet" {
						t.Errorf("expected owner kind to be ReplicaSet, got %s", ownerRef.Kind)
					}
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			}
		})
	}
}
